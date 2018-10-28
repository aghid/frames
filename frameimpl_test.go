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

package frames

import (
	"fmt"
	"testing"
)

func TestFrameNew(t *testing.T) {
	val0, val1, size := 7, "n", 10
	col0, _ := NewLabelColumn("col0", val0, size)
	col1, _ := NewLabelColumn("col1", val1, size)
	cols := []Column{col0, col1}

	frame, err := NewFrame(cols, nil, nil)
	if err != nil {
		t.Fatalf("can't create frame - %s", err)
	}

	names := frame.Names()
	if len(names) != len(cols) {
		t.Fatalf("# of columns mismatch - %d != %d", len(names), len(cols))
	}

	for i, name := range names {
		col := cols[i]
		if col.Name() != name {
			t.Fatalf("%d: name mismatch - %q != %q", i, col.Name(), name)
		}

		if col.Len() != size {
			t.Fatalf("%d: length mismatch - %d != %d", i, col.Len(), size)
		}

		switch i {
		case 0:
			val, _ := col.IntAt(0)
			if val != val0 {
				t.Fatalf("%d: value mismatch - %d != %d", i, val, val0)
			}
		case 1:
			val, _ := col.StringAt(0)
			if val != val1 {
				t.Fatalf("%d: value mismatch - %q != %q", i, val, val1)
			}
		}

	}
}

func TestFrameSlice(t *testing.T) {
	nCols, size := 7, 10
	cols := newIntCols(t, nCols, size)
	frame, err := NewFrame(cols, nil, nil)
	if err != nil {
		t.Fatalf("can't create frame - %s", err)
	}

	names := frame.Names()
	if len(names) != nCols {
		t.Fatalf("# of columns mismatch - %d != %d", len(names), nCols)
	}

	start, end := 2, 7
	frame2, err := frame.Slice(start, end)
	if err != nil {
		t.Fatalf("can't create slice - %s", err)
	}

	if frame2.Len() != end-start {
		t.Fatalf("bad # of rows in slice - %d != %d", frame2.Len(), end-start)
	}

	names2 := frame2.Names()
	if len(names2) != nCols {
		t.Fatalf("# of columns mismatch - %d != %d", len(names2), nCols)
	}
}

func TestFrameIndex(t *testing.T) {
	nCols, size := 2, 12
	cols := newIntCols(t, nCols, size)

	indices := newIntCols(t, 3, size)

	frame, err := NewFrame(cols, indices, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(frame.Indices()) != len(indices) {
		t.Fatalf("index len mismatch (%d != %d)", len(frame.Indices()), len(indices))
	}
}

func TestNewFrameFromRows(t *testing.T) {
	rows := []map[string]interface{}{
		{"x": 1, "y": "a"},
		{"x": 2, "z": 1.0},
		{"x": 3, "y": "b", "z": 2.0},
	}

	indices := []string{"z"}
	frame, err := NewFrameFromRows(rows, indices, nil)
	if err != nil {
		t.Fatal(err)
	}

	if frame.Len() != len(rows) {
		t.Fatalf("rows len mismatch %d != %d", frame.Len(), len(rows))
	}

	if len(frame.Names()) != 2 {
		t.Fatalf("columns len mismatch %d != %d", len(frame.Names()), 2)
	}

	if len(frame.Indices()) != len(indices) {
		t.Fatalf("indices len mismatch %d != %d", len(frame.Indices()), len(rows))
	}
}

func TestNewFrameFromRowsMissing(t *testing.T) {
	rows := []map[string]interface{}{
		{"x": 1, "y": "a"},
		{"x": 2, "z": 1.0},
	}

	frame, err := NewFrameFromRows(rows, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if frame.Len() != 2 {
		t.Fatalf("frame length mismatch: %d != 2", frame.Len())
	}
}

func newIntCols(t *testing.T, numCols int, size int) []Column {
	var cols []Column

	for i := 0; i < numCols; i++ {
		col, err := NewLabelColumn(fmt.Sprintf("col%d", i), i, size)
		if err != nil {
			t.Fatalf("can't create column - %s", err)
		}

		cols = append(cols, col)
	}

	return cols
}
