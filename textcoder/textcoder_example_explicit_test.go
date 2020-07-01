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

func Example_coderUsingByUnderlyingBasicType() {
	// Since x has an underlying basic type, its coder will be be used to
	// implement a Coder for x if x has no explicitly registered coder.
	type x int64

	v := x(3)
	err := Unmarshal("46", &v)
	fmt.Printf("%d, err = %v\n", v, err != nil)

	s, err := Marshal(x(77))
	fmt.Printf("%q, err = %v\n", s, err != nil)

	// Output:
	// 46, err = false
	// "77", err = false
}

func Example_a() {
	type x []string

	MustRegister(
		reflect.TypeOf(x{}),
		func(v x) (string, error) {
			return strings.Join([]string(v), " | "), nil
		},
		func(s string, dst *x) error {
			*dst = x(strings.Split(s, " | "))
			return nil
		})

	v := x{}
	err := Unmarshal("a | b | c", &v)
	fmt.Printf("%+v, err = %v\n", v, err != nil)

	s, err := Marshal(x{"hello", "world"})
	fmt.Printf("%s, err = %v\n", s, err != nil)

	// Output:
	// [a b c], err = false
	// hello | world, err = false
}

func Example_b() {
	r := NewRegistry()

	type i3 struct{ x, y, z int64 }

	r.Register(
		reflect.TypeOf(i3{}),
		func(v i3) (string, error) {
			return fmt.Sprintf("(%d, %d, %d)", v.x, v.y, v.z), nil
		},
		func(s string, dst *i3) error {
			ints, err := splitAndParseInts(s)
			if err != nil {
				return err
			}
			if len(ints) != 3 {
				return fmt.Errorf("got %d values, want 3", len(ints))
			}
			dst.x, dst.y, dst.z = ints[0], ints[1], ints[2]
			return nil
		})

	for _, input := range []string{"1,2,3", "1,2,5,6"} {
		vec := i3{}
		decoder := r.GetDecoder(reflect.TypeOf(vec))
		if err := decoder.DecodeText(nil, input, &vec); err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}
		fmt.Printf("(%d, %d, %d)\n", vec.x, vec.y, vec.z)
	}

	// Output:
	// (1, 2, 3)
	// error: got 4 values, want 3
}

func splitAndParseInts(s string) ([]int64, error) {
	var out []int64
	for _, ss := range strings.Split(s, ",") {
		i, err := strconv.Atoi(strings.TrimSpace(ss))
		if err != nil {
			return nil, err
		}
		out = append(out, int64(i))
	}
	return out, nil
}
