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

package tsdb

import (
	"github.com/nuclio/logger"

	"github.com/v3io/frames"
	"github.com/v3io/frames/backends"

	"github.com/pkg/errors"
	"github.com/v3io/frames/v3ioutils"
	"github.com/v3io/v3io-tsdb/pkg/config"
	"github.com/v3io/v3io-tsdb/pkg/tsdb"
	"github.com/v3io/v3io-tsdb/pkg/tsdb/schema"
	tsdbutils "github.com/v3io/v3io-tsdb/pkg/utils"
)

// Backend is a tsdb backend
type Backend struct {
	adapters      map[string]*tsdb.V3ioAdapter
	backendConfig *frames.BackendConfig
	framesConfig  *frames.Config
	logger        logger.Logger
}

// NewBackend return a new tsdb backend
func NewBackend(logger logger.Logger, cfg *frames.BackendConfig, framesConfig *frames.Config) (frames.DataBackend, error) {

	frames.InitBackendDefaults(cfg, framesConfig)
	newBackend := Backend{
		adapters:      map[string]*tsdb.V3ioAdapter{},
		logger:        logger.GetChild("tsdb"),
		backendConfig: cfg,
		framesConfig:  framesConfig,
	}

	return &newBackend, nil
}

func (b *Backend) newConfig(session *frames.Session) *config.V3ioConfig {
	return &config.V3ioConfig{
		WebApiEndpoint: session.Url,
		Container:      session.Container,
		Username:       session.User,
		Password:       session.Password,
		Workers:        b.backendConfig.Workers,
		LogLevel:       b.framesConfig.Log.Level,
	}
}

func (b *Backend) newAdapter(session *frames.Session, path string) (*tsdb.V3ioAdapter, error) {

	if path == "" {
		path = session.Path
	}

	cfg := b.newConfig(session)
	_, err := config.GetOrLoadFromStruct(cfg)
	if err != nil {
		// if we couldn't load the file and its not the default
		return nil, err
	}

	container, err := v3ioutils.CreateContainer(b.logger,
		session.Url, cfg.Container, cfg.Username, cfg.Password, cfg.Workers)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create V3IO data container")
	}

	cfg.TablePath = path
	b.logger.DebugWith("tsdb config", "config", cfg)
	adapter, err := tsdb.NewV3ioAdapter(cfg, container, b.logger)
	if err != nil {
		return nil, err
	}

	return adapter, nil
}

// GetAdapter returns an adapter
func (b *Backend) GetAdapter(session *frames.Session, path string) (*tsdb.V3ioAdapter, error) {
	// TODO: maintain adapter cache, for now create new per read/write request
	//adapter, ok := b.adapters[path]
	//if !ok {
	//	b.adapters[path] = adapter
	//}

	adapter, err := b.newAdapter(session, path)
	if err != nil {
		return nil, err
	}
	return adapter, nil
}

// Create creates a table
func (b *Backend) Create(request *frames.CreateRequest) error {

	attrs := request.Attributes()

	attr, ok := attrs["rate"]
	if !ok {
		return errors.New("Must specify 'rate' attribute to specify maximum sample rate, e.g. '1/m'")
	}
	rate, isStr := attr.(string)
	if !isStr {
		return errors.New("'rate' attribute must be a string, e.g. '1/m'")
	}

	aggregationGranularity := config.DefaultAggregationGranularity
	attr, ok = attrs["aggregation-granularity"]
	if ok {
		val, isStr := attr.(string)
		if !isStr {
			return errors.New("'aggregation-granularity' attribute must be a string")
		}
		aggregationGranularity = val
	}

	defaultRollups := ""
	attr, ok = attrs["aggregates"]
	if ok {
		val, isStr := attr.(string)
		if !isStr {
			return errors.New("'aggregates' attribute must be a string")
		}
		defaultRollups = val
	}

	// TODO: create unique tsdb cfg object, avoid conflict w other requests
	cfg := b.newConfig(request.Session)
	cfg.TablePath = request.Table
	dbSchema, err := schema.NewSchema(cfg, rate, aggregationGranularity, defaultRollups)

	if err != nil {
		return errors.Wrap(err, "Failed to create a TSDB schema.")
	}

	return tsdb.CreateTSDB(cfg, dbSchema)
}

// Delete deletes a table or part of it
func (b *Backend) Delete(request *frames.DeleteRequest) error {

	start, err := tsdbutils.Str2duration(request.Start)
	if err != nil {
		return err
	}

	end, err := tsdbutils.Str2duration(request.End)
	if err != nil {
		return err
	}

	delAll := request.Start == "" && request.End == ""

	adapter, err := b.GetAdapter(request.Session, request.Table)
	if err != nil {
		return err
	}

	return adapter.DeleteDB(delAll, request.Force, start, end)
}

func init() {
	if err := backends.Register("tsdb", NewBackend); err != nil {
		panic(err)
	}
}
