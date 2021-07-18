/* Package wirepath is an xpath-like means of representing a location within a protocol buffer message.

 */
package wirepath

import (
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/xtoproto/internal/protoreflectcmp"
	"github.com/google/xtoproto/proto/wirepath"
	"github.com/google/xtoproto/proto/wirepath/testproto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
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
			within: &testproto.Example{ProtoType: "hello"},
			want:   "hello",
		},
		{
			name: "b",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 7},
				Slot:    &wirepath.WirePath_RepeatedFieldOffset{RepeatedFieldOffset: 1},
			},
			within: &testproto.Example{
				ProtoImports: []string{"zero", "one", "two"},
			},
			want: "one",
		},
		{
			name: "c",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 9},
				Slot:    &wirepath.WirePath_MapKeyInt32{MapKeyInt32: 5},
			},
			within: &testproto.Example{
				ModbusValues: map[int32]string{5: "five"},
			},
			want: "five",
		},
		{
			name: "d",
			path: MustParse(`9[5:int32]`).Proto(),
			within: &testproto.Example{
				ModbusValues: map[int32]string{5: "five"},
			},
			want: "five",
		},
		{
			name: "field cannot exist on non message type",
			path: MustParse(`9[5:int32]/5`).Proto(),
			within: &testproto.Example{
				ModbusValues: map[int32]string{5: "five"},
			},
			wantErr: true,
		},
		{
			name:    "tried to get child of a map value for an entry that does not exist",
			path:    MustParse(`11["child"]/4(proto_type)`).Proto(),
			within:  &testproto.Example{},
			wantErr: true,
		},
		{
			name: "map value's field",
			path: MustParse(`12[1:bool]/20000(ignored)`).Proto(),
			within: &testproto.Example{
				NamedFriends: map[bool]*testproto.Friend{
					true: {Name: "Jaya"},
				},
			},
			want: "Jaya",
		},
		{
			name: "map value that does not exist as final path",
			path: MustParse(`12[0:bool]`).Proto(),
			within: &testproto.Example{
				NamedFriends: map[bool]*testproto.Friend{
					true: {Name: "Jaya"},
				},
			},
			want: nil,
		},
		{
			name: "child of repeated field",
			path: MustParse(`8(proto_imports)[1]/5(proto_tag)`).Proto(),
			within: &testproto.Example{
				Children: []*testproto.Example{
					{},
					{ProtoTag: 42},
				},
			},
			want: int32(42),
		},
		{
			name: "child of repeated field out of bounds",
			path: MustParse(`8(proto_imports)[2]/5(proto_tag)`).Proto(),
			within: &testproto.Example{
				Children: []*testproto.Example{
					{},
					{ProtoTag: 42},
				},
			},
			wantErr: true, // out of bounds
		},
		{
			name: "child of repeated field ",
			path: MustParse(`8(proto_imports)/5(proto_tag)`).Proto(),
			within: &testproto.Example{
				Children: []*testproto.Example{
					{},
					{ProtoTag: 42},
				},
			},
			wantErr: true, // cannot specify a child path elemnt of a list without a slot selector.
		},
		{
			name: "repeated field should return a list",
			path: MustParse(`8(proto_imports)`).Proto(),
			within: &testproto.Example{
				Children: []*testproto.Example{
					{},
					{ProtoTag: 42},
				},
			},
			want: []*testproto.Example{
				{},
				{ProtoTag: 42},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := FromProto(tt.path)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}
			gotValue, err := GetValue(parsed, tt.within)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			got := gotValue.Interface()
			w, g := &tt.want, &got
			if diff := cmp.Diff(w, g, protocmp.Transform(), protoreflectcmp.Transform()); diff != "" {
				t.Errorf("GetValue() got unexpected diff (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestDebugString(t *testing.T) {
	tests := []struct {
		name         string
		path         *wirepath.WirePath
		againstValue protoreflect.Value
		want         string
	}{
		{
			name: "a",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 4},
			},
			want: "4",
		},
		{
			name: "b",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 7},
				Slot:    &wirepath.WirePath_RepeatedFieldOffset{RepeatedFieldOffset: 1},
			},
			want: "7[1]",
		},
		{
			name: "c",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 9},
				Slot:    &wirepath.WirePath_MapKeyInt32{MapKeyInt32: 5},
			},
			want: "9[5:int32]",
		},
		{
			name: "d",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 9},
				Slot:    &wirepath.WirePath_MapKeySint32{MapKeySint32: 5},
				Child: &wirepath.WirePath{
					Element: &wirepath.WirePath_FieldNumber{FieldNumber: 1},
				},
			},
			want: "9[5:sint32]/1",
		},
		{
			name: "e",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 9},
				Slot:    &wirepath.WirePath_MapKeyInt32{MapKeyInt32: 5},
				Child: &wirepath.WirePath{
					Element: &wirepath.WirePath_FieldNumber{FieldNumber: 1},
				},
			},
			againstValue: protoreflect.ValueOfMessage((&testproto.Example{}).ProtoReflect()),
			want:         "9(modbus_values)[5:int32]/1!ERROR",
		},
		{
			name: "map value that is not present",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 11},
				Slot:    &wirepath.WirePath_MapKeyString{MapKeyString: "abc"},
				Child: &wirepath.WirePath{
					Element: &wirepath.WirePath_FieldNumber{FieldNumber: 1},
				},
			},
			againstValue: protoreflect.ValueOfMessage((&testproto.Example{
				NamedChildren: map[string]*testproto.Example{},
			}).ProtoReflect()),
			want: `11(named_children)["abc"]/1(column_index)`,
		},
		{
			name: "map value that is is present",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 11},
				Slot:    &wirepath.WirePath_MapKeyString{MapKeyString: "abc"},
				Child: &wirepath.WirePath{
					Element: &wirepath.WirePath_FieldNumber{FieldNumber: 1},
				},
			},
			againstValue: protoreflect.ValueOfMessage((&testproto.Example{
				NamedChildren: map[string]*testproto.Example{"abc": {}},
			}).ProtoReflect()),
			want: `11(named_children)["abc"]/1(column_index)`,
		},
		{
			name: "map key of incorrect type",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 11},
				Slot:    &wirepath.WirePath_MapKeyUint32{MapKeyUint32: 5013},
				Child: &wirepath.WirePath{
					Element: &wirepath.WirePath_FieldNumber{FieldNumber: 1},
				},
			},
			againstValue: protoreflect.ValueOfMessage((&testproto.Example{
				NamedChildren: map[string]*testproto.Example{"abc": {}},
			}).ProtoReflect()),
			want: `11(named_children)[5013:uint32]!ERROR/1`,
		},
		{
			name: "child",
			path: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 10},
				Child: &wirepath.WirePath{
					Element: &wirepath.WirePath_FieldNumber{FieldNumber: 4},
				},
			},
			againstValue: protoreflect.ValueOfMessage((&testproto.Example{}).ProtoReflect()),
			want:         "10(child)/4(proto_type)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := FromProto(tt.path)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}
			got := debugString(parsed, tt.againstValue, nil)
			gotVerbose := debugString(parsed, tt.againstValue, &pathFormatOptions{verboseErrors: true})
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("debugString() got unexpected diff (-want, +got):\n%s\nverbose value: %s", diff, gotVerbose)
			}
		})
	}
}

func TestParseString(t *testing.T) {
	tests := []struct {
		literal string
		want    *wirepath.WirePath
		wantErr bool
	}{
		{
			literal: "10/4",
			want: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 10},
				Child: &wirepath.WirePath{
					Element: &wirepath.WirePath_FieldNumber{FieldNumber: 4},
				},
			},
			wantErr: false,
		},
		{
			literal: "10(foo)/4",
			want: &wirepath.WirePath{
				Element:   &wirepath.WirePath_FieldNumber{FieldNumber: 10},
				FieldName: "foo",
				Child: &wirepath.WirePath{
					Element: &wirepath.WirePath_FieldNumber{FieldNumber: 4},
				},
			},
			wantErr: false,
		},
		{
			literal: "10(foo)[3]/4",
			want: &wirepath.WirePath{
				Element:   &wirepath.WirePath_FieldNumber{FieldNumber: 10},
				FieldName: "foo",
				Slot:      &wirepath.WirePath_RepeatedFieldOffset{RepeatedFieldOffset: 3},
				Child: &wirepath.WirePath{
					Element: &wirepath.WirePath_FieldNumber{FieldNumber: 4},
				},
			},
			wantErr: false,
		},
		{
			literal: "10[3:int32]/4",
			want: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 10},
				Slot:    &wirepath.WirePath_MapKeyInt32{MapKeyInt32: 3},
				Child: &wirepath.WirePath{
					Element: &wirepath.WirePath_FieldNumber{FieldNumber: 4},
				},
			},
			wantErr: false,
		},
		{
			literal: "10[3:sint32]",
			want: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 10},
				Slot:    &wirepath.WirePath_MapKeySint32{MapKeySint32: 3},
			},
			wantErr: false,
		},
		{
			literal: "10[3:uint32]",
			want: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 10},
				Slot:    &wirepath.WirePath_MapKeyUint32{MapKeyUint32: 3},
			},
			wantErr: false,
		},
		{
			literal: "10[-3:uint32]",
			wantErr: true,
		},
		{
			literal: `10["abc"]/4`,
			want: &wirepath.WirePath{
				Element: &wirepath.WirePath_FieldNumber{FieldNumber: 10},
				Slot:    &wirepath.WirePath_MapKeyString{MapKeyString: "abc"},
				Child: &wirepath.WirePath{
					Element: &wirepath.WirePath_FieldNumber{FieldNumber: 4},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.literal, func(t *testing.T) {
			got, err := ParseString(tt.literal)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("got error %v, wantErr = %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got.Proto(), protocmp.Transform()); diff != "" {
				t.Errorf("ParseString() got unexpected diff (-want, +got):\n  %s", diff)
			}
		})
	}
}

func TestRegexpAssumptions(t *testing.T) {
	tests := []struct {
		re    *regexp.Regexp
		input string
		want  []string
	}{
		{
			regexp.MustCompile(stringLiteral),
			`"hi"`,
			[]string{`"hi"`},
		},
		{
			regexp.MustCompile(slotGroups),
			`[1]`,
			[]string{`[1]`, `1`, ``, ``, ``},
		},
		{
			regexp.MustCompile(slotGroups),
			`["abc"]`,
			[]string{`["abc"]`, ``, `"abc"`, ``, ``},
		},
		{
			regexp.MustCompile(slotGroups),
			`[123:uint32]`,
			[]string{`[123:uint32]`, ``, ``, `123`, `uint32`},
		},
		{
			protobufStringElemRegexp,
			`a`,
			[]string{`a`, ``, ``, ``, `a`},
		},
		{
			protobufStringElemRegexp,
			`\n`,
			[]string{`\n`, ``, ``, `n`, ``},
		},
		{
			protobufStringElemRegexp,
			`\x1f`,
			[]string{`\x1f`, `1f`, ``, ``, ``},
		},
		{
			protobufStringElemRegexp,
			`\0134`,
			[]string{`\0134`, ``, `134`, ``, ``},
		},
	}

	for _, tt := range tests {
		t.Run(tt.re.String(), func(t *testing.T) {
			got := tt.re.FindStringSubmatch(tt.input)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("%s got unexpected diff (-want, +got):\n  %s", tt.re.String(), diff)
			}
		})
	}
}

func TestParseProtobufStringLiteral(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: `"abc"`, want: "abc"},
		{input: `"\x16\xFf"`, want: "\x16\xFF"},
		{input: `"\xc9\0311"`, want: "\xc9\xc9"},
		{input: `"\n"`, want: "\n"},
		{input: `"\a"`, want: "\a"},
		{input: `"\b"`, want: "\b"},
		{input: `"\f"`, want: "\f"},
		{input: `"\r"`, want: "\r"},
		{input: `"\t"`, want: "\t"},
		{input: `"\v"`, want: "\v"},
		{input: `"\""`, want: `"`},
		{input: `"\'"`, want: "'"},
		{input: `"\\"`, want: `\`},
		{input: `"my quote\""`, want: `my quote"`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseProtobufStringLiteral(tt.input)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("unexpected error = %v, wantErr = %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("unexpected diff (-want, +got):\nwant: %v\ngot:  %v\ndiff:\n%s", []byte(tt.want), []byte(got), diff)
			}
		})
	}
}
