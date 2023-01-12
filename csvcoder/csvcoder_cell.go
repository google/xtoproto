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
	"encoding"
	"fmt"
	"reflect"
	"sync"

	"github.com/google/xtoproto/textcoder"
)

var (
	textUnmarshalerInterface = func() reflect.Type {
		var i encoding.TextUnmarshaler
		ptr := &i
		return reflect.TypeOf(ptr).Elem()
	}()
	defaultRegistryMapMutex = sync.RWMutex{}
)

// CellContext is passed to the function parsing a single CSV cell.
type CellContext struct {
	r *Row
}

// NewCellContext returns a new CellContext to use when parsing within a given
// Row.
func NewCellContext(rctx *Row) *CellContext {
	return &CellContext{rctx}
}

// Row returns the row that is being parsed.
func (c *CellContext) Row() *Row {
	return c.r
}

// ParseCell parses a single CSV cell's textual value into dst.
func ParseCell(ctx *CellContext, value string, dst interface{}) error {
	outV := reflect.ValueOf(dst)
	outType := outV.Type()
	cp, err := getOrCreateCellParserForType(outType)
	if err != nil {
		return fmt.Errorf("could not parce value %q: %w", value, err)
	}
	return cp.ParseCSVCell(ctx, value, outV)
}

// registeredCellParser is used ins
type registeredCellParser struct {
	impl    cellParser
	decoder textcoder.Decoder
}

// ParseCSVCell dispatches to the underlying implementation. This indirection
// allows newly registered cell parsers to override the older registered value.
func (cp *registeredCellParser) ParseCSVCell(ctx *CellContext, value string, field reflect.Value) error {
	if cp.impl != nil {
		return cp.impl.ParseCSVCell(ctx, value, field)
	}
	if cp.decoder != nil {
		return cp.decoder.DecodeText(textcoder.NewContext().WithValue("csvcoder.CellContext", ctx), value, field.Interface())
	}
	panic("internal error in csvcoder: registeredCellParser has no implementation")
}

// getOrCreateCellParserForType returns an object for parsing the contents of a
// textual CSV cell into an object of that type.
//
// The argument is typically a pointer type.
func getOrCreateCellParserForType(t reflect.Type) (cellParser, error) {
	if t.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("internal error: must only pass pointers to getOrCreateCellParserForType, got %v", t)
	}
	
	parser := defaultRegistry.cellParsers[t]
	if parser == nil {
		parser = &registeredCellParser{}
		defaultRegistryMapMutex.Lock()
		defaultRegistry.cellParsers[t] = parser
		defaultRegistryMapMutex.Unlock()
	}
	if parser.impl != nil {
		return parser, nil
	}
	if dec := textcoder.DefaultRegistry().GetDecoder(t.Elem()); dec != nil {
		parser.decoder = dec
		return parser, nil
	}

	return nil, fmt.Errorf("no cell parser registered for type %v", t)

}

type cellParser interface {
	// ParseCSVField parses a CSV cell value. V is the reflected value of the field.
	ParseCSVCell(ctx *CellContext, value string, field reflect.Value) error
}

type simpleCellParser func(ctx *CellContext, value string, field reflect.Value) error

func (p simpleCellParser) ParseCSVCell(ctx *CellContext, value string, field reflect.Value) error {
	return p(ctx, value, field)
}

var (
	errorInterfaceType = func() reflect.Type {
		var err error
		errPtr := &err
		return reflect.TypeOf(errPtr).Elem()
	}()
	blankInterfaceType = func() reflect.Type {
		var v interface{}
		ptr := &v
		return reflect.TypeOf(ptr).Elem()
	}()
)

var (
	stringType = reflect.TypeOf("abc")
)

func checkSignature(f reflect.Type, in, out []reflect.Type) error {
	if f.Kind() != reflect.Func {
		return fmt.Errorf("argument is not a function: %v", f)
	}
	if got, want := f.NumIn(), len(in); got != want {
		return fmt.Errorf("function takes %d arguments, want %d", got, want)
	}
	for i, want := range in {
		if got := f.In(i); got != want {
			return fmt.Errorf("function signature mismatch: %v.in[%d] got %v, want %v", f, i, got, want)
		}
	}
	if got, want := f.NumOut(), len(out); got != want {
		return fmt.Errorf("function returns %d value, want %d", got, want)
	}
	for i, want := range out {
		if got := f.Out(i); got != want {
			return fmt.Errorf("function signature mismatch: %v.out[%d] got %v, want %v", f, i, got, want)
		}
	}
	return nil
}
