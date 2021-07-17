package wirepath

import "google.golang.org/protobuf/reflect/protoreflect"

// Value is like protoreflect.Value, but it preserves the protobuf kind of the
// value.
type Value struct {
	kind protoreflect.Kind
	v    protoreflect.Value
}

func ValueOf(v interface{}, kind protoreflect.Kind) Value {
	panic("unsupported")
}

func ValueOfEnum(v protoreflect.EnumNumber) Value {
	return Value{protoreflect.EnumKind, protoreflect.ValueOfEnum(v)}
}
func ValueOfInt32(v int32) Value { return Value{protoreflect.Int32Kind, protoreflect.ValueOfInt32(v)} }
func ValueOfSint32(v int32) Value {
	return Value{protoreflect.Sint32Kind, protoreflect.ValueOfInt32(v)}
}
func ValueOfUint32(v uint32) Value {
	return Value{protoreflect.Uint32Kind, protoreflect.ValueOfUint32(v)}
}
func ValueOfInt64(v int64) Value { return Value{protoreflect.Int64Kind, protoreflect.ValueOfInt64(v)} }
func ValueOfSint64(v int64) Value {
	return Value{protoreflect.Sint64Kind, protoreflect.ValueOfInt64(v)}
}
func ValueOfUint64(v uint64) Value {
	return Value{protoreflect.Uint64Kind, protoreflect.ValueOfUint64(v)}
}
func ValueOfSfixed32(v int32) Value {
	return Value{protoreflect.Sfixed32Kind, protoreflect.ValueOfInt32(v)}
}
func ValueOfFixed32(v uint32) Value {
	return Value{protoreflect.Fixed32Kind, protoreflect.ValueOfUint32(v)}
}
func ValueOfFloat(v float32) Value {
	return Value{protoreflect.FloatKind, protoreflect.ValueOfFloat32(v)}
}
func ValueOfSfixed64(v int64) Value {
	return Value{protoreflect.Sfixed64Kind, protoreflect.ValueOfInt64(v)}
}
func ValueOfFixed64(v uint64) Value {
	return Value{protoreflect.Fixed64Kind, protoreflect.ValueOfUint64(v)}
}
func ValueOfDouble(v float64) Value {
	return Value{protoreflect.DoubleKind, protoreflect.ValueOfFloat64(v)}
}
func ValueOfString(v string) Value {
	return Value{protoreflect.StringKind, protoreflect.ValueOfString(v)}
}
func ValueOfBytes(v []byte) Value { return Value{protoreflect.BytesKind, protoreflect.ValueOfBytes(v)} }
func ValueOfMessage(v protoreflect.Message) Value {
	return Value{protoreflect.MessageKind, protoreflect.ValueOfMessage(v)}
}
func ValueOfList(v protoreflect.List, kind protoreflect.Kind) Value {
	return Value{kind, protoreflect.ValueOfList(v)}
}

// func ValueOf(v interface{}) Value
// func ValueOfBool(v bool) Value
// func ValueOfBytes(v []byte) Value
// func ValueOfEnum(v protoreflect.EnumNumber) Value
// func ValueOfFloat32(v float32) Value
// func ValueOfFloat64(v float64) Value
// func ValueOfInt32(v int32) Value
// func ValueOfInt64(v int64) Value
// func ValueOfList(v protoreflect.List) Value
// func ValueOfMap(v protoreflect.Map) Value
// func ValueOfMessage(v protoreflect.Message) Value
// func ValueOfString(v string) Value
// func ValueOfUint32(v uint32) Value
// func ValueOfUint64(v uint64) Value

func (v Value) Kind() protoreflect.Kind       { return v.kind }
func (v Value) Bool() bool                    { return v.v.Bool() }
func (v Value) Bytes() []byte                 { return v.v.Bytes() }
func (v Value) Enum() protoreflect.EnumNumber { return v.v.Enum() }
func (v Value) Float() float64                { return v.v.Float() }
func (v Value) Int() int64                    { return v.v.Int() }
func (v Value) Interface() interface{}        { return v.v.Interface() }
func (v Value) IsValid() bool                 { return v.v.IsValid() }
func (v Value) List() protoreflect.List       { return v.v.List() }
func (v Value) Map() protoreflect.Map         { return v.v.Map() }
func (v Value) MapKey() protoreflect.MapKey   { return v.v.MapKey() }
func (v Value) Message() protoreflect.Message { return v.v.Message() }
func (v Value) String() string                { return v.v.String() }
func (v Value) Uint() uint64                  { return v.v.Uint() }
