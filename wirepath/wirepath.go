/* Package wirepath is an xpath-like means of representing a location within a protocol buffer message.

 */
package wirepath

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/google/xtoproto/proto/wirepath"
)

// WirePath is a parsed and validated Go representation of a WirePath protobuf.
type WirePath struct {
	proto *pb.WirePath
}

// String returns a wirepath expression that may be parsed into a *WirePath object.
func (p *WirePath) String() string {
	return debugString(p, protoreflect.Value{}, &pathFormatOptions{verboseErrors: true})
}

// Proto returns the protobuf representation of the WirePath object. The value
// should not be modified.
func (p *WirePath) Proto() *pb.WirePath {
	return p.proto
}

func (p *WirePath) child() *WirePath {
	c := p.proto.GetChild()
	if c == nil {
		return nil
	}
	return &WirePath{c}
}

func (p *WirePath) withoutChild() *WirePath {
	outProto := proto.Clone(p.proto).(*pb.WirePath)
	outProto.Child = nil
	return &WirePath{outProto}
}

// FromProto returns a parsed and validated version of the argument.
func FromProto(proto *pb.WirePath) (*WirePath, error) {
	return &WirePath{proto}, nil
}

// GetValue returns the value of some path within a protocol buffer message.
func GetValue(p *WirePath, within proto.Message) (protoreflect.Value, error) {
	return getValue(&getValueContext{nil, p, within})
}

type getValueContext struct {
	parent      *getValueContext
	path        *WirePath
	withinValue proto.Message
}

func (c *getValueContext) string() string {
	if c.parent == nil {
		return debugString(c.path, protoreflect.ValueOfMessage(c.withinValue.ProtoReflect()), &pathFormatOptions{verboseErrors: true})
	}
	return c.parent.string()
}

func (c *getValueContext) errorf(format string, args ...interface{}) error {
	allArgs := []interface{}{c.string()}
	allArgs = append(allArgs, args...)
	return fmt.Errorf("%s: "+format, allArgs...)
}

func getValue(ctx *getValueContext) (protoreflect.Value, error) {
	p := ctx.path
	within := ctx.withinValue
	if p == nil || p.proto == nil {
		return protoreflect.ValueOfMessage(within.ProtoReflect()), nil
	}

	var immediateValue protoreflect.Value
	if p.proto.GetSpecialPath() == pb.WirePath_SELF {
		immediateValue = protoreflect.ValueOfMessage(within.ProtoReflect())
	} else {
		m := within.ProtoReflect()

		fieldDescriptor := m.Descriptor().Fields().ByNumber(protoreflect.FieldNumber(p.proto.GetFieldNumber()))
		immediateValue = m.Get(fieldDescriptor)
	}
	if p.proto.GetSlot() != nil {
		slotIsMapKey := false
		var mapKey protoreflect.MapKey
		switch slot := p.proto.GetSlot().(type) {
		case *pb.WirePath_RepeatedFieldOffset:
			list, ok := immediateValue.Interface().(protoreflect.List)
			if !ok {
				return protoreflect.Value{}, ctx.errorf("path references element %d of a non-repeated field", slot)
			}
			if int(slot.RepeatedFieldOffset) >= list.Len() {
				return protoreflect.Value{}, ctx.errorf("%d is out of range (repeated field length = %d)", slot, list.Len())
			}
			return list.Get(int(slot.RepeatedFieldOffset)), nil

		case *pb.WirePath_MapKeyString:
			slotIsMapKey = true
			mapKey = protoreflect.MapKey(protoreflect.ValueOf(slot.MapKeyString))

		case *pb.WirePath_MapKeyInt32:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyInt32).MapKey()

		case *pb.WirePath_MapKeyInt64:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyInt64).MapKey()

		case *pb.WirePath_MapKeyUint32:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyUint32).MapKey()

		case *pb.WirePath_MapKeyUint64:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyUint64).MapKey()

		case *pb.WirePath_MapKeySint32:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeySint32).MapKey()

		case *pb.WirePath_MapKeySint64:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeySint64).MapKey()

		case *pb.WirePath_MapKeyFixed32:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyFixed32).MapKey()

		case *pb.WirePath_MapKeyFixed64:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyFixed64).MapKey()

		case *pb.WirePath_MapKeySfixed32:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeySfixed32).MapKey()

		case *pb.WirePath_MapKeySfixed64:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeySfixed64).MapKey()

		case *pb.WirePath_MapKeyBool:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyBool).MapKey()

		default:
			return protoreflect.Value{}, ctx.errorf("unsupported slot type: %v", p.proto.GetSlot())
		}

		isMap := func() bool {
			_, ok := immediateValue.Interface().(protoreflect.Map)
			return ok
		}

		if slotIsMapKey && !isMap() {
			return protoreflect.Value{}, ctx.errorf("tried to get value of map key %v of value that is not a Map", mapKey.Interface())
		} else if slotIsMapKey {
			immediateValue = immediateValue.Map().Get(mapKey)
		}
	}

	child := p.child()
	if child == nil {
		return immediateValue, nil
	}
	childWithinValue := immediateValue.Interface()
	if childWithinValue == nil {
		return protoreflect.Value{}, ctx.errorf("%s is nil, cannot evaluate remainder of expression %s", p.withoutChild().String(), child.String())
	}
	childMessage, ok := childWithinValue.(protoreflect.Message)
	if !ok {
		return protoreflect.Value{}, ctx.errorf("%q evaluted to %v, which is not a protobuf message; can't get child path %q", p.withoutChild(), childWithinValue, child)
	}

	newCtx := &getValueContext{
		parent:      ctx,
		path:        p.child(),
		withinValue: childMessage.Interface(),
	}
	return getValue(newCtx)
}

func debugString(path *WirePath, evaluatedAgainstValue protoreflect.Value, formatOpts *pathFormatOptions) string {
	if formatOpts == nil {
		formatOpts = &pathFormatOptions{}
	}
	var strs []string
	i := 0
	for ; path != nil && path.proto != nil; path = path.child() {
		elem, childValue := extractPrettyPathElem(path, evaluatedAgainstValue)
		strs = append(strs, elem.string(formatOpts))
		evaluatedAgainstValue = childValue
		i++
		if i > 10 {
			break
		}
	}
	return strings.Join(strs, "/")
}

type pathFormatOptions struct {
	verboseErrors bool
}

type pathElemPretty struct {
	self          bool
	fieldNumber   protoreflect.FieldNumber
	fieldName     protoreflect.Name
	formattedSlot string
	error         error
}

func (e *pathElemPretty) string(opts *pathFormatOptions) string {
	field := strconv.Itoa(int(e.fieldNumber))
	if e.self {
		field = "."
	}
	if e.fieldName != "" {
		field += fmt.Sprintf("(%s)", e.fieldName)
	}
	suffix := ""
	if e.formattedSlot != "" {
		suffix = fmt.Sprintf("[%s]", e.formattedSlot)
	}
	final := field + suffix
	if e.error != nil {
		msg := "!ERROR"
		if opts.verboseErrors {
			msg += fmt.Sprintf(":%q", e.error.Error())
		}
		return final + msg
	}
	return final
}

func extractPrettyPathElem(path *WirePath, value protoreflect.Value) (*pathElemPretty, protoreflect.Value) {
	out := &pathElemPretty{}
	var immediateValue protoreflect.Value
	var fieldDescriptor protoreflect.FieldDescriptor
	if path == nil || path.proto == nil || path.proto.GetSpecialPath() == pb.WirePath_SELF {
		out.self = true
		immediateValue = value
	} else {
		out.fieldNumber = protoreflect.FieldNumber(path.proto.GetFieldNumber())
		out.fieldName = protoreflect.Name(path.proto.GetFieldName())
		m, isMessage := value.Interface().(protoreflect.Message)
		if isMessage {
			fieldDescriptor = m.Descriptor().Fields().ByNumber(protoreflect.FieldNumber(path.proto.GetFieldNumber()))
			if fieldDescriptor != nil {
				if out.fieldName == "" {
					out.fieldName = fieldDescriptor.Name()
				}
				immediateValue = m.Get(fieldDescriptor)
				if !immediateValue.IsValid() {
					immediateValue = m.NewField(fieldDescriptor)
				}
			}
		} else if value.IsValid() {
			out.error = fmt.Errorf("field cannot exist on non-message type")
		}
	}
	if path == nil || path.proto == nil {
		return out, protoreflect.Value{}
	}

	var childValue protoreflect.Value

	formattedSlotForMapKey := func(literal string, kind protoreflect.Kind) string {
		return fmt.Sprintf("%s:%s", literal, kind.String())
	}

	if path.proto.GetSlot() == nil {
		childValue = immediateValue
	} else {
		mapKeyKind := extractSlotMapKeyKind(path.proto)
		var mapKey protoreflect.MapKey
		switch slot := path.proto.GetSlot().(type) {
		case *pb.WirePath_RepeatedFieldOffset:
			out.formattedSlot = strconv.Itoa(int(slot.RepeatedFieldOffset))

			list, isList := immediateValue.Interface().(protoreflect.List)
			if isList {
				if int(slot.RepeatedFieldOffset) < list.Len() {
					childValue = list.Get(int(slot.RepeatedFieldOffset))
				} else {
					childValue = list.NewElement()
				}
			}

		case *pb.WirePath_MapKeyString:
			out.formattedSlot = strconv.Quote(slot.MapKeyString)
			mapKey = protoreflect.ValueOf(slot.MapKeyString).MapKey()

		case *pb.WirePath_MapKeyInt32:
			out.formattedSlot = formattedSlotForMapKey(fmt.Sprintf("%v", slot.MapKeyInt32), protoreflect.Int32Kind)
			mapKey = protoreflect.ValueOf(slot.MapKeyInt32).MapKey()

		case *pb.WirePath_MapKeyInt64:
			out.formattedSlot = formattedSlotForMapKey(fmt.Sprintf("%v", slot.MapKeyInt64), protoreflect.Int64Kind)
			mapKey = protoreflect.ValueOf(slot.MapKeyInt64).MapKey()

		case *pb.WirePath_MapKeyUint32:
			out.formattedSlot = formattedSlotForMapKey(fmt.Sprintf("%v", slot.MapKeyUint32), protoreflect.Uint32Kind)
			mapKey = protoreflect.ValueOf(slot.MapKeyUint32).MapKey()

		case *pb.WirePath_MapKeyUint64:
			out.formattedSlot = formattedSlotForMapKey(fmt.Sprintf("%v", slot.MapKeyUint64), protoreflect.Uint64Kind)
			mapKey = protoreflect.ValueOf(slot.MapKeyUint64).MapKey()

		case *pb.WirePath_MapKeySint32:
			out.formattedSlot = formattedSlotForMapKey(fmt.Sprintf("%v", slot.MapKeySint32), protoreflect.Sint32Kind)
			mapKey = protoreflect.ValueOf(slot.MapKeySint32).MapKey()

		case *pb.WirePath_MapKeySint64:
			out.formattedSlot = formattedSlotForMapKey(fmt.Sprintf("%v", slot.MapKeySint64), protoreflect.Sint64Kind)
			mapKey = protoreflect.ValueOf(slot.MapKeySint64).MapKey()

		case *pb.WirePath_MapKeyFixed32:
			out.formattedSlot = formattedSlotForMapKey(fmt.Sprintf("%v", slot.MapKeyFixed32), protoreflect.Fixed32Kind)
			mapKey = protoreflect.ValueOf(slot.MapKeyFixed32).MapKey()

		case *pb.WirePath_MapKeyFixed64:
			out.formattedSlot = formattedSlotForMapKey(fmt.Sprintf("%v", slot.MapKeyFixed64), protoreflect.Fixed64Kind)
			mapKey = protoreflect.ValueOf(slot.MapKeyFixed64).MapKey()

		case *pb.WirePath_MapKeySfixed32:
			out.formattedSlot = formattedSlotForMapKey(fmt.Sprintf("%v", slot.MapKeySfixed32), protoreflect.Sfixed32Kind)
			mapKey = protoreflect.ValueOf(slot.MapKeySfixed32).MapKey()

		case *pb.WirePath_MapKeySfixed64:
			out.formattedSlot = formattedSlotForMapKey(fmt.Sprintf("%v", slot.MapKeySfixed64), protoreflect.Sfixed64Kind)
			mapKey = protoreflect.ValueOf(slot.MapKeySfixed64).MapKey()

		case *pb.WirePath_MapKeyBool:
			out.formattedSlot = formattedSlotForMapKey(fmt.Sprintf("%v", slot.MapKeyBool), protoreflect.BoolKind)
			mapKey = protoreflect.ValueOf(slot.MapKeyBool).MapKey()

		default:
			out.formattedSlot = fmt.Sprintf("ERROR: invalid slot type: %v", path.proto.GetSlot())
		}

		if mapKeyKind != 0 && immediateValue.IsValid() {
			m, isMap := immediateValue.Interface().(protoreflect.Map)

			if !isMap {
				out.error = fmt.Errorf("specified map key for non-map value")
			} else {
				if fieldDescriptor == nil {
					panic("internal assumption is incorrect - field descriptor should be available")
				}
				if got, want := fieldDescriptor.MapKey().Kind(), mapKeyKind; got != want {
					out.error = fmt.Errorf("wirepath specifies map key %v of kind %s, %s key type is %s", mapKey.Interface(), want, fieldDescriptor.FullName(), got)
				} else {
					childValue = m.Get(mapKey)
					if !childValue.IsValid() {
						childValue = m.NewValue()
					}
				}
			}
		}
	}
	return out, childValue
}

func extractSlotMapKeyKind(proto *pb.WirePath) protoreflect.Kind {
	var defaultKind protoreflect.Kind
	switch proto.GetSlot().(type) {

	case *pb.WirePath_MapKeyString:
		return protoreflect.StringKind

	case *pb.WirePath_MapKeyInt32:
		return protoreflect.Int32Kind

	case *pb.WirePath_MapKeyInt64:
		return protoreflect.Int64Kind

	case *pb.WirePath_MapKeyUint32:
		return protoreflect.Uint32Kind

	case *pb.WirePath_MapKeyUint64:
		return protoreflect.Uint64Kind

	case *pb.WirePath_MapKeySint32:
		return protoreflect.Sint32Kind

	case *pb.WirePath_MapKeySint64:
		return protoreflect.Sint64Kind

	case *pb.WirePath_MapKeyFixed32:
		return protoreflect.Fixed32Kind

	case *pb.WirePath_MapKeyFixed64:
		return protoreflect.Fixed64Kind

	case *pb.WirePath_MapKeySfixed32:
		return protoreflect.Sfixed32Kind

	case *pb.WirePath_MapKeySfixed64:
		return protoreflect.Sfixed64Kind

	case *pb.WirePath_MapKeyBool:
		return protoreflect.BoolKind

	default:
		return defaultKind
	}
}
