/*
Copyright 2018 Iguazio Systems Ltd.

Licensed under the Apache License, Version 2.0 (the "License") with
an addition restriction as set forth herein. You may not use this
file except in compliance with the License. You may obtain a copy of
the License at http://www.apache.org/licenses/LICENSE-2.0.

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
implied. See the License for the specific language governing
permissions and limitations under the License.

In addition, you may not use the software for any purposes that are
illegal under applicable law, and the grant of the foregoing license
under the Apache 2.0 license is conditioned upon your compliance with
such restriction.
*/

package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/nuclio/logger"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack"

	"github.com/v3io/frames"
)

// Client is v3io streaming client
type Client struct {
	URL    string
	logger logger.Logger
	err    error // last error
}

var (
	// Make sure we're implementing frames.Client
	_ frames.Client = &Client{}
)

// NewClient returns a new HTTP client
func NewClient(url string, logger logger.Logger) (*Client, error) {
	var err error
	if logger == nil {
		logger, err = frames.NewLogger("info")
		if err != nil {
			return nil, errors.Wrap(err, "can't create logger")
		}
	}

	if url == "" {
		url = os.Getenv("V3IO_URL")
	}

	if url == "" {
		return nil, fmt.Errorf("empty URL")
	}

	client := &Client{
		URL:    url,
		logger: logger,
	}

	return client, nil
}

// Read runs a query on the client
func (c *Client) Read(request *frames.ReadRequest) (frames.FrameIterator, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(request); err != nil {
		return nil, errors.Wrap(err, "can't encode query")
	}

	req, err := http.NewRequest("POST", c.URL+"/read", &buf)
	if err != nil {
		return nil, errors.Wrap(err, "can't create HTTP request")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "can't call API")
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		var buf bytes.Buffer
		io.Copy(&buf, resp.Body)

		return nil, fmt.Errorf("API returned with bad code - %d\n%s", resp.StatusCode, buf.String())
	}

	it := &streamFrameIterator{
		reader:  resp.Body,
		decoder: frames.NewDecoder(resp.Body),
		logger:  c.logger,
	}

	return it, nil
}

// Write writes data
func (c *Client) Write(request *frames.WriteRequest) (frames.FrameAppender, error) {
	if request.Backend == "" || request.Table == "" {
		return nil, fmt.Errorf("missing request parameters")
	}

	var buf bytes.Buffer
	if err := msgpack.NewEncoder(&buf).Encode(request); err != nil {
		return nil, errors.Wrap(err, "Can't encode request")
	}

	reader, writer := io.Pipe()
	req, err := http.NewRequest("POST", c.URL+"/write", io.MultiReader(&buf, reader))
	if err != nil {
		return nil, errors.Wrap(err, "can't create HTTP request")
	}

	appender := &streamFrameAppender{
		writer:  writer,
		encoder: frames.NewEncoder(writer),
		ch:      make(chan *appenderHTTPResponse, 1),
		logger:  c.logger,
	}

	// Call API in a goroutine since it's going to block reading from pipe
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			c.logger.ErrorWith("error calling API", "error", err)
		}

		appender.ch <- &appenderHTTPResponse{resp, err}
	}()

	return appender, nil
}

// Delete deletes data
func (c *Client) Delete(request *frames.DeleteRequest) error {
	return c.jsonCall("/delete", request)
}

// Create creates a table
func (c *Client) Create(request *frames.CreateRequest) error {
	return c.jsonCall("/create", request)
}

// Exec executes a command
func (c *Client) Exec(request *frames.ExecRequest) error {
	return c.jsonCall("/exec", request)
}

func (c *Client) jsonCall(path string, request interface{}) error {
	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(request); err != nil {
		return errors.Wrap(err, "can't encode request")
	}

	req, err := http.NewRequest("POST", c.URL+path, &buf)
	if err != nil {
		return errors.Wrap(err, "can't create HTTP request")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "can't call server")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error calling server - %q", resp.Status)
	}

	return nil
}

// streamFrameIterator implements FrameIterator over io.Reader
type streamFrameIterator struct {
	frame   frames.Frame
	err     error
	reader  io.Reader
	decoder *frames.Decoder
	logger  logger.Logger
}

func (it *streamFrameIterator) Next() bool {
	var err error

	it.frame, err = it.decoder.DecodeFrame()
	if err == nil {
		return true
	}

	if err == io.EOF {
		closer, ok := it.reader.(io.Closer)
		if ok {
			if err := closer.Close(); err != nil {
				it.logger.WarnWith("can't close reader", "error", err)
			}
		}

		return false
	}

	it.err = err
	return false
}

func (it *streamFrameIterator) At() frames.Frame {
	return it.frame
}

func (it *streamFrameIterator) Err() error {
	return it.err
}

type appenderHTTPResponse struct {
	resp *http.Response
	err  error
}

// streamFrameAppender implements FrameAppender over io.Writer
type streamFrameAppender struct {
	writer  io.Writer
	encoder *frames.Encoder
	ch      chan *appenderHTTPResponse
	logger  logger.Logger
}

func (a *streamFrameAppender) Add(frame frames.Frame) error {
	if err := a.encoder.Encode(frame); err != nil {
		return errors.Wrap(err, "can't encode frame")
	}

	return nil
}

func (a *streamFrameAppender) WaitForComplete(timeout time.Duration) error {
	closer, ok := a.writer.(io.Closer)
	if !ok {
		return fmt.Errorf("writer is not a closer")
	}

	if err := closer.Close(); err != nil {
		return errors.Wrap(err, "can't close writer")
	}

	select {
	case hr := <-a.ch:
		if hr.resp.StatusCode != http.StatusOK {
			var buf bytes.Buffer
			io.Copy(&buf, hr.resp.Body)
			hr.resp.Body.Close()
			return fmt.Errorf("server returned error - %d\n%s", hr.resp.StatusCode, buf.String())
		}

		hr.resp.Body.Close()
		return hr.err
	case <-time.After(timeout):
		return fmt.Errorf("timeout after %s", timeout)
	}
}
