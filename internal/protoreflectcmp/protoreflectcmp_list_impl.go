package protoreflectcmp

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ezList struct {
	slice   []protoreflect.Value
	newElem func() protoreflect.Value
}

var _ protoreflect.List = &ezList{}

func newEZlist(prototype proto.Message) *ezList {
	return &ezList{
		nil,
		func() protoreflect.Value {
			return protoreflect.ValueOfMessage(proto.Clone(prototype).ProtoReflect())
		},
	}
}

func newListOfMessages(msg ...proto.Message) *ezList {
	if len(msg) == 0 {
		panic("cannot create empty list because the argument is used to determine the element type of the list")
	}
	l := newEZlist(proto.Clone(msg[0]))
	for _, x := range msg {
		l.Append(protoreflect.ValueOfMessage(x.ProtoReflect()))
	}
	return l
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
