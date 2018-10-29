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
	"fmt"

	"github.com/golang/protobuf/ptypes"
	tspb "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/v3io/frames"
)

func readRequest(request *ReadRequest) *frames.ReadRequest {
	return &frames.ReadRequest{
		Backend: request.Backend,
		Table:   request.Table,
		Query:   request.Query,
	}
}

func frameMessage(frame frames.Frame) (*Frame, error) {
	names := frame.Names()
	columns := make([]*Column, len(names))
	for i, name := range names {
		col, err := frame.Column(name)
		if err != nil {
			return nil, err
		}

		pbCol, err := columnMessage(col)
		if err != nil {
			return nil, err
		}
		columns[i] = pbCol
	}

	indices := make([]*Column, len(frame.Indices()))
	for i, col := range frame.Indices() {
		pbCol, err := columnMessage(col)
		if err != nil {
			return nil, err
		}
		indices[i] = pbCol
	}

	pbFrame := &Frame{
		Columns: columns,
		Indices: indices,
	}
	return pbFrame, nil
}

func columnMessage(column frames.Column) (*Column, error) {
	pbCol := &Column{}
	switch column.(type) {
	case *frames.SliceColumn:
		sliceCol, err := pbSliceCol(column.(*frames.SliceColumn))
		if err != nil {
			return nil, err
		}
		pbCol.Data = sliceCol
	case *frames.LabelColumn:
		labelCol, err := pbLabelCol(column.(*frames.LabelColumn))
		if err != nil {
			return nil, err
		}
		pbCol.Data = labelCol
	default:
		return nil, fmt.Errorf("unknown column type - %T", column)
	}

	return pbCol, nil
}

func pbSliceCol(column *frames.SliceColumn) (*Column_Slice, error) {
	slice := SliceCol{
		Name: column.Name(),
	}

	switch column.DType() {
	case frames.IntType:
		data, err := column.Ints()
		if err != nil {
			return nil, err
		}
		// TODO: Instead of copy use unsafe cast?
		slice.Ints = make([]int64, len(data))
		for i, n := range data {
			slice.Ints[i] = int64(n)
		}
	case frames.FloatType:
		data, err := column.Floats()
		if err != nil {
			return nil, err
		}
		slice.Floats = data
	case frames.StringType:
		data := column.Strings()
		slice.Strings = data
	case frames.TimeType:
		data, err := column.Times()
		if err != nil {
			return nil, err
		}

		slice.Times = make([]*tspb.Timestamp, len(data))
		for i, t := range data {
			slice.Times[i], err = ptypes.TimestampProto(t)
			if err != nil {
				return nil, err
			}
		}
	case frames.BoolType:
		data, err := column.Bools()
		if err != nil {
			return nil, err
		}

		slice.Bools = data
	}

	return &Column_Slice{&slice}, nil
}

func pbLabelCol(column *frames.LabelColumn) (*Column_Label, error) {
	label := &LabelCol{
		Name:  column.Name(),
		Size:  int64(column.Len()),
		Dtype: column.DType().String(),
	}

	if label.Size > 0 {
		switch column.DType() {
		case frames.IntType:
			value, err := column.IntAt(0)
			if err != nil {
				return nil, err
			}

			label.Value = &LabelCol_Ival{int64(value)}
		case frames.FloatType:
			value, err := column.FloatAt(0)
			if err != nil {
				return nil, err
			}

			label.Value = &LabelCol_Fval{value}
		case frames.StringType:
			value, err := column.StringAt(0)
			if err != nil {
				return nil, err
			}

			label.Value = &LabelCol_Sval{value}
		case frames.TimeType:
			value, err := column.TimeAt(0)
			if err != nil {
				return nil, err
			}

			ts, err := ptypes.TimestampProto(value)
			if err != nil {
				return nil, err
			}

			label.Value = &LabelCol_Tval{ts}
		case frames.BoolType:
			value, err := column.BoolAt(0)
			if err != nil {
				return nil, err
			}

			label.Value = &LabelCol_Bval{value}
		}
	}

	return &Column_Label{label}, nil
}