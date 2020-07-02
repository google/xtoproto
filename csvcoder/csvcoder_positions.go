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

// ColumnNumber and RowNumber definitions.

// ColumnNumber is used instead of an int for column indexes.
type ColumnNumber int

// InvalidColumn returns an invalid column number.
const InvalidColumn ColumnNumber = -1

// IsValid returns whether the column refers to a valid column in the CSV file or or not.
func (n ColumnNumber) IsValid() bool {
	return n >= 0
}

// Offset returns the offset of the value within a row.
func (n ColumnNumber) Offset() int {
	return int(n)
}

// RowNumber is used instead of an int for representing the position of a row in a CSV file.
type RowNumber int

// InvalidRow returns an invalid row number.
const InvalidRow RowNumber = -1

// IsValid returns whether the row refers to a valid row in the CSV file or or not.
func (n RowNumber) IsValid() bool {
	return n >= 0
}

// Offset returns the offset of the row. The first row has offset 0.
func (n RowNumber) Offset() int {
	return int(n)
}

// Ordinal returns the 1-based offset of the row. The first row has ordinal value 1.
func (n RowNumber) Ordinal() int {
	return n.Offset() + 1
}
