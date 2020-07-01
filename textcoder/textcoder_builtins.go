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
	"reflect"
	"strconv"
	"strings"
)

var (
	boolValues = map[string]bool{
		"true":  true,
		"false": false,
		"1":     true,
		"0":     false,
		"on":    true,
		"off":   false,
		"yes":   true,
		"no":    false,
	}
)

// RegisterBasicTypes attempts to register coders for the following types:
// int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
// string, float32, float64.
func RegisterBasicTypes(r *Registry) error {
	var errors []error
	putErr := func(err error) {
		errors = append(errors, err)
	}

	// int, int8, int16, int32, int64
	putErr(r.Register(
		reflect.TypeOf(int(0)),
		func(v int) (string, error) { return strconv.Itoa(v), nil },
		func(s string, dst *int) error {
			i, err := strconv.Atoi(s)
			*dst = i
			return err
		}))
	putErr(r.Register(
		reflect.TypeOf(bool(true)),
		func(v bool) (string, error) {
			if v {
				return "true", nil
			}
			return "false", nil
		},
		func(s string, dst *bool) error {
			got, ok := boolValues[strings.ToLower(s)]
			if !ok {
				return fmt.Errorf("unsupported bool value %q", s)
			}
			*dst = got
			return nil
		}))
	putErr(r.Register(
		reflect.TypeOf(int8(0)),
		func(v int8) (string, error) { return fmt.Sprintf("%d", v), nil },
		func(value string, dst *int8) error {
			i, err := strconv.ParseInt(value, 10, 8)
			if err != nil {
				return err
			}
			*dst = int8(i)
			return nil
		}))
	putErr(r.Register(
		reflect.TypeOf(int32(0)),
		func(v int32) (string, error) { return fmt.Sprintf("%d", v), nil },
		func(value string, dst *int32) error {
			i, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return err
			}
			*dst = int32(i)
			return nil
		}))
	putErr(r.Register(
		reflect.TypeOf(int16(0)),
		func(v int16) (string, error) { return fmt.Sprintf("%d", v), nil },
		func(value string, dst *int16) error {
			i, err := strconv.ParseInt(value, 10, 16)
			if err != nil {
				return err
			}
			*dst = int16(i)
			return nil
		}))
	putErr(r.Register(
		reflect.TypeOf(int64(0)),
		func(v int64) (string, error) { return fmt.Sprintf("%d", v), nil },
		func(value string, dst *int64) error {
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			*dst = int64(i)
			return nil
		}))

	// uint, uint8, uint16, uint32, uint64
	putErr(r.Register(
		reflect.TypeOf(uint(0)),
		func(v uint) (string, error) { return fmt.Sprintf("%d", v), nil },
		func(value string, dst *uint) error {
			i, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return err
			}
			*dst = uint(i)
			return nil
		}))
	putErr(r.Register(
		reflect.TypeOf(uint8(0)),
		func(v uint8) (string, error) { return fmt.Sprintf("%d", v), nil },
		func(value string, dst *uint8) error {
			i, err := strconv.ParseUint(value, 10, 8)
			if err != nil {
				return err
			}
			*dst = uint8(i)
			return nil
		}))
	putErr(r.Register(
		reflect.TypeOf(uint32(0)),
		func(v uint32) (string, error) { return fmt.Sprintf("%d", v), nil },
		func(value string, dst *uint32) error {
			i, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return err
			}
			*dst = uint32(i)
			return nil
		}))
	putErr(r.Register(
		reflect.TypeOf(uint16(0)),
		func(v uint16) (string, error) { return fmt.Sprintf("%d", v), nil },
		func(value string, dst *uint16) error {
			i, err := strconv.ParseUint(value, 10, 16)
			if err != nil {
				return err
			}
			*dst = uint16(i)
			return nil
		}))
	putErr(r.Register(
		reflect.TypeOf(uint64(0)),
		func(v uint64) (string, error) { return fmt.Sprintf("%d", v), nil },
		func(value string, dst *uint64) error {
			i, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return err
			}
			*dst = uint64(i)
			return nil
		}))

	// float32, float64
	putErr(r.Register(
		reflect.TypeOf(float64(0)),
		func(v float64) (string, error) { return fmt.Sprintf("%f", v), nil },
		func(value string, dst *float64) error {
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			*dst = f
			return nil
		}))
	putErr(r.Register(
		reflect.TypeOf(float32(0)),
		func(v float32) (string, error) { return fmt.Sprintf("%f", v), nil },
		func(value string, dst *float32) error {
			f, err := strconv.ParseFloat(value, 32)
			if err != nil {
				return err
			}
			*dst = float32(f)
			return nil
		}))

	putErr(r.Register(
		reflect.TypeOf(""),
		func(v string) (string, error) { return v, nil },
		func(value string, dst *string) error {
			*dst = value
			return nil
		}))

	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}
