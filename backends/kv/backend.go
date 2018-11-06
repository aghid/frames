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

package kv

import (
	"fmt"
	"github.com/nuclio/logger"
	"github.com/pkg/errors"
	"github.com/v3io/frames"
	"github.com/v3io/frames/v3ioutils"
	"github.com/v3io/v3io-go-http"
)

// Backend is key/value backend
type Backend struct {
	container  *v3io.Container
	logger     logger.Logger
	numWorkers int
}

// NewBackend return a new key/value backend
func NewBackend(logger logger.Logger, config *frames.BackendConfig, framesConfig *frames.Config) (frames.DataBackend, error) {

	frames.InitBackendDefaults(config, framesConfig)
	container, err := v3ioutils.CreateContainer(
		logger, config.URL, config.Container, config.Username, config.Password, config.Workers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create data container")
	}

	newBackend := Backend{
		logger:     logger.GetChild("kv"),
		container:  container,
		numWorkers: config.Workers,
	}

	return &newBackend, nil
}

// Create creates a table
func (b *Backend) Create(request *frames.CreateRequest) error {
	return fmt.Errorf("not requiered, table is created on first write")
}

// Delete deletes a table (or part of it)
func (b *Backend) Delete(request *frames.DeleteRequest) error {

	return v3ioutils.DeleteTable(b.logger, b.container, request.Table, request.Filter, b.numWorkers)
	// TODO: delete the table directory entry if filter == ""
}
