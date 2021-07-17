/* Package wirepath is an xpath-like means of representing a location within a protocol buffer message.

 */
package wirepath

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/xtoproto/proto/recordtoproto"
	"github.com/google/xtoproto/proto/wirepath"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestGetValue(t *testing.T) {
	tests := []struct {
		name    string
		path    *wirepath.WirePath
		within  proto.Message
		want    interface{}
		wantErr bool
	}{
		{
			name: "a",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 4},
			},
			within: &recordtoproto.ColumnToFieldMapping{ProtoType: "hello"},
			want:   "hello",
		},
		{
			name: "b",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 7},
				Slot: &wirepath.WirePath_RepeatedFieldOffset{
					RepeatedFieldOffset: 1,
				},
			},
			within: &recordtoproto.ColumnToFieldMapping{
				ProtoImports: []string{"zero", "one", "two"},
			},
			want: "one",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(tt.path)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}
			got, err := GetValue(parsed, tt.within)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got.Interface(), protocmp.Transform()); diff != "" {
				t.Errorf("GetValue() got unexpected diff (-want, +got):\n%s", diff)
			}
		})
	}
}
