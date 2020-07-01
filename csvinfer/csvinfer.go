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

// Package csvinfer guesses the types of CSV columns and uses these to generate a
// CSVProtoMapping object that in turn may be used to generate a .proto definition and
// CSV-to-proto parser.
package csvinfer

import (
	"encoding/csv"
	"strings"

	"github.com/google/xtoproto/recordinfer"
)

// InferProto returns a guess at the schema of a provided CSV sample. The input
// may be a full CSV file, but users should note that values are kept in memory during
// inference.
func InferProto(csvLines string, opts *recordinfer.Options) (*recordinfer.InferredProto, error) {
	reader := csv.NewReader(strings.NewReader(csvLines))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	b := recordinfer.NewRecordBasedInferrer(opts)

	for _, row := range rows {
		b.AddRow(row)
	}

	return b.Build()
}
