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

package csvcoder_test

import (
	"encoding/csv"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/xtoproto/csvcoder"
)

func Example_aRowParsing() {
	type mass float64

	type species struct {
		Name string `csv:"name"`
		Mass mass   `csv:"weight_kg"`
	}
	csvcoder.RegisterRowStruct(reflect.TypeOf(&species{}))

	redwood := &species{}
	err := csvcoder.ParseRow(
		csvcoder.NewRow(
			[]string{"Redwood", "1200000"},
			csvcoder.NewHeader([]string{"name", "weight_kg"}),
			csvcoder.RowNumber(10),
			"example.csv"),
		redwood)

	fmt.Printf("error: %v\n", err != nil)
	fmt.Printf("record: name = %q, mass = %f", redwood.Name, redwood.Mass)
	// Output:
	// error: false
	// record: name = "Redwood", mass = 1200000.000000
}

func Example_bFileParsing() {
	type mass float64

	type species struct {
		Name string  `csv:"name"`
		Mass float64 `csv:"weight_kg"`
	}
	csvcoder.RegisterRowStruct(reflect.TypeOf(&species{}))

	inputReader := csv.NewReader(strings.NewReader(`name,weight_kg
Redwood,1200000
"Blue whale",200000
`))

	fp, err := csvcoder.NewFileParser(inputReader, "in-memory.csv", &species{})
	if err != nil {
		fmt.Printf("NewFileParser error: %v", err)
		return
	}

	if err := fp.ReadAll(func(i interface{}) error {
		rec := i.(*species)
		fmt.Printf("record: name = %q, mass = %f\n", rec.Name, rec.Mass)
		return nil
	}); err != nil {
		fmt.Printf("ReadAll error: %v\n", err)
	}
	// Output:
	// record: name = "Redwood", mass = 1200000.000000
	// record: name = "Blue whale", mass = 200000.000000
}
