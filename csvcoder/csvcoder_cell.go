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
	"strings"

	"github.com/golang/glog"
)

var (
	textUnmarshalerInterface = func() reflect.Type {
		var i encoding.TextUnmarshaler
		ptr := &i
		return reflect.TypeOf(ptr).Elem()
	}()
)

// RegisterTextCoder registers an encoding function and a decoding function
// for parsing and printing values of the provided type.
//
// If the provided type is T, the decoder should be of the form
// func(string, *T) error, and the encoder should be of the form
// func(T) (string, error).
func RegisterTextCoder(t reflect.Type, encoder, decoder interface{}) {
	if err := SafeRegisterTextCoder(t, encoder, decoder); err != nil {
		panic(err)
	}
}

// SafeRegisterTextCoder is like RegisterTextCoder but returns an error instead
// of panicking if an error is encountered.
func SafeRegisterTextCoder(t reflect.Type, encoder, decoder interface{}) error {
	return registerCellParser(t, encoder, decoder)
}

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
	impl cellParser
}

// ParseCSVCell dispatches to the underlying implementation. This indirection
// allows newly registered cell parsers to orverride the older registered value.
func (cp *registeredCellParser) ParseCSVCell(ctx *CellContext, value string, field reflect.Value) error {
	if cp.impl == nil {
		panic("internal error in csvcoder: registeredCellParser impl is nil")
	}
	return cp.impl.ParseCSVCell(ctx, value, field)
}

// getOrCreateCellParserForType returns an object for parsing the contents of a
// textual CSV cell into an object of that type.
//
// The argument is typically a pointer type.
func getOrCreateCellParserForType(t reflect.Type) (cellParser, error) {
	parser := defaultRegistry.cellParsers[t]
	if parser == nil {
		parser = &registeredCellParser{nil}
		defaultRegistry.cellParsers[t] = parser
	}
	if parser.impl != nil {
		return parser, nil
	}
	checkedExplicitParsers := []reflect.Type{t}

	if t.Kind() == reflect.Ptr {
		pointedToType := t.Elem()
		pointeeParser := defaultRegistry.cellParsers[pointedToType]
		if pointeeParser != nil {
			parser.impl = simpleCellParser(func(ctx *CellContext, value string, field reflect.Value) error {
				if field.IsNil() {
					field.Set(reflect.New(pointedToType))
				}
				return pointeeParser.ParseCSVCell(ctx, value, field.Elem())
			})
			return parser, nil
		}

		checkedExplicitParsers = append(checkedExplicitParsers, pointedToType)
	}

	if t.Implements(textUnmarshalerInterface) {
		if t.Kind() == reflect.Ptr {
			pointedToType := t.Elem()
			parser.impl = simpleCellParser(func(ctx *CellContext, value string, field reflect.Value) error {
				if field.IsNil() {
					field.Set(reflect.New(pointedToType))
				}
				return field.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value))
			})
		} else {
			parser.impl = simpleCellParser(func(ctx *CellContext, value string, field reflect.Value) error {
				return field.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value))
			})
		}
		return parser, nil
	}

	var typeStrings []string
	for _, t := range checkedExplicitParsers {
		typeStrings = append(typeStrings, t.String())
	}

	return nil, fmt.Errorf("no cell parser registered for any type in [%s], and %v does not implement encoding.TextUnmarshaler", strings.Join(typeStrings, ", "), t)

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

func registerCellParser(t reflect.Type, encoder, decoder interface{}) error {
	{
		if encoder == nil {
			glog.Warningf("encoder for %v is nil", t)
		} else {
			enc := reflect.TypeOf(encoder)
			if err := checkSignature(
				enc,
				[]reflect.Type{t},
				[]reflect.Type{stringType, errorInterfaceType}); err != nil {
				return fmt.Errorf("cell encoder doesn't match expected signature: %w", err)
			}
		}
	}
	if decoder == nil {
		return fmt.Errorf("decoder for %v is nil", t)
	}
	rdec := reflect.TypeOf(decoder)
	if err := checkSignature(
		rdec,
		[]reflect.Type{stringType, reflect.PtrTo(t)},
		[]reflect.Type{errorInterfaceType}); err != nil {
		return fmt.Errorf("cell parser decoder doesn't match expected signature: %w", err)
	}
	decFn := reflect.ValueOf(decoder)

	// Rather than completely overwrite the entry, keep it so that existing
	// references to it are not invalidated.
	existing := defaultRegistry.cellParsers[t]
	if existing == nil {
		existing = &registeredCellParser{nil}
		defaultRegistry.cellParsers[t] = existing
	}

	existing.impl = simpleCellParser(func(_ *CellContext, value string, field reflect.Value) error {
		out := decFn.Call([]reflect.Value{reflect.ValueOf(value), field.Addr()})[0].Interface()
		if out == nil {
			return nil
		}
		return out.(error)
	})
	return nil
}

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
