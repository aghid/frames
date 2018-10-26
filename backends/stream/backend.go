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

package stream

import (
	"fmt"
	"github.com/nuclio/logger"
	"github.com/pkg/errors"
	"github.com/v3io/frames"
	"github.com/v3io/frames/backends"
	"github.com/v3io/frames/v3ioutils"
	"github.com/v3io/v3io-go-http"
	"strings"
)

// Backend is a tsdb backend
type Backend struct {
	backendConfig *frames.BackendConfig
	framesConfig  *frames.Config
	logger        logger.Logger
	container     *v3io.Container
}

// NewBackend return a new v3io stream backend
func NewBackend(logger logger.Logger, cfg *frames.BackendConfig, framesConfig *frames.Config) (frames.DataBackend, error) {

	frames.InitBackendDefaults(cfg, framesConfig)
	newBackend := Backend{
		logger:        logger.GetChild("stream"),
		backendConfig: cfg,
		framesConfig:  framesConfig,
	}

	container, err := v3ioutils.CreateContainer(logger,
		cfg.URL, cfg.Container, cfg.Username, cfg.Password, cfg.Workers)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create V3IO data container")
	}
	newBackend.container = container

	return &newBackend, nil
}

// Create creates a table
func (b *Backend) Create(request *frames.CreateRequest) error {

	var isInt bool

	shards := 1
	shardsVar, ok := request.Attributes["shards"]
	if ok {
		shards, isInt = shardsVar.(int)
		if !isInt || shards < 1 {
			return errors.Errorf("Shards attribute must be a positive integer (got %v)", shardsVar)
		}
	}

	retention := 24
	retentionVar, ok := request.Attributes["retention_hours"]
	if ok {
		retention, isInt = retentionVar.(int)
		if !isInt || shards < 1 {
			return errors.Errorf("retention_hours attribute must be a positive integer (got %v)", retentionVar)
		}
	}

	if !strings.HasSuffix(request.Table, "/") {
		request.Table += "/"
	}

	err := b.container.Sync.CreateStream(&v3io.CreateStreamInput{
		Path: request.Table, ShardCount: shards, RetentionPeriodHours: retention})
	if err != nil {
		b.logger.ErrorWith("CreateStream failed", "path", request.Table, "err", err)
	}

	return nil
}

// Delete deletes a table or part of it
func (b *Backend) Delete(request *frames.DeleteRequest) error {
	return fmt.Errorf("Delete not implemented")
}

func init() {
	if err := backends.Register("stream", NewBackend); err != nil {
		panic(err)
	}
}
