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

// Package textcoder defines a registry of Go types and associated textual
// encoding/decoding functions.
//
// Unlike the standard library's encoding package, textcoder allows any package
// to define a coder for a type instead of requiring methods to be defined on
// the type itself.
//
// For each type T registered in the default registry (using Register or
// MustRegister), textcoder.DefaultRegistry().GetCoder(reflect.TypeOf(t)) will
// return a Coder object for marshaling and unmarshaling a textual value. The
// Coder object returned has an EncodeText() method that takes *Context and a
// second argument of type T. The DecodeText method of the coder takes a
// *Context, a string, and an interace{} type that should be a non-nil pointer
// of type *T.
//
// If a type T implements the encoding.TextUnmarshaler interface, a
// Registry.GetDecoder(T) will return a Decoder that dispatches to
// UnmarshalText; the same is true of encoding.TextMarshaler and GetEncoder.
//
// If a type T has an underlying type that is a basic type (bool, int, string,
// uint, uint8, float32, etc.), textcoder.Registry.GetCoder, GetEncoder, and
// GetDecoder will return a coder for T based on the underlying type. This
// allows types like `type distance float64` to use float64's encoder. Due to
// limitations of Go's reflect package, which does not support obtaining the
// underlying type of a named type, this functionality is limited to types with
// an underlying basic type (see https://github.com/golang/go/issues/39574).
//
// Currently the registry does not attempt to find registered coders based on
// registered interface types. While coders may be registered for an interface
// type, that coder only be returned then the reflect.Type of the interface is
// used to obtain a Coder, Encoder, or Decoder; the coder will not be returned
// if a coder is requested of a type that implements the registered interface.
// This behavior is likely to change in the future.
//
// The types string, int, uint, float64, float32, uint8, int8, uint16, int16,
// uint32, int32, uint64, and int64 have coders registered in the default
// registry. This means these basic types can be encoded and decoded from
// strings. Most of these types use the functions in strconv to parse and
// fmt.Sprintf("%d" or "%f") to format. The bool coder is case insensitive and
// accepts values like "true", "YeS", "on", and "1". These coders may be added
// to other registries using RegisterBasicTypes().
//
// See the examples for usage.
package textcoder

import (
	"encoding"
	"fmt"
	"reflect"
)

var (
	textUnmarshalerInterface = func() reflect.Type {
		var i encoding.TextUnmarshaler
		ptr := &i
		return reflect.TypeOf(ptr).Elem()
	}()
	textMarshalerInterface = func() reflect.Type {
		var i encoding.TextMarshaler
		ptr := &i
		return reflect.TypeOf(ptr).Elem()
	}()

	defaultRegistry = func() *Registry {
		r := NewRegistry()
		must(RegisterBasicTypes(r))
		return r
	}()
)

// T is used in place of interface{} to represent a templated type. When go adds
// generics, we can revisit the API.
type T = interface{}

// Encoder encodes an argument of type T into a string.
type Encoder interface {
	// EncodeText returns the textual form of the value. The value should be of
	// type T where T is a registered type.
	EncodeText(ctx *Context, value T) (string, error)
}

// Decoder decodes a textual representation of a value into dstValuePointer,
// which is of type T*.
type Decoder interface {
	// DecodeText decodes the given text into the destination dst. dst must not
	// be nil.
	DecodeText(ctx *Context, text string, dst T) error
}

// Coder implements both Encoder and Decoder for the same type.
type Coder interface {
	Encoder
	Decoder
}

// DefaultRegistry returns the default registry for registring text coders.
func DefaultRegistry() *Registry {
	return defaultRegistry
}

type coderMap = map[reflect.Type]*registryEntry

// Registry is a set of registered text coders.
//
// Typically users will use the default registry, but creating a specialized
// Registry object is fully supported.
type Registry struct {
	coders coderMap
}

// NewRegistry returns a new object for registring text coders.
func NewRegistry() *Registry {
	return &Registry{make(coderMap)}
}

// NewContext returns a new context that uses this registry for textual encoding
// purposes.
func (r *Registry) NewContext() *Context {
	return NewContext().WithValue(registryKey, r)
}

// Register registers an encoding function and a decoding function for parsing
// and printing values of the provided type.
//
// If the provided type is T, the decoder should have one of the following
// signatures:
//
// 1. func(*textcoder.Context, *T) error
//
// 2. func(string, *T) error
//
// The encoder should have one of the following signatures:
//
// 1. func(*textcoder.Context, T) (string, error).
//
// 2. func(T) (string, error).
//
// Encoders and decoders should take a Context argument if they need to make
// nested calls to textcoder functions.
func (r *Registry) Register(t reflect.Type, encoder, decoder interface{}) error {
	return register(r, t, encoder, decoder)
}

// GetDecoder returns the decoder for the given type or nil.
//
// The decoder will be determined based on applying the following rules in
// order:
//
// 1. If the type is explicitly registered because of a previous call to
// r.Register(t), r.GetDecoder(t) will return that decoder.
//
// 2. If the type implements encoding.TextUnmarshaler interface, GetDecoder(t)
// returns an decoder that dispatches to UnmarshalText.
//
// 3. If the type has an underlying type that is a basic type (bool, int,
// string, uint, uint8, float32, etc.), GetDecoder(t) will return a decoder for
// t based on the underlying type.
func (r *Registry) GetDecoder(t reflect.Type) Decoder {
	explicit := r.getExplicit(t)
	if explicit != nil {
		return explicit
	}
	if reflect.PtrTo(t).Implements(textUnmarshalerInterface) {
		return &textEncodingCoder{}
	}
	if basicCoder, _ := r.getEntryForUnderlying(t); basicCoder != nil {
		return r.GetCoder(t)
	}
	return nil
}

// GetEncoder returns the encoder for the given type or nil.
//
// The encoder will be determined based on applying the following rules in
// order:
//
// 1. If the type is explicitly registered because of a previous call to
// r.Register(t), r.GetEncoder(t) will return that encoder.
//
// 2. If the type implements encoding.TextMarshaler interface, GetEncoder(t)
// returns an encoder that dispatches to MarshalText.
//
// 3. If the type has an underlying type that is a basic type (bool, int,
// string, uint, uint8, float32, etc.), GetEncoder(t) will return a encoder for
// t based on the underlying type.
func (r *Registry) GetEncoder(t reflect.Type) Encoder {
	explicit := r.getExplicit(t)
	if explicit != nil {
		return explicit
	}
	if reflect.PtrTo(t).Implements(textMarshalerInterface) {
		return &textEncodingCoder{}
	}
	if basicCoder, _ := r.getEntryForUnderlying(t); basicCoder != nil {
		return r.GetCoder(t)
	}
	return nil
}

// GetCoder returns the Coder for the given type or nil if no coder can be
// determined. The coder can be used to marshal and unmarshal textual
// representations of values of the given type.
//
// The coder will be determined based on applying the following rules in order.
// Let T be the argument to GetCoder:
//
// 1. If the type is explicitly registered because of a previous call to
// r.Register(t), r.GetCoder(t) will return that coder.
//
// 2. If the type implements encoding.TextUnmarshaler and encoding.TextMarshaler
// interface, GetCoder(t) returns a Decoder that dispatches to those methods.
//
// 3. If the type has an underlying type that is a basic type (bool, int,
// string, uint, uint8, float32, etc.), GetCoder(t) will return a coder for T
// based on the underlying type. This allows types like `type distance float64`
// to use float64's Coder. Due to limitations of Go's reflect package, which
// does not support obtaining the underlying type of a named type, this
// functionality is limited to types with an underlying basic type (see
// https://github.com/golang/go/issues/39574).
//
// Currently the registry does not attempt to find registered coders based on
// registered interface types. While coders may be registered for an interface
// type, that coder only be returned then the reflect.Type of the interface is
// used to obtain a Coder, Encoder, or Decoder; the coder will not be returned
// if a coder is requested of a type that implements the registered interface.
// This behavior is likely to change in the future.
//
// The types string, int, uint, float64, float32, uint8, int8, uint16, int16,
// uint32, int32, uint64, and int64 have coders registered in the default
// registry. This means these basic types can be encoded and decoded from
// strings. Most of these types use the functions in strconv to parse and
// fmt.Sprintf("%d" or "%f") to format. The bool coder is case insensitive and
// accepts values like "true", "YeS", "on", and "1". These coders may be added
// to other registries using RegisterBasicTypes().
func (r *Registry) GetCoder(t reflect.Type) Coder {
	explicit := r.getExplicit(t)
	if explicit != nil {
		return explicit
	}
	tPtr := reflect.PtrTo(t)
	if tPtr.Implements(textUnmarshalerInterface) && tPtr.Implements(textMarshalerInterface) {
		return &textEncodingCoder{}
	}
	if basicCoder, underlyingType := r.getEntryForUnderlying(t); basicCoder != nil {
		return &registryEntry{
			encode: func(ctx *Context, value T) (string, error) {
				return basicCoder.EncodeText(ctx, reflect.ValueOf(value).Convert(underlyingType).Interface())
			},
			decode: func(ctx *Context, text string, dst T) error {
				dstBasicReflect := reflect.New(underlyingType)
				err := basicCoder.DecodeText(ctx, text, dstBasicReflect.Interface())
				// roughly equivalent to *dst = T(*dstBasic) where dst is of
				// type T and dstBasic is of type U, and T is convertible to U.
				reflect.ValueOf(dst).Elem().Set(dstBasicReflect.Elem().Convert(t))
				return err
			},
		}
	}
	return nil
}

func (r *Registry) getExplicit(t reflect.Type) *registryEntry {
	return r.coders[t]
}

func (r *Registry) setExplicit(t reflect.Type, e *registryEntry) {
	r.coders[t] = e
}

// getEntryForUnderlying returns the registryEntry for the underlying type of t,
// if one exists.
//
// This is only implemented for primitive types because reflect provides no way
// of getting the underlying type of a named type: see
// https://github.com/golang/go/issues/39574.
func (r *Registry) getEntryForUnderlying(t reflect.Type) (*registryEntry, reflect.Type) {
	basicType := typeOfBasicKind(t.Kind())
	if basicType == nil {
		return nil, nil
	}
	explicit := r.getExplicit(basicType)
	if explicit == nil {
		return nil, nil
	}
	return explicit, basicType
}

// MustRegister registers an encoding function and a decoding function for
// parsing and printing values of the provided type. This function panics if
// registration fails; use Register if a panic is unacceptable.
//
// To use the default registry, most users should make calls to MustRegister in
// an init() function.
//
// See Registry.Register for details of the signatures of encoder and decoder.
func MustRegister(t reflect.Type, encoder, decoder interface{}) {
	if err := Register(t, encoder, decoder); err != nil {
		panic(err)
	}
}

// Register registers an encoding function and a decoding function
// for parsing and printing values of the provided type.
//
// See Registry.Register for details of the signatures of encoder and decoder.
func Register(t reflect.Type, encoder, decoder interface{}) error {
	return DefaultRegistry().Register(t, encoder, decoder)
}

// Context is passed to the encoder or decoder as a weigh of passing arbitrary
// contextual formatting information.
type Context struct {
	values map[interface{}]interface{}
}

const registryKey = "registry"

// NewContext returns a new Context to use when parsing within a given
// Row.
func NewContext() *Context {
	c := &Context{make(map[interface{}]interface{})}
	c.values[registryKey] = DefaultRegistry()
	return c
}

// Registry returns the registry to use for encoding and decoding textual values.
func (c *Context) Registry() *Registry {
	v, _ := c.Value(registryKey)
	return v.(*Registry)
}

// Value returns a value associated with the given key.
//
// This may be used to pass contextual information to the text coder that may be
// relevant for printing out the
func (c *Context) Value(key interface{}) (interface{}, bool) {
	v, ok := c.values[key]
	return v, ok
}

// WithValue returns a new context with an additional key and value.
//
// WithValue does not modify the receiver.
func (c *Context) WithValue(key, value interface{}) *Context {
	n := NewContext()
	for k, v := range c.values {
		n.values[k] = v
	}
	n.values[key] = value
	return n
}

// Unmarshal decodes a textual value into dst.
func Unmarshal(value string, dst T) error {
	return UnmarshalContext(NewContext(), value, dst)
}

// UnmarshalContext is like Unmarshal, but it takes an extra context argument.
//
// To use a registered coder of type T, dst should be of type *T.
func UnmarshalContext(ctx *Context, value string, dst T) error {
	t := reflect.TypeOf(dst)
	if t.Kind() != reflect.Ptr {
		return fmt.Errorf("Unmarshal requires a pointer argument, got type %v", t)
	}
	dec := DefaultRegistry().GetDecoder(t.Elem())
	if dec == nil {
		return fmt.Errorf("no registered decoder for type %v", t)
	}
	return dec.DecodeText(NewContext(), value, dst)
}

// MarshalContext attempts to encode the value into a string using one of the
// default registered coders.
//
// To use a registered coder of type T, value should be of type T.
func MarshalContext(ctx *Context, value T) (string, error) {
	t := reflect.TypeOf(value)
	e := DefaultRegistry().GetEncoder(t)
	if e == nil {
		return "", fmt.Errorf("no encoder registered of type %v", t)
	}
	return e.EncodeText(ctx, value)
}

// Marshal attempts to encode the value into a string using one of the default
// registered coders.
//
// To use a registered coder of type T, value should be of type T.
func Marshal(value T) (string, error) {
	return MarshalContext(DefaultRegistry().NewContext(), value)
}

// registryEntry is the result of calling register and implements the Coder interface.
type registryEntry struct {
	decode func(ctx *Context, text string, dst T) error
	encode func(ctx *Context, value T) (string, error)
}

func (cp *registryEntry) DecodeText(ctx *Context, text string, dst T) error {
	return cp.decode(ctx, text, dst)
}

func (cp *registryEntry) EncodeText(ctx *Context, value T) (string, error) {
	return cp.encode(ctx, value)
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

func register(r *Registry, t reflect.Type, encoder, decoder interface{}) error {
	if encoder == nil {
		return fmt.Errorf("encoder for %v is nil", t)
	}
	encFn, err := createEncoderFn(t, reflect.ValueOf(encoder))
	if err != nil {
		return err
	}

	if decoder == nil {
		return fmt.Errorf("decoder for %v is nil", t)
	}

	decFn, err := createDecoderFn(t, reflect.ValueOf(decoder))
	if err != nil {
		return err
	}

	// Rather than completely overwrite the entry, keep it so that existing
	// references to it are not invalidated.
	existing := r.getExplicit(t)
	if existing == nil {
		existing = &registryEntry{nil, nil}
		r.setExplicit(t, existing)
	}
	existing.decode = decFn
	existing.encode = encFn
	return nil
}

func createEncoderFn(t reflect.Type, encoder reflect.Value) (func(_ *Context, value interface{}) (string, error), error) {
	sig1Err := checkSignature(
		encoder.Type(),
		[]reflect.Type{t},
		[]reflect.Type{stringType, errorInterfaceType})
	if sig1Err == nil {
		return func(_ *Context, value interface{}) (string, error) {
			out := encoder.Call([]reflect.Value{
				reflect.ValueOf(value),
			})
			var err error
			if errIface := out[1].Interface(); errIface != nil {
				err = errIface.(error)
			}
			return out[0].Interface().(string), err
		}, nil
	}

	sig2Err := checkSignature(
		encoder.Type(),
		[]reflect.Type{reflect.TypeOf(&Context{}), t},
		[]reflect.Type{stringType, errorInterfaceType})
	if sig2Err == nil {
		return func(ctx *Context, value interface{}) (string, error) {
			out := encoder.Call([]reflect.Value{
				reflect.ValueOf(ctx),
				reflect.ValueOf(value),
			})
			var err error
			if errIface := out[1].Interface(); errIface != nil {
				err = errIface.(error)
			}
			return out[0].Interface().(string), err
		}, nil
	}

	return nil, fmt.Errorf("text encoder doesn't match any expected signature:\n  %v\n  %v", sig1Err, sig2Err)

}

func createDecoderFn(t reflect.Type, decoder reflect.Value) (func(_ *Context, text string, dst interface{}) error, error) {
	errSig1 := checkSignature(
		decoder.Type(),
		[]reflect.Type{stringType, reflect.PtrTo(t)},
		[]reflect.Type{errorInterfaceType})

	if errSig1 == nil {
		return func(_ *Context, text string, dst interface{}) error {
			out := decoder.Call([]reflect.Value{
				reflect.ValueOf(text),
				reflect.ValueOf(dst)},
			)[0].Interface()
			if out == nil {
				return nil
			}
			return out.(error)
		}, nil
	}

	errSig2 := checkSignature(
		decoder.Type(),
		[]reflect.Type{reflect.TypeOf(&Context{}), stringType, reflect.PtrTo(t)},
		[]reflect.Type{errorInterfaceType})

	if errSig2 == nil {
		return func(ctx *Context, text string, dst interface{}) error {
			out := decoder.Call([]reflect.Value{
				reflect.ValueOf(ctx),
				reflect.ValueOf(text),
				reflect.ValueOf(dst)},
			)[0].Interface()
			if out == nil {
				return nil
			}
			return out.(error)
		}, nil
	}

	return nil, fmt.Errorf("text decoder doesn't match any expected signature:\n  %v\n  %v", errSig1, errSig2)
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

// textEncodingCoder implements Coder using the TextUnmarshaler interface of the
// value.
type textEncodingCoder struct{}

func (c *textEncodingCoder) DecodeText(_ *Context, text string, dst T) error {
	um, ok := dst.(encoding.TextUnmarshaler)
	if !ok {
		return fmt.Errorf("dst does not implement encoding.TextUnmarshaler: %v of type %v", dst, reflect.TypeOf(dst))
	}
	return um.UnmarshalText([]byte(text))
}

func (c *textEncodingCoder) EncodeText(_ *Context, value T) (string, error) {
	m, ok := value.(encoding.TextMarshaler)
	if !ok {
		return "", fmt.Errorf("value does not implement encoding.TextMarshaler: %v of type %v", value, reflect.TypeOf(value))
	}
	text, err := m.MarshalText()
	return string(text), err
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

var typeOfBasicKindMap = map[reflect.Kind]reflect.Type{
	reflect.Bool:       reflect.TypeOf(bool(true)),
	reflect.Int:        reflect.TypeOf(int(0)),
	reflect.Int8:       reflect.TypeOf(int8(0)),
	reflect.Int16:      reflect.TypeOf(int16(0)),
	reflect.Int32:      reflect.TypeOf(int32(0)),
	reflect.Int64:      reflect.TypeOf(int64(0)),
	reflect.Uint:       reflect.TypeOf(uint(0)),
	reflect.Uint8:      reflect.TypeOf(uint8(0)),
	reflect.Uint16:     reflect.TypeOf(uint16(0)),
	reflect.Uint32:     reflect.TypeOf(uint32(0)),
	reflect.Uint64:     reflect.TypeOf(uint64(0)),
	reflect.Uintptr:    reflect.TypeOf(uintptr(0)),
	reflect.Float32:    reflect.TypeOf(float32(0)),
	reflect.Float64:    reflect.TypeOf(float64(0)),
	reflect.Complex64:  reflect.TypeOf(complex64(0)),
	reflect.Complex128: reflect.TypeOf(complex128(0)),
	reflect.String:     reflect.TypeOf(string("")),
}

func typeOfBasicKind(k reflect.Kind) reflect.Type {
	return typeOfBasicKindMap[k]
}
