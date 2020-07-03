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

package xmlinfer

import (
	"fmt"
	"strconv"

	"github.com/jhump/protoreflect/desc/builder"
)

func inferFieldTypeFromExampleStrings(exampleCounts map[string]int) (*builder.FieldType, error) {
	total := 0
	var examples []string
	for value, c := range exampleCounts {
		total += c
		examples = append(examples, value)
	}
	if total == 0 {
		return builder.FieldTypeString(), nil
	}
	for _, parser := range []struct {
		ft   func() *builder.FieldType
		pred func(string) bool
	}{
		{
			builder.FieldTypeInt64,
			func(s string) bool {
				_, err := strconv.ParseInt(s, 10, 64)
				return err == nil
			},
		},
		{
			builder.FieldTypeDouble,
			func(s string) bool {
				_, err := strconv.ParseFloat(s, 64)
				return err == nil
			},
		},
		{
			builder.FieldTypeString,
			func(s string) bool {
				return true
			},
		},
	} {
		if allStringsPass(examples, parser.pred) {
			return parser.ft(), nil
		}
	}
	return nil, fmt.Errorf("failed to infer type from strings %v", examples)
}

type enumInferrer struct {
	// minExampleCount is the minimum number of examples the inferrer must see
	// to infer an enum value.
	minExampleCount int

	// uniqueValuesOverCountThreshold specifies how many unique values
	uniqueValuesOverCountThreshold float64

	// maxStringLength is the maximum permitted value
	maxStringLength int

	// If true, whitespace is trimmed before performinginference.
	trimWhitespace bool
}

func allStringsPass(strs []string, pred func(string) bool) bool {
	for _, s := range strs {
		if !pred(s) {
			return false
		}
	}
	return true
}
