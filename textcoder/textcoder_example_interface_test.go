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

type r2 struct{ x, y float64 }

func (v *r2) UnmarshalText(text []byte) error {
	s := string(text)
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return fmt.Errorf("bad input to r2 decoder: %q", s)
	}
	x, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return err
	}
	y, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return err
	}
	v.x, v.y = x, y
	return nil
}

func Example_c() {
	r := NewRegistry()

	vec := r2{}
	decoder := r.GetDecoder(reflect.TypeOf(vec))
	if err := decoder.DecodeText(nil, "1.3,2.4", &vec); err != nil {
		fmt.Printf("error: %v", err)
		return
	}
	// Output: (1.3, 2.4)
	fmt.Printf("(%.1f, %.1f)\n", vec.x, vec.y)
}
