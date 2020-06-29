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
	"time"

	pb "github.com/google/xtoproto/proto/recordtoproto"
)

func candidateTimeColumnTypes(loc *time.Location) []*timeColumnType {
	create := func(layout string) *timeColumnType {
		return &timeColumnType{layout, loc}
	}
	return []*timeColumnType{
		create("2006-01-02 15:04:05"),
		create("2006-01-02 03:04:05 PM"),
		create("1/2/2006 3:04:05 PM"),
		create(time.ANSIC),
		create(time.UnixDate),
		create(time.RubyDate),
		create(time.RFC822),
		create(time.RFC822Z),
		create(time.RFC850),
		create(time.RFC1123),
		create(time.RFC1123Z),
		create(time.RFC3339),
		create(time.RFC3339Nano),
		create(time.Kitchen),
		create(time.Stamp),
		create(time.StampMilli),
		create(time.StampMicro),
		create(time.StampNano),
		create("2006-1-2"),
		create("2006/1/2"),
		create("20060102"),
	}
}

type timeColumnType struct {
	layout string
	loc    *time.Location
}

func (t *timeColumnType) protoType() string {
	return "google.protobuf.Timestamp"
}

func (t *timeColumnType) protoImports() []string {
	return []string{"google/protobuf/timestamp.proto"}
}

func (t *timeColumnType) updateMapping(mapping *pb.ColumnToFieldMapping) {
	tz := ""
	if t.loc != nil {
		tz = t.loc.String()
	}
	mapping.ParsingInfo = &pb.ColumnToFieldMapping_TimeFormat{
		TimeFormat: &pb.TimeFormat{
			GoLayout:     t.layout,
			TimeZoneName: tz,
		},
	}
}

func (t *timeColumnType) asInferrerFunc() func(string) (columnType, error) {
	return func(value string) (columnType, error) {
		var parseErr error
		if t.loc == nil {
			_, parseErr = time.Parse(t.layout, value)
		} else {
			_, parseErr = time.ParseInLocation(t.layout, value, t.loc)
		}
		if parseErr == nil {
			return t, nil
		}
		return nil, nil
	}
}

func timeFormatInferrers(loc *time.Location) []func(string) (columnType, error) {
	var out []func(string) (columnType, error)
	for _, ct := range candidateTimeColumnTypes(loc) {
		out = append(out, ct.asInferrerFunc())
	}
	return out
}
