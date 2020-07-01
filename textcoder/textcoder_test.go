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

package textcoder

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func ExampleRegistry_GetDecoder() {
	r := NewRegistry()

	r.Register(
		reflect.TypeOf(int(0)),
		func(v int) (string, error) {
			return strconv.Itoa(v), nil
		},
		func(s string, dst *int) error {
			i, err := strconv.Atoi(s)
			*dst = i
			return err
		})

	dec := r.GetDecoder(reflect.TypeOf(int(42)))

	i1 := 0
	dec.DecodeText(NewContext(), "43", &i1)
	fmt.Printf("%d\n", i1)

	err := dec.DecodeText(NewContext(), "nan", &i1)
	fmt.Printf("error: %v\n", err != nil)
	// Output:
	// 43
	// error: true
}

func ExampleRegistry_GetEncoder() {
	r := NewRegistry()

	r.Register(
		reflect.TypeOf(int(0)),
		func(v int) (string, error) {
			return strconv.Itoa(v), nil
		},
		func(s string, dst *int) error {
			i, err := strconv.Atoi(s)
			*dst = i
			return err
		})

	enc := r.GetEncoder(reflect.TypeOf(int(42)))

	s, err := enc.EncodeText(NewContext(), int(54))
	fmt.Printf("%q, %v\n", s, err != nil)

	// Output: "54", false
}

func TestRegister(t *testing.T) {
	type example struct {
		name             string
		t                reflect.Type
		encoder, decoder interface{}
		wantErr          *regexp.Regexp
	}
	var examples []example
	pushExample := func(ex example) {
		examples = append(examples, ex)
	}
	pushExample(example{
		"int64 - valid",
		reflect.TypeOf(int64(3)),
		func(int64) (string, error) { return "", nil },
		func(string, *int64) error { return nil },
		nil,
	})
	pushExample(example{
		"int64 - takes context",
		reflect.TypeOf(int64(3)),
		func(*Context, int64) (string, error) { return "", nil },
		func(*Context, string, *int64) error { return nil },
		nil,
	})
	pushExample(example{
		"int64 - not supported",
		reflect.TypeOf(int64(3)),
		func(int64) (string, error) { return "", nil },
		func(string) (int64, error) { return 0, nil },
		regexp.MustCompile("doesn't match any expected signature"),
	})
	{
		type abc struct{}
	}
	for _, tt := range examples {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			err := r.Register(tt.t, tt.encoder, tt.decoder)
			checkErr(t, err, tt.wantErr, "Register")
			if err != nil {
				if r.getExplicit(tt.t) != nil {
					t.Errorf("registration failed, but %v is registered", tt.t)
				}
				return
			}
			if r.getExplicit(tt.t) == nil {
				t.Errorf("registration succeeded, but %v is not registered", tt.t)
			}
		})
	}
}

func TestDefaultEncoders(t *testing.T) {
	type example struct {
		name    string
		value   interface{}
		want    string
		wantErr *regexp.Regexp
	}
	var examples []example
	pushExample := func(ex example) {
		examples = append(examples, ex)
	}
	pushExample(example{
		"float64",
		// Encode a value bigger than the biggest float32
		float64(math.MaxFloat32 * 8),
		"2722258773108230878493633467876135403520.000000",
		nil,
	})
	pushExample(example{
		"float32",
		float32(math.MaxFloat32 / 8),
		"42535293329816107476463022935564615680.000000",
		nil,
	})
	pushExample(example{"string", "abc", "abc", nil})
	pushExample(example{"int8", int8(-7), "-7", nil})
	pushExample(example{"int16", int16(-7), "-7", nil})
	pushExample(example{"int32", int32(-7), "-7", nil})
	pushExample(example{"int64", int64(-7), "-7", nil})
	pushExample(example{"int", int(-7), "-7", nil})
	pushExample(example{"uint8", uint8(7), "7", nil})
	pushExample(example{"uint16", uint16(7), "7", nil})
	pushExample(example{"uint32", uint32(7), "7", nil})
	pushExample(example{"uint64", uint64(7), "7", nil})
	pushExample(example{"uint", uint(7), "7", nil})
	pushExample(example{"bool", bool(true), "true", nil})
	pushExample(example{"bool", bool(false), "false", nil})

	pushExample(example{"distance", distance(42), "42.000000", nil})
	for _, tt := range examples {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(tt.value)
			checkErr(t, err, tt.wantErr, "Register")
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("got %q, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultDecoders(t *testing.T) {
	type example struct {
		name      string
		input     string
		dst, want interface{}
		wantErr   *regexp.Regexp
	}
	var examples []example
	pushExample := func(ex example) {
		examples = append(examples, ex)
	}
	pushExample(example{
		"float64",
		"2722258773108230878493633467876135403520.000000",
		// Encode a value bigger than the biggest float32
		float64Ptr(0),
		float64Ptr(2722258773108230878493633467876135403520.000000),
		nil,
	})
	pushExample(example{
		"float64",
		"2722258773108230878493633467876135403520.000000",
		// Encode a value bigger than the biggest float32
		float32Ptr(0),
		float32Ptr(0),
		regexp.MustCompile("value out of range"),
	})
	pushExample(example{
		"float32",
		"42535293329816107476463022935564615680.000000",
		float32Ptr(0),
		float32Ptr(42535293329816107476463022935564615680.000000),
		nil,
	})
	pushExample(example{"string", "abc", stringPtr(""), stringPtr("abc"), nil})
	pushExample(example{"int8", "-7", int8Ptr(0), int8Ptr(-7), nil})
	pushExample(example{"int16", "-7", int16Ptr(0), int16Ptr(-7), nil})
	pushExample(example{"int32", "-7", int32Ptr(0), int32Ptr(-7), nil})
	pushExample(example{"int64", "-7", int64Ptr(0), int64Ptr(-7), nil})
	pushExample(example{"int", "-7", intPtr(0), intPtr(-7), nil})
	pushExample(example{"uint", "7", uintPtr(0), uintPtr(7), nil})
	pushExample(example{"uint8", "7", uint8Ptr(0), uint8Ptr(7), nil})
	pushExample(example{"uint16", "7", uint16Ptr(0), uint16Ptr(7), nil})
	pushExample(example{"uint32", "7", uint32Ptr(0), uint32Ptr(7), nil})
	pushExample(example{"uint64", "7", uint64Ptr(0), uint64Ptr(7), nil})

	pushExample(example{"bool", "yes", boolPtr(false), boolPtr(true), nil})
	pushExample(example{"bool", "no", boolPtr(false), boolPtr(false), nil})
	pushExample(example{"bool", "true", boolPtr(false), boolPtr(true), nil})
	pushExample(example{"bool", "FALSE", boolPtr(false), boolPtr(false), nil})
	pushExample(example{"bool", "1", boolPtr(false), boolPtr(true), nil})
	pushExample(example{"bool", "0", boolPtr(false), boolPtr(false), nil})

	pushExample(example{"bool", " TRUE ", boolPtr(false), boolPtr(false), regexp.MustCompile("unsupported bool value")})
	pushExample(example{"bool", "124", boolPtr(false), boolPtr(false), regexp.MustCompile("unsupported bool value")})

	pushExample(example{"uint", "-7", uintPtr(0), uintPtr(0), regexp.MustCompile("invalid syntax")})
	pushExample(example{"uint8", "-7", uint8Ptr(0), uint8Ptr(0), regexp.MustCompile("invalid syntax")})
	pushExample(example{"uint16", "-7", uint16Ptr(0), uint16Ptr(0), regexp.MustCompile("invalid syntax")})
	pushExample(example{"uint32", "-7", uint32Ptr(0), uint32Ptr(0), regexp.MustCompile("invalid syntax")})
	pushExample(example{"uint64", "-7", uint64Ptr(0), uint64Ptr(0), regexp.MustCompile("invalid syntax")})

	pushExample(example{
		"distance - underly type float64",
		"1600",
		distancePtr(0),
		distancePtr(1600),
		nil,
	})

	for _, tt := range examples {
		t.Run(tt.name, func(t *testing.T) {
			err := Unmarshal(tt.input, tt.dst)
			checkErr(t, err, tt.wantErr, "Unmarshal")
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, tt.dst); diff != "" {
				t.Errorf("unexpected diff from Unmarshal (-want, +got):\n%s", diff)
			}
		})
	}
}

func checkErr(t *testing.T, err error, wantErr *regexp.Regexp, prefix string) {
	if gotErr, wantErr := err != nil, wantErr != nil; gotErr != wantErr {
		t.Fatalf("%s: got err %v; wantErr = %v", prefix, err, wantErr)
	}
	if err != nil && !wantErr.MatchString(err.Error()) {
		t.Fatalf("%s: got err %v; this error does not match expected message regexp %v", prefix, err, wantErr)
	}
	if err != nil {
		t.SkipNow()
	}
}

func boolPtr(v bool) *bool          { return &v }
func uintPtr(v uint) *uint          { return &v }
func uint8Ptr(v uint8) *uint8       { return &v }
func uint16Ptr(v uint16) *uint16    { return &v }
func uint32Ptr(v uint32) *uint32    { return &v }
func uint64Ptr(v uint64) *uint64    { return &v }
func intPtr(v int) *int             { return &v }
func int8Ptr(v int8) *int8          { return &v }
func int16Ptr(v int16) *int16       { return &v }
func int32Ptr(v int32) *int32       { return &v }
func int64Ptr(v int64) *int64       { return &v }
func stringPtr(v string) *string    { return &v }
func float32Ptr(v float32) *float32 { return &v }
func float64Ptr(v float64) *float64 { return &v }

type distance float64                  // distance in meters
func distancePtr(v distance) *distance { return &v }
