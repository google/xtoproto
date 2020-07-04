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

package converter02_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/google/go-cmp/cmp"
	"github.com/google/xtoproto/csvtoprotoparse"
	"github.com/google/xtoproto/examples/example02/converter02"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/google/xtoproto/examples/example02"
)

var pacificTZ = csvtoprotoparse.MustLoadLocation("America/Los_Angeles")

func TestReader(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		csv                     string
		wantNewErr, wantReadErr *regexp.Regexp
		want                    interface{}
	}{
		{
			"single line",
			`project_name,lines_of_code,url,last_modified
"xtoproto",3000,"https://github.com/google/xtoproto",2020-10-04
"bazel",500000,"https://bazel.build",2020-2-26
`,
			nil,
			nil,
			[]*pb.Example2{
				{
					ProjectName:  "xtoproto",
					LinesOfCode:  3000,
					Url:          "https://github.com/google/xtoproto",
					LastModified: mustTimestamp(time.Date(2020, 10, 4, 0, 0, 0, 0, pacificTZ)),
				},
				{
					ProjectName:  "bazel",
					LinesOfCode:  500000,
					Url:          "https://bazel.build",
					LastModified: mustTimestamp(time.Date(2020, 2, 26, 0, 0, 0, 0, pacificTZ)),
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {

			r, err := converter02.NewReader(strings.NewReader(tt.csv))
			if err != nil {
				t.Fatalf("NewReader error: %v", err)
			}
			recs, err := r.ReadAll()
			if err != nil {
				t.Fatalf("Read() error: %v", err)
			}
			got, want := recs, tt.want
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected diff (-want, +got):\n%s", diff)
			}
		})
	}
}

func mustTimestamp(t time.Time) *timestamppb.Timestamp {
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		panic(err)
	}
	return ts
}
