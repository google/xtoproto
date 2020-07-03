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

// Package csvcoder decodes Go types from CSV records using struct tags, similar
// to encoding/csv and encoding/xml packages in the standard library.
//
// Like the standard library packages for JSON and XML encoding, this package
// uses tags on struct fields to specify the correspondence of a CSV column with
// a struct field. See the examples for full usage, but to get a sense of how
// csvcoder works, observe the following structure definition:
//
//		type mass float64
//
//		type species struct {
//			Name                string `csv:"name"`
//			EstimatedPopulation int    `csv:"population_estimate"`
//			Mass                mass   `csv:"weight_kg"`
//		}
//		csvcoder.RegisterRowStruct(reflect.TypeOf(&species{}))
//
// Unlike the standard library packages, this package uses textcoder for
// decoding textual values, allowing any package to provide a decoder for a
// given type rather than using methods of the type.
package csvcoder

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// Registered row types.
var (
	defaultRegistry = newRegistry()
)

// ParseRow returns an error if the row fails to parse.
//
// If the destination object has a `ParseCSVRow(*Row) error` method, that method
// will be called on the destination object.
//
//
func ParseRow(row *Row, destination interface{}) error {
	type parsable interface {
		ParseCSVRow(row *Row) error
	}

	if p, ok := destination.(parsable); ok {
		err := p.ParseCSVRow(row)
		if err != nil {
			return row.errorf("%w", err)
		}
		return nil
	}
	t := reflect.ValueOf(destination).Type()
	p, err := getRegisteredTypeOrErr(t)
	if err != nil {
		return row.errorf("failed to parse CSV row into destination %v: %w", destination, err)
	}
	if err := p.parser.ParseCSVRow(row, destination); err != nil {
		return row.errorf("%w", err)
	}
	return nil
}

// Row is passed to Unmarshal
type Row struct {
	values   []string
	h        *Header
	num      RowNumber
	fileName string
}

// NewRow returns a new parsing context.
func NewRow(values []string, h *Header, num RowNumber, fileName string) *Row {
	return &Row{values, h, num, fileName}
}

// Strings returns the string values of the row.
func (r *Row) Strings() []string {
	if r == nil {
		return nil
	}
	return r.values
}

// Header returns the header of the  CSV file.
func (r *Row) Header() *Header {
	if r == nil {
		return nil
	}
	return r.h
}

// Number returns the row number.
func (r *Row) Number() RowNumber {
	if r == nil {
		return InvalidRow
	}
	return r.num
}

// Path returns the path of the CSV file or empty if this information is not available.
func (r *Row) Path() string {
	if r == nil {
		return ""
	}
	return r.fileName
}

// PositionString returns a human readable representation of the row position.
func (r *Row) PositionString() string {
	parts := []string{}
	fileName := r.Path()
	num := r.Number()
	if fileName != "" {
		parts = append(parts, fileName)
	}
	rowNumStr := "<unknown row number>"
	if num.IsValid() {
		rowNumStr = fmt.Sprintf("%d", num.Ordinal())
	}
	parts = append(parts, rowNumStr)
	return strings.Join(parts, ":")
}

// errorf returns a human readable representation of the row position.
func (r *Row) errorf(format string, a ...interface{}) error {
	return fmt.Errorf("%s: %w", r.PositionString(), fmt.Errorf(format, a...))
}

// Header contains the values of the first row of the CSV file.
type Header struct {
	m      map[string]ColumnNumber
	values []string
}

// NewHeader returns a Header based on the given values.
func NewHeader(values []string) *Header {
	h := &Header{make(map[string]ColumnNumber), values}
	for i, s := range values {
		h.m[s] = ColumnNumber(i)
	}
	return h
}

// ColumnIndex returns the index of the column with the given name.
func (h *Header) ColumnIndex(col string) ColumnNumber {
	cn, ok := h.m[col]
	if !ok {
		return InvalidColumn
	}
	return cn
}

// ColumnNames returns the names of the columns.
func (h *Header) ColumnNames() []string {
	return h.values
}

// RegisterRowStruct registers a struct type T that can be encoded as a CSV row.
//
// Each public field of the struct definition will be examined and treated as
// follows:
//
// 1. If the field has a `csv-skip` tag, it will not be parsed.
//
// 2. If the field has a `csv` tag, that tag will be treated as the name of the
// CSV column for the field. If the tag is absent, the column name used will be
// the name of the field.
//
// 3. Let FT be the Go type of the field. If there is a registered decoder for
// *FT, that decoder will be used to decode the string value of the field
// with the name from step 2 into the field of a row being parsed. If there is
// no registered decoder for *FT, RegisterRowStruct will panic and
// SafeRegisterRowStruct returns an error.
func RegisterRowStruct(t reflect.Type) {
	if _, err := getOrRegisterType(t); err != nil {
		panic(fmt.Errorf("RegisterStruct failed: %w", err))
	}
}

// SafeRegisterRowStruct calls RegisterRowStruct but returns an error instead of
// panicking if there are any issues.
func SafeRegisterRowStruct(t reflect.Type) error {
	_, err := getOrRegisterType(t)
	return err
}

type rowParser interface {
	ParseCSVRow(row *Row, dst interface{}) error
}

type registeredType struct {
	t                   reflect.Type
	requiredColumnNames map[string]struct{}
	parser              *structParser
	makeZero            func() interface{}
}

func (rt *registeredType) parseRow(row *Row) (interface{}, error) {
	v := rt.makeZero()
	err := rt.parser.ParseCSVRow(row, v)
	return v, err
}

func getRegisteredType(t reflect.Type) *registeredType {
	var rt *registeredType
	func() {
		defaultRegistry.registeredRowTypesLock.RLock()
		defer defaultRegistry.registeredRowTypesLock.RUnlock()
		rt = defaultRegistry.registeredRowTypes[t]
	}()
	return rt
}

func getRegisteredTypeOrErr(t reflect.Type) (*registeredType, error) {
	rt := getRegisteredType(t)
	if rt == nil {
		var typeStrings []string
		for _, rt := range defaultRegistry.registeredRowTypes {
			typeStrings = append(typeStrings, fmt.Sprintf("%v", rt))
		}
		return nil, fmt.Errorf("no CSV row parser registered for type %v; registered types: [%s]", t, strings.Join(typeStrings, ", "))
	}
	return rt, nil
}

func getOrRegisterType(t reflect.Type) (*registeredType, error) {
	if rt := getRegisteredType(t); rt != nil {
		return rt, nil
	}
	rt, err := inferRegisteredType(t)
	if err != nil {
		return nil, err
	}
	defaultRegistry.registeredRowTypesLock.Lock()
	defer defaultRegistry.registeredRowTypesLock.Unlock()
	defaultRegistry.registeredRowTypes[t] = rt
	return rt, nil
}

func inferRegisteredType(t reflect.Type) (*registeredType, error) {
	if !(t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct) {
		return nil, fmt.Errorf("type %v is not a pointer to a struct, so could not infer a CSV row parser", t)
	}

	requiredColumns := make(map[string]struct{})
	var valueExtractors []func(row *Row, dst reflect.Value) error
	for i := 0; i < t.Elem().NumField(); i++ {
		f := t.Elem().FieldByIndex([]int{i})
		if _, ok := f.Tag.Lookup("csv-skip"); ok {
			continue
		}
		colName, ok := f.Tag.Lookup("csv")
		if !ok {
			colName = f.Name
		}
		requiredColumns[colName] = struct{}{}

		cellParser, err := getOrCreateCellParserForType(reflect.PtrTo(f.Type))
		if err != nil {
			return nil, err
		}

		valueExtractors = append(valueExtractors, func(row *Row, dstRow reflect.Value) error {
			idx := row.Header().ColumnIndex(colName)
			if !idx.IsValid() {
				return fmt.Errorf("csv file missing required column %q", colName)
			}
			if idx.Offset() >= len(row.Strings()) {
				return fmt.Errorf("csv row does not have a value for column %q", colName)
			}
			strValue := row.Strings()[idx.Offset()]
			return cellParser.ParseCSVCell(NewCellContext(row), strValue, dstRow.Elem().FieldByIndex(f.Index).Addr())
		})

	}
	return &registeredType{
		t,
		requiredColumns,
		&structParser{valueExtractors},
		func() interface{} {
			return reflect.New(t.Elem()).Interface()
		},
	}, nil
}

type structParser struct {
	fieldParsers []func(row *Row, dstRow reflect.Value) error
}

func (p *structParser) ParseCSVRow(row *Row, dst interface{}) error {
	dstReflect := reflect.ValueOf(dst)
	for _, fp := range p.fieldParsers {
		if err := fp(row, dstReflect); err != nil {
			return row.errorf("error parsing struct row: %w", err)
		}
	}
	return nil
}

type registry struct {
	cellParsers            map[reflect.Type]*registeredCellParser
	registeredRowTypes     map[reflect.Type]*registeredType
	registeredRowTypesLock sync.RWMutex
}

func newRegistry() *registry {
	return &registry{
		make(map[reflect.Type]*registeredCellParser),
		make(map[reflect.Type]*registeredType),
		sync.RWMutex{},
	}
}
