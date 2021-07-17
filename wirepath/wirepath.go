/* Package wirepath is an xpath-like means of representing a location within a protocol buffer message.

 */
package wirepath

import (
	"fmt"

	"github.com/google/xtoproto/proto/wirepath"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// WirePath is a parsed and validated Go representation of a WirePath protobuf.
type WirePath struct {
	proto *wirepath.WirePath
}

func (p *WirePath) child() *WirePath {
	c := p.proto.GetChild()
	if c == nil {
		return nil
	}
	return &WirePath{c}
}

// Parse returns a parsed and validated version of the argument.
func Parse(proto *wirepath.WirePath) (*WirePath, error) {
	return &WirePath{proto}, nil
}

// GetValue returns the value of some path within a protocol buffer message.
func GetValue(p *WirePath, within proto.Message) (protoreflect.Value, error) {
	if p == nil || p.proto == nil {
		return protoreflect.ValueOfMessage(within.ProtoReflect()), nil
	}

	var immediateValue protoreflect.Value
	if p.proto.GetSpecialPath() == wirepath.WirePath_SELF {
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
		case *wirepath.WirePath_RepeatedFieldOffset:
			list, ok := immediateValue.Interface().(protoreflect.List)
			if !ok {
				return protoreflect.Value{}, fmt.Errorf("path references element %d of a non-repeated field", slot)
			}
			if int(slot.RepeatedFieldOffset) >= list.Len() {
				return protoreflect.Value{}, fmt.Errorf("%d is out of range", slot)
			}
			return list.Get(int(slot.RepeatedFieldOffset)), nil

		case *wirepath.WirePath_MapKeyString:
			slotIsMapKey = true
			mapKey = protoreflect.MapKey(protoreflect.ValueOf(slot.MapKeyString))

		case *wirepath.WirePath_MapKeyInt32:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyInt32).MapKey()

		case *wirepath.WirePath_MapKeyInt64:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyInt64).MapKey()

		case *wirepath.WirePath_MapKeyBytes:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyBytes).MapKey()

		case *wirepath.WirePath_MapKeyDouble:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyDouble).MapKey()

		case *wirepath.WirePath_MapKeyFloat:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyFloat).MapKey()

		case *wirepath.WirePath_MapKeyUint32:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyUint32).MapKey()

		case *wirepath.WirePath_MapKeyUint64:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyUint64).MapKey()

		case *wirepath.WirePath_MapKeySint32:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeySint32).MapKey()

		case *wirepath.WirePath_MapKeySint64:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeySint64).MapKey()

		case *wirepath.WirePath_MapKeyFixed32:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyFixed32).MapKey()

		case *wirepath.WirePath_MapKeyFixed64:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyFixed64).MapKey()

		case *wirepath.WirePath_MapKeySfixed32:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeySfixed32).MapKey()

		case *wirepath.WirePath_MapKeySfixed64:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeySfixed64).MapKey()

		case *wirepath.WirePath_MapKeyBool:
			slotIsMapKey = true
			mapKey = protoreflect.ValueOf(slot.MapKeyBool).MapKey()

		default:
			return protoreflect.Value{}, fmt.Errorf("unsupported slot type: %v", p.proto.GetSlot())
		}

		isMap := func() bool {
			_, ok := immediateValue.Interface().(protoreflect.Map)
			return ok
		}

		if slotIsMapKey && !isMap() {
			return protoreflect.Value{}, fmt.Errorf("tried to get value of map key %v of value that is not a Map", mapKey.Interface())
		} else if slotIsMapKey {
			immediateValue = immediateValue.Map().Get(mapKey)
		}
	}

	child := p.child()
	if child == nil {
		return immediateValue, nil
	}
	childMessage, ok := immediateValue.Interface().(protoreflect.Message)
	if !ok {
		return protoreflect.Value{}, fmt.Errorf("value is not a message, can't get child path: %v", childMessage)
	}

	return GetValue(p.child(), immediateValue.Message().Interface())
}
