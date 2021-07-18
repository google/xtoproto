/* Package wirepath is an xpath-like means of representing a location within a protocol buffer message.

 */
package wirepath

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
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
			name: "child of repeated field",
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
			name: "child of repeated field",
			path: MustParse(`8(proto_imports)/5(proto_tag)`).Proto(),
			within: &testproto.Example{
				Children: []*testproto.Example{
					{},
					{ProtoTag: 42},
				},
			},
			wantErr: true, // cannot specify a field (proto_tag) of a list.
		},
		{
			name: "child of repeated field",
			path: MustParse(`8(proto_imports)`).Proto(),
			within: &testproto.Example{
				Children: []*testproto.Example{
					{},
					{ProtoTag: 42},
				},
			},
			want: func() protoreflect.List {
				l := newEZlist(&testproto.Example{})
				return l
			}(),
			// want: []*testproto.Example{
			// 	{},
			// 	{ProtoTag: 42},
			// },
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
			if diff := cmp.Diff(w, g, protocmp.Transform(), cmpProtoReflectOpt); diff != "" {
				//if diff := cmp.Diff(tt.want, got.Interface(), protocmp.Transform(), reflectListTransform, transform2); diff != "" {
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

var (
	cmpProtoReflectOpt = cmp.FilterValues(func(a, b interface{}) bool {
		_, ok1 := a.(protoreflect.List)
		_, ok2 := a.(protoreflect.List)
		panic("x")
		return ok1 && ok2
	}, reflectListTransform)

	reflectListTransform = cmp.Transformer("listToSlice", func(list protoreflect.List) interface{} {
		if !list.IsValid() {
			return nil
		}
		elemType := reflect.TypeOf(list.NewElement().Interface())
		out := reflect.MakeSlice(elemType, list.Len(), list.Len())
		for i := 0; i < list.Len(); i++ {
			out.Index(i).Set(reflect.ValueOf(list.Get(i).Interface()))
		}
		return out.Interface()
	})
)

var transform3 = cmp.Transformer("listToSlice", func(list protoreflect.List) []protoreflect.Value {
	out := make([]protoreflect.Value, list.Len())
	for i := 0; i < list.Len(); i++ {
		out[i] = list.Get(i)
	}
	return out
})

var transform2 = cmp.Comparer(func(a, b protoreflect.List) bool {
	//out := make([]protoreflect.Value, list.Len())
	panic(fmt.Sprintf("comparing %v and %v", a, b))
	// if a.Len() != b.Len() {
	// 	return false
	// }
	// for i := 0; i < list.Len(); i++ {
	// 	out[i] = list.Get(i)
	// }
	// return out
})

type ezList struct {
	slice   []protoreflect.Value
	newElem func() protoreflect.Value
}

func newEZlist(prototype proto.Message) *ezList {
	return &ezList{
		nil,
		func() protoreflect.Value {
			return protoreflect.ValueOf(proto.Clone(prototype))
		},
	}
}

// Len reports the number of entries in the List.
// Get, Set, and Truncate panic with out of bound indexes.
func (l *ezList) Len() int {
	return len(l.slice)
}

// Get retrieves the value at the given index.
// It never returns an invalid value.
func (l *ezList) Get(i int) protoreflect.Value {
	return l.slice[i]
}

// Set stores a value for the given index.
// When setting a composite type, it is unspecified whether the set
// value aliases the source's memory in any way.
//
// Set is a mutating operation and unsafe for concurrent use.
func (l *ezList) Set(i int, v protoreflect.Value) {
	l.slice[i] = v
}

// Append appends the provided value to the end of the list.
// When appending a composite type, it is unspecified whether the appended
// value aliases the source's memory in any way.
//
// Append is a mutating operation and unsafe for concurrent use.
func (l *ezList) Append(v protoreflect.Value) {
	l.slice = append(l.slice, v)
}

// AppendMutable appends a new, empty, mutable message value to the end
// of the list and returns it.
// It panics if the list does not contain a message type.
func (l *ezList) AppendMutable() protoreflect.Value {
	e := l.newElem()
	l.Append(e)
	return e
}

// Truncate truncates the list to a smaller length.
//
// Truncate is a mutating operation and unsafe for concurrent use.
func (l *ezList) Truncate(newLen int) {
	l.slice = l.slice[0:newLen]
}

// NewElement returns a new value for a list element.
// For enums, this returns the first enum value.
// For other scalars, this returns the zero value.
// For messages, this returns a new, empty, mutable value.
func (l *ezList) NewElement() protoreflect.Value {
	return l.newElem()
}

// IsValid reports whether the list is valid.
//
// An invalid list is an empty, read-only value.
//
// Validity is not part of the protobuf data model, and may not
// be preserved in marshaling or other operations.
func (l *ezList) IsValid() bool {
	return true
}
