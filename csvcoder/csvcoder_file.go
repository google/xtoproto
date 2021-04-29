// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package csvcoder

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

// FileParser is an object used to parse an entire CSV file.
type FileParser struct {
	r        RowReader
	filePath string
	rt       *registeredType

	hdrOpt headerOption

	hdr      *Header
	rowNum   RowNumber
	fatalErr error
}

// RowReader is an interface of a reader that reads raw records from the data
// source. For example, csv.Reader is a RowReader.
type RowReader interface {
	Read() ([]string, error)
}

// NewFileParser returns an object for parsing a set of records from a file.
//
// The input RowReader will be used to read all rows. The path argument is only
// for error reporting purposes.
//
// The type of the recordPrototype should have been registered with a call to
// RegisterRowStruct.
func NewFileParser(r RowReader, path string, recordPrototype interface{}) (*FileParser, error) {
	rt, err := getOrRegisterType(reflect.ValueOf(recordPrototype).Type())
	if err != nil {
		return nil, fmt.Errorf("could not find or infer coder for type %v: %w", reflect.ValueOf(recordPrototype).Type(), err)
	}
	fp := &FileParser{
		r, path, rt,
		headerOption{expectedColumns: rt.requiredColumnNames},
		nil,
		0,
		nil,
	}

	if err := fp.parseHeader(); err != nil {
		return nil, err
	}

	return fp, nil
}

func (fp *FileParser) parseHeader() error {
	if fp.hdrOpt.noHeader {
		fp.hdr = fp.hdrOpt.predeterminedHeader
	}
	gotHeaderValues, err := fp.r.Read()
	if err != nil {
		return fmt.Errorf("error reading header row: %w", err)
	}
	fp.rowNum = 1
	fp.hdr = NewHeader(gotHeaderValues)
	missing := []string{}
	for wantCol := range fp.hdrOpt.expectedColumns {
		if !fp.hdr.ColumnIndex(wantCol).IsValid() {
			missing = append(missing, fmt.Sprintf("%q", wantCol))
		}
	}
	if len(missing) != 0 {
		sort.Strings(missing)
		return fmt.Errorf("header row is missing %d columns: %s", len(missing), strings.Join(missing, ", "))
	}

	return nil
}

// Read parses the next record in the CSV. The header is parsed automatically.
func (fp *FileParser) Read() (interface{}, error) {
	if fp.hdr == nil {
		if err := fp.parseHeader(); err != nil {
			return nil, fmt.Errorf("failed to parse header: %w", err)
		}
	}

	rowVals, err := fp.r.Read()
	if err == io.EOF {
		return nil, err
	}
	row := NewRow(rowVals, fp.hdr, fp.rowNum, fp.filePath)
	if err != nil {
		return nil, row.errorf("csv.Reader error: %w", err)
	}
	fp.rowNum++

	return fp.rt.parseRow(row)
}

// ReadAll calls Read() until the end of the file and calls cb for each value.
func (fp *FileParser) ReadAll(callback func(interface{}) error) error {
	for {
		got, err := fp.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := callback(got); err != nil {
			return err
		}
	}
}

// headerOption is an argument passed to ParseRecords
//
// TODO(reddaly): Export this?
type headerOption struct {
	expectedColumns     map[string]struct{}
	noHeader            bool
	predeterminedHeader *Header
}

// expectedHeaderUnordered returns a HeaderOption to use that requires the
// provided set of column names in any order.
func expectedHeaderUnordered(columns []string) headerOption {
	m := make(map[string]struct{})
	for _, c := range columns {
		m[c] = struct{}{}
	}
	return headerOption{expectedColumns: m}
}

// noHeader returns a HeaderOption that asserts that no header
func noHeader(predeterminedHeader *Header) headerOption {
	if predeterminedHeader == nil {
		panic("nil predeterminedHeader")
	}
	return headerOption{noHeader: true, predeterminedHeader: predeterminedHeader}
}
