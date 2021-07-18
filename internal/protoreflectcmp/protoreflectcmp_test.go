package protoreflectcmp

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/xtoproto/proto/wirepath/testproto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	exampleProtoDescriptor = (&testproto.Example{}).ProtoReflect().Descriptor()
)

func TestSliceComparison(t *testing.T) {
	tests := []struct {
		name        string
		left, right interface{}
		wantEqual   bool
	}{
		{
			name: "list and slice - no diff",
			left: newListOfMessages(
				&testproto.Example{ColName: "one"},
				&testproto.Example{ColName: "two"},
			),
			right: []*testproto.Example{
				{ColName: "one"},
				{ColName: "two"},
			},
			wantEqual: true,
		},
		{
			name: "list and slice - diff exists",
			left: newListOfMessages(
				&testproto.Example{ColName: "one"},
			),
			right: []*testproto.Example{
				{ColName: "one"},
				{ColName: "not peresent in left"},
			},
			wantEqual: false,
		},
		{
			name: "list and slice - generic []proto.Message comparison",
			left: newListOfMessages(
				&testproto.Example{ColName: "one"},
				&testproto.Example{ColName: "two"},
			),
			right: []proto.Message{
				&testproto.Example{ColName: "one"},
				&testproto.Example{ColName: "two"},
			},
			wantEqual: true,
		},
		{
			name: "list and list - no diff",
			left: newListOfMessages(
				&testproto.Example{ColName: "one"},
			),
			right: (&testproto.Example{
				Children: []*testproto.Example{
					{ColName: "one"},
				},
			}).ProtoReflect().Get(exampleProtoDescriptor.Fields().ByName("children")).List(),
			wantEqual: true,
		},
		{
			name: "list and list - diff exists",
			left: newListOfMessages(
				&testproto.Example{ColName: "one"},
			),
			right: (&testproto.Example{
				Children: []*testproto.Example{
					{ColName: "one"},
					{ColName: "not peresent in left"},
				},
			}).ProtoReflect().Get(exampleProtoDescriptor.Fields().ByName("children")).List(),
			wantEqual: false,
		},
		{
			name: "slice and slice - will not activate the filter",
			left: []*testproto.Example{
				{ColName: "one"},
				{ColName: "two"},
			},
			right: []*testproto.Example{
				{ColName: "one"},
				{ColName: "two"},
			},
			wantEqual: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			equal := cmp.Equal(tt.left, tt.right, protocmp.Transform(), Transform())
			diff := cmp.Diff(tt.left, tt.right, protocmp.Transform(), Transform())

			if equal != tt.wantEqual {
				t.Errorf("got cmp.Equal(...) = %v, want %v", equal, tt.wantEqual)
			}
			if diffEmpty, want := diff == "", tt.wantEqual; diffEmpty != want {
				t.Errorf("Unexpected diff (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestSliceAnalyze(t *testing.T) {
	type summary struct {
		NormalizedSliceIsValid bool
		Name                   protoreflect.FullName
	}
	tests := []struct {
		name string
		arg  interface{}
		want summary
	}{
		{
			name: "descriptor from slice type",
			arg: []*testproto.Example{
				{ColName: "one"},
				{ColName: "two"},
			},
			want: summary{
				Name:                   "xtoproto.wirepath.internal.Example",
				NormalizedSliceIsValid: true,
			},
		},
		{
			name: "descriptor from generic slice type is not currently supported",
			arg: []proto.Message{
				&testproto.Example{ColName: "one"},
				&testproto.Example{ColName: "two"},
			},
			want: summary{
				Name:                   "<proto.Message slice>",
				NormalizedSliceIsValid: true,
			},
		},
		{
			name: "descriptor from slice type",
			arg: newListOfMessages(
				&testproto.Example{ColName: "one"},
				&testproto.Example{ColName: "two"},
			),
			want: summary{
				Name:                   "xtoproto.wirepath.internal.Example",
				NormalizedSliceIsValid: true,
			},
		},
		{
			name: "descriptor from slice type",
			arg:  newEZlist(&testproto.Example{}),
			want: summary{
				Name:                   "xtoproto.wirepath.internal.Example",
				NormalizedSliceIsValid: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := analyzeValue(tt.arg)
			got := summary{
				Name:                   analysis.SliceMessageName,
				NormalizedSliceIsValid: analysis.NormalizedSlice != nil,
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Unexpected analysis diff (-want, +got):\n%s", diff)
				t.Logf("full analysis: %+v", analysis)
			}
		})
	}
}
