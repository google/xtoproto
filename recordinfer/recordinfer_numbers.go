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

package recordinfer

import (
	"strconv"

	pb "github.com/google/xtoproto/proto/recordtoproto"
)

type numberColumnType struct {
	floatingPoint bool
}

func (t *numberColumnType) protoType() string {
	if t.floatingPoint {
		return "float"
	}
	return "int64"
}

func (t *numberColumnType) protoImports() []string {
	return nil
}

func (t *numberColumnType) updateMapping(mapping *pb.ColumnToFieldMapping) {}

func inferFloat32Format(value string) (columnType, error) {
	if _, err := strconv.ParseFloat(value, 32); err == nil {
		return &numberColumnType{floatingPoint: true}, nil
	}
	return nil, nil
}

func inferInt64Format(value string) (columnType, error) {
	if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return &numberColumnType{floatingPoint: false}, nil
	}
	return nil, nil
}
