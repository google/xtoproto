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

package converter_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/xtoproto/examples/example01/converter"
	"google.golang.org/protobuf/testing/protocmp"

	pb "github.com/google/xtoproto/examples/example01"
)

func TestReader(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		csv                     string
		wantNewErr, wantReadErr *regexp.Regexp
		want                    interface{}
	}{
		{
			"single line",
			"name,age,height\nfred,40,3m\n",
			nil,
			nil,
			[]*pb.MyMessage{
				{
					Name:   "fred",
					Age:    40,
					Height: "3m",
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {

			r, err := converter.NewReader(strings.NewReader(tt.csv))
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
