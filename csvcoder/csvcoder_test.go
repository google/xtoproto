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
	"encoding/csv"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func init() {
	RegisterRowStruct(reflect.TypeOf(&abee{}))
	RegisterRowStruct(reflect.TypeOf(&implicitFields{}))
	RegisterRowStruct(reflect.TypeOf(&measurements{}))
}

type abee struct {
	A string `csv:"A"`
	B int    `csv:"Bee"`
}

type implicitFields struct {
	A string
}

type measurements struct {
	// Both fields are backed by the same column.
	Dist  distance
	Dist2 *distance `csv:"Dist"`
}

type distance float64 // in meters

func (p *distance) UnmarshalText(textBytes []byte) error {
	s := string(textBytes)

	multiplier := 1.0
	for _, suf := range []struct {
		suffix     string
		multiplier float64
	}{
		{"km", 1000},
		{"Mm", 1000 * 1000},
		{"Gm", 1000 * 1000},
		{"mm", .001},
		{"cm", .01},
		{"m", 1},
	} {
		ss := strings.TrimSuffix(s, suf.suffix)
		if len(ss) != len(s) {
			s = ss
			multiplier = suf.multiplier
			break
		}
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return err
	}
	*p = distance(v * multiplier)
	return nil
}

func distancePtr(d distance) *distance { return &d }

func Test_dynamicParsable_ParseCSVRow(t *testing.T) {
	RegisterRowStruct(reflect.TypeOf(&abee{}))
}

func ExampleRegisterRowStruct() {
	type Pet struct {
		Name         string  `csv:"pet-name"`
		Internal     string  `csv-skip:""`
		WeightPounds float64 `csv:"weight-lb"`
	}
	RegisterRowStruct(reflect.TypeOf(&Pet{}))

	henry := &Pet{}
	err := ParseRow(NewRow(
		[]string{"ignored", "Henry", "32.15"},
		NewHeader([]string{"id", "pet-name", "weight-lb"}),
		1,
		"example.csv"),
		henry)

	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}

	fmt.Printf("%q weighs %d pounds", henry.Name, int(henry.WeightPounds))
	// Output: "Henry" weighs 32 pounds
}

func ExampleRegisterTextCoder() {

}

func TestParseCSVRow(t *testing.T) {
	type example struct {
		name           string
		header, values []string
		dst            interface{}
		want           interface{}
		wantErr        *regexp.Regexp
	}
	for _, tt := range []example{
		{
			"abee1",
			[]string{"Bee", "A"},
			[]string{"153", "y"},
			&abee{},
			&abee{"y", 153},
			nil,
		},
		{
			"abee2 - error",
			[]string{"Bee", "A"},
			[]string{"153x", "y"},
			&abee{},
			&abee{},
			regexp.MustCompile(`^test\.csv:124: .*153x`),
		},
		{
			"implicitFields",
			[]string{"Bee", "A"},
			[]string{"153x", "y"},
			&implicitFields{},
			&implicitFields{"y"},
			nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ParseRow(NewRow(tt.values, NewHeader(tt.header), 123, "test.csv"), tt.dst)
			checkErr(t, err, tt.wantErr, "ParseRow")
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, tt.dst); diff != "" {
				t.Errorf("unexpected diff (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestFileParser(t *testing.T) {
	type example struct {
		name                    string
		csvIn                   string
		prototype               interface{}
		want                    []interface{}
		wantNewErr, wantReadErr *regexp.Regexp
	}
	for _, tt := range []example{
		{
			"two lines abee",
			joinWithNewlines(`A,Bee`, `xy,42`, `66,45`),
			&abee{},
			[]interface{}{
				&abee{A: "xy", B: 42},
				&abee{A: "66", B: 45},
			},
			nil,
			nil,
		},
		{
			"abee - not enough columns",
			joinWithNewlines(`A,C`, `xy,42`, `66,45`),
			&abee{},
			[]interface{}{},
			regexp.MustCompile(`header row is missing.*"Bee"`),
			nil,
		},
		{
			"measurements",
			joinWithNewlines(`Dist,extra`, `50  ,x`, ` 50 km,`),
			&measurements{},
			[]interface{}{
				&measurements{Dist: 50, Dist2: distancePtr(50)},
				&measurements{Dist: 50000, Dist2: distancePtr(50000)},
			},
			nil,
			nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cr := csv.NewReader(strings.NewReader(tt.csvIn))
			fp, err := NewFileParser(cr, "test.csv", tt.prototype)
			checkErr(t, err, tt.wantNewErr, "NewFileParser")
			if err != nil {
				return
			}
			var got []interface{}
			err = fp.ReadAll(func(v interface{}) error {
				got = append(got, v)
				return nil
			})

			checkErr(t, err, tt.wantReadErr, "ReadAll")
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("unexpected diff (-want, +got):\n%s", diff)
			}
		})
	}
}

func checkErr(t *testing.T, err error, wantErr *regexp.Regexp, prefix string) {
	if gotErr, wantErr := err != nil, wantErr != nil; gotErr != wantErr {
		t.Fatalf("%s: got err %v, wantErr = %v", prefix, err, wantErr)
	}
	if err != nil && !wantErr.MatchString(err.Error()) {
		t.Fatalf("%s: got err %v, but it does not match expected message regexp %v", prefix, err, wantErr)
	}
	if err != nil {
		t.SkipNow()
	}
}

func joinWithNewlines(s ...string) string {
	return strings.Join(s, "\n")
}

func Test_ParseCell(t *testing.T) {
	type X int64

	RegisterTextCoder(
		reflect.TypeOf(X(3)),
		nil,
		func(value string, dst *X) error {
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			*dst = X(i)
			return nil
		})

	myX := X(5)

	if err := ParseCell(nil, "55", &myX); err != nil {
		t.Errorf("ParseCell() error: %v", err)
	}
	if myX != 55 {
		t.Errorf("got %v, want %v", myX, 55)
	}
}
