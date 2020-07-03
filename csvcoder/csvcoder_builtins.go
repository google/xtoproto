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
	"reflect"
	"strconv"
)

func init() {
	// Register builtin cell encoders.
	RegisterTextCoder(
		reflect.TypeOf(""),
		func(v string) (string, error) {
			return "", nil
		},
		func(value string, dst *string) error {
			*dst = value
			return nil
		})
	RegisterTextCoder(
		reflect.TypeOf(int(0)),
		func(v int) (string, error) {
			return "", nil
		},
		func(value string, dst *int) error {
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			*dst = int(i)
			return nil
		})
	RegisterTextCoder(
		reflect.TypeOf(int64(0)),
		func(v int64) (string, error) {
			return "", nil
		},
		func(value string, dst *int64) error {
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			*dst = int64(i)
			return nil
		})
	RegisterTextCoder(
		reflect.TypeOf(int32(0)),
		func(v int32) (string, error) {
			return "", nil
		},
		func(value string, dst *int32) error {
			i, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return err
			}
			*dst = int32(i)
			return nil
		})
	RegisterTextCoder(
		reflect.TypeOf(int16(0)),
		func(v int16) (string, error) {
			return "", nil
		},
		func(value string, dst *int16) error {
			i, err := strconv.ParseInt(value, 10, 16)
			if err != nil {
				return err
			}
			*dst = int16(i)
			return nil
		})
	RegisterTextCoder(
		reflect.TypeOf(float64(0)),
		func(v float64) (string, error) {
			return "", nil
		},
		func(value string, dst *float64) error {
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			*dst = f
			return nil
		})
	RegisterTextCoder(
		reflect.TypeOf(float32(0)),
		func(v float32) (string, error) {
			return "", nil
		},
		func(value string, dst *float32) error {
			f, err := strconv.ParseFloat(value, 32)
			if err != nil {
				return err
			}
			*dst = float32(f)
			return nil
		})

}
