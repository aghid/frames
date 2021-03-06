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
	"strings"

	v3io "github.com/v3io/v3io-go-http"

	"github.com/v3io/frames"
	"github.com/v3io/frames/backends"
	"github.com/v3io/frames/backends/utils"
	"github.com/v3io/frames/v3ioutils"
)

const (
	indexColKey = "__name"
)

// Read does a read request
func (kv *Backend) Read(request *frames.ReadRequest) (frames.FrameIterator, error) {
	tablePath := request.Table
	if !strings.HasSuffix(tablePath, "/") {
		tablePath += "/"
	}

	if request.MessageLimit == 0 {
		request.MessageLimit = 256 // TODO: More?
	}

	columns := request.Columns
	if len(columns) < 1 || columns[0] == "" {
		columns = []string{"*"}
	}

	input := v3io.GetItemsInput{Path: tablePath, Filter: request.Filter, AttributeNames: columns}
	kv.logger.DebugWith("read input", "input", input, "request", request)

	container, err := kv.newContainer(request.Session)
	if err != nil {
		return nil, err
	}

	iter, err := v3ioutils.NewAsyncItemsCursor(
		container, &input, kv.numWorkers, request.ShardingKeys, kv.logger, 0)
	if err != nil {
		return nil, err
	}

	newKVIter := Iterator{request: request, iter: iter}
	return &newKVIter, nil
}

// Iterator is key/value iterator
type Iterator struct {
	request   *frames.ReadRequest
	iter      *v3ioutils.AsyncItemsCursor
	err       error
	currFrame frames.Frame
}

// Next advances the iterator to next frame
func (ki *Iterator) Next() bool {
	var columns []frames.Column
	byName := map[string]frames.Column{}

	rowNum := 0
	for ; rowNum < int(ki.request.MessageLimit) && ki.iter.Next(); rowNum++ {
		row := ki.iter.GetFields()

		// Skip table schema object
		rowIndex, ok := row[indexColKey]
		if ok && rowIndex == ".#schema" {
			continue
		}

		for name, field := range row {
			col, ok := byName[name]
			if !ok {
				data, err := utils.NewColumn(field, rowNum)
				if err != nil {
					ki.err = err
					return false
				}

				col, err = frames.NewSliceColumn(name, data)
				if err != nil {
					ki.err = err
					return false
				}
				columns = append(columns, col)
				byName[name] = col
			}

			if err := utils.AppendColumn(col, field); err != nil {
				ki.err = err
				return false
			}
		}

		// fill columns with nil if there was no value
		for name, col := range byName {
			if _, ok := row[name]; ok {
				continue
			}

			var err error
			err = utils.AppendNil(col)
			if err != nil {
				ki.err = err
				return false
			}
		}
	}

	if ki.iter.Err() != nil {
		ki.err = ki.iter.Err()
		return false
	}

	if rowNum == 0 {
		return false
	}

	var indices []frames.Column
	indexCol, ok := byName[indexColKey]
	if ok {
		delete(byName, indexColKey)
		indices = []frames.Column{indexCol}
		columns = utils.RemoveColumn(indexColKey, columns)
	}

	var err error
	ki.currFrame, err = frames.NewFrame(columns, indices, nil)
	if err != nil {
		ki.err = err
		return false
	}

	return true
}

// Err return the last error
func (ki *Iterator) Err() error {
	return ki.err
}

// At return the current frames
func (ki *Iterator) At() frames.Frame {
	return ki.currFrame
}

func init() {
	if err := backends.Register("kv", NewBackend); err != nil {
		panic(err)
	}
}
