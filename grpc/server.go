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

package grpc

import (
	"github.com/v3io/frames"
	"github.com/v3io/frames/api"

	"github.com/nuclio/logger"
	"github.com/pkg/errors"
)

// Server is a frames gRPC server
type Server struct {
	config *frames.Config
	api    *api.API
	logger logger.Logger
}

// NewServer returns a new gRPC server
func NewServer(config *frames.Config, addr string, logger logger.Logger) (*Server, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "bad configuration")
	}

	if err := config.InitDefaults(); err != nil {
		return nil, errors.Wrap(err, "failed to init defaults")
	}

	if logger == nil {
		var err error
		logger, err = frames.NewLogger(config.Log.Level)
		if err != nil {
			return nil, errors.Wrap(err, "can't create logger")
		}
	}

	api, err := api.New(logger, config)
	if err != nil {
		return nil, errors.Wrap(err, "can't create API")
	}

	server := &Server{
		config: config,
		api:    api,
		logger: logger,
	}

	return server, nil
}

func (s *Server) Read(request *ReadRequest, stream Frames_ReadServer) error {
	ch := make(chan frames.Frame)

	var apiError error
	go func() {
		defer close(ch)
		apiError = s.api.Read(readRequest(request), ch)
		if apiError != nil {
			s.logger.ErrorWith("API error reading", "error", apiError)
		}
	}()

	for frame := range ch {
		pbFrame, err := frameMessage(frame)
		if err != nil {
			return err
		}

		if err := stream.Send(pbFrame); err != nil {
			return err
		}
	}

	return apiError
}

func (s *Server) Write(Frames_WriteServer) error {
	return nil
}