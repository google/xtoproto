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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	pb "github.com/google/xtoproto/proto/recordtoproto"
)

var montreal = mustLoadLocation("America/Montreal")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func TestRecordBasedInferrer(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rows    [][]string
		opts    *Options
		want    *pb.RecordProtoMapping
		wantErr bool
	}{
		{
			name: "test 1",
			rows: [][]string{
				{"AlphaBet", "BBFriendOfMine_1", "cee cee", "d__u_de", "XTimeUTC"},
				{"1", "blah", "2016-10-1", "1", "6/13/2019 3:00:00 AM"},
				{"2", "1", "2016-10-1", "2.0", "6/13/2019 3:00:00 AM"},
				{"3", "2", "2016-10-1", "3.0", "6/13/2019 3:00:00 AM"},
				{"4", "3", "2016-10-1", "4.0", "6/13/2019 3:00:00 AM"},
				{"5", "4", "2016-10-1", "4.0", "06/13/2019 3:00:00 PM"},
			},
			opts: &Options{
				PackageName:   "abc",
				MessageName:   "ABC",
				GoProtoImport: "foo/bar/test1_go_proto",
				GoPackageName: "test1",
			},
			want: &pb.RecordProtoMapping{
				PackageName: "abc",
				MessageName: "ABC",
				GoOptions: &pb.GoOptions{
					GoPackageName: "test1",
					ProtoImport:   "foo/bar/test1_go_proto",
				},
				ColumnToFieldMappings: []*pb.ColumnToFieldMapping{
					{
						ColName:      "AlphaBet",
						ColumnIndex:  0,
						Ignored:      false,
						ProtoType:    "int64",
						ProtoName:    "alpha_bet",
						ProtoImports: nil,
						ProtoTag:     1,
						Comment:      "Field type inferred from 5 unique values in 5 rows; 5 most common: \"1\" (1); \"2\" (1); \"3\" (1); \"4\" (1); \"5\" (1)",
					},
					{
						ColName:      "BBFriendOfMine_1",
						ColumnIndex:  1,
						Ignored:      false,
						ProtoType:    "string",
						ProtoName:    "bb_friend_of_mine_1",
						ProtoImports: nil,
						ProtoTag:     2,
						Comment:      "Field type inferred from 5 unique values in 5 rows; 5 most common: \"1\" (1); \"2\" (1); \"3\" (1); \"4\" (1); \"blah\" (1)",
					},
					{
						ColName:      "cee cee",
						ColumnIndex:  2,
						Ignored:      false,
						ProtoType:    "google.protobuf.Timestamp",
						ProtoName:    "cee_cee",
						ProtoImports: []string{"google/protobuf/timestamp.proto"},
						ProtoTag:     3,
						ParsingInfo: &pb.ColumnToFieldMapping_TimeFormat{
							TimeFormat: &pb.TimeFormat{
								GoLayout: "2006-1-2",
							},
						},
						Comment: "Field type inferred from 1 unique values in 5 rows; 1 most common: \"2016-10-1\" (5)",
					},
					{
						ColName:      "d__u_de",
						ColumnIndex:  3,
						Ignored:      false,
						ProtoType:    "float",
						ProtoName:    "d_u_de",
						ProtoImports: nil,
						ProtoTag:     4,
						Comment:      "Field type inferred from 4 unique values in 5 rows; 4 most common: \"4.0\" (2); \"1\" (1); \"2.0\" (1); \"3.0\" (1)",
					},
					{
						ColName:      "XTimeUTC",
						ColumnIndex:  4,
						Ignored:      false,
						ProtoType:    "google.protobuf.Timestamp",
						ProtoName:    "x_time_utc",
						ProtoImports: []string{"google/protobuf/timestamp.proto"},
						ProtoTag:     5,
						ParsingInfo: &pb.ColumnToFieldMapping_TimeFormat{
							TimeFormat: &pb.TimeFormat{
								GoLayout: "1/2/2006 3:04:05 PM",
							},
						},
						Comment: "Field type inferred from 2 unique values in 5 rows; 2 most common: \"6/13/2019 3:00:00 AM\" (4); \"06/13/2019 3:00:00 PM\" (1)",
					},
				},
			},
		},
		{
			name: "test 2",
			rows: [][]string{
				{"XTimeEastern"},
				{"6/13/2019 3:00:00 AM"},
				{"6/13/2019 3:00:00 AM"},
				{"6/13/2019 3:00:00 AM"},
				{"6/13/2019 3:00:00 AM"},
				{"06/13/2019 3:00:00 PM"},
			},
			opts: &Options{
				PackageName:       "abc",
				MessageName:       "ABC",
				TimestampLocation: montreal,
			},
			want: &pb.RecordProtoMapping{
				PackageName: "abc",
				MessageName: "ABC",
				ColumnToFieldMappings: []*pb.ColumnToFieldMapping{
					{
						ColName:      "XTimeEastern",
						ColumnIndex:  0,
						Ignored:      false,
						ProtoType:    "google.protobuf.Timestamp",
						ProtoName:    "x_time_eastern",
						ProtoImports: []string{"google/protobuf/timestamp.proto"},
						ProtoTag:     1,
						ParsingInfo: &pb.ColumnToFieldMapping_TimeFormat{
							TimeFormat: &pb.TimeFormat{
								GoLayout:     "1/2/2006 3:04:05 PM",
								TimeZoneName: "America/Montreal",
							},
						},
						Comment: "Field type inferred from 2 unique values in 5 rows; 2 most common: \"6/13/2019 3:00:00 AM\" (4); \"06/13/2019 3:00:00 PM\" (1)",
					},
				},
			},
		},
		{
			name: "invalid row length",
			rows: [][]string{
				{"XTimeEastern", "hello"},
				{"6/13/2019 3:00:00 AM"},
				{"6/13/2019 3:00:00 AM"},
				{"6/13/2019 3:00:00 AM"},
				{"6/13/2019 3:00:00 AM"},
				{"06/13/2019 3:00:00 PM"},
			},
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := NewRecordBasedInferrer(tc.opts)
			var err error
			for _, row := range tc.rows {
				err = b.AddRow(row)
				if err != nil {
					break
				}
			}

			if err != nil {
				if !tc.wantErr {
					t.Errorf("unexpected error adding row: %v", err)
				}
				return
			}

			gotIP, err := b.Build()
			if err != nil {
				if tc.wantErr {
					t.Errorf("unexpected builder error: %v", err)
				}
				return
			}

			if diff := cmp.Diff(tc.want, gotIP.Mapping(), protocmp.Transform()); diff != "" {
				t.Errorf("unexpected diff: %s", diff)
			}
		})
	}
}
