/* Package wirepath is an xpath-like means of representing a location within a protocol buffer message.

 */
package wirepath

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/golang/glog"
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

// Proto returns the protobuf representation of the WirePath object.
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

// Parse returns a parsed and validated version of the argument.
func Parse(proto *pb.WirePath) (*WirePath, error) {
	return &WirePath{proto}, nil
}

// GetValue returns the value of some path within a protocol buffer message.
func GetValue(p *WirePath, within proto.Message) (protoreflect.Value, error) {
	return getValue(nil, p, within)
}

func getValue(parent, p *WirePath, within proto.Message) (protoreflect.Value, error) {
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
				return protoreflect.Value{}, fmt.Errorf("path references element %d of a non-repeated field", slot)
			}
			if int(slot.RepeatedFieldOffset) >= list.Len() {
				return protoreflect.Value{}, fmt.Errorf("%d is out of range (repeated field length = %d)", slot, list.Len())
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

	return getValue(p, p.child(), immediateValue.Message().Interface())
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

const (
	// Valid protobuf identifier regexp.
	// See https://developers.google.com/protocol-buffers/docs/reference/proto3-spec#identifiers
	protobufIdentifierRegexp = `[A-Za-z][_\dA-Za-z]*`
	mapKeywordIdentifier     = protobufIdentifierRegexp // discriminate later

	stringLiteralElement = `(?:[^"]|\\.)`
	stringLiteral        = `(?:"` + stringLiteralElement + `*")`

	signedDecimalLiteral = `[\+\-]?[1-9][0-9]*`

	// The only slashes that exist within a single path element will be inside of
	// a string, so write a regexp that parses [^/*] plus any string literal.
	//pathPart = `[^/"]*`
	pathPart = `(?:(?:` + protobufStringLiteralDoubleQuoted + `|[^/"])+)`
	//          ^^^^^ any non double quote, non-/ character when outside of a string.
	//                   ^ beginning of string literal
	//                                                 ^ end of string literal

	slotGroups = (`(?:` +
		`\[` +
		`(?:` +
		`(` + signedDecimalLiteral + `)` + `|` +
		`(` + stringLiteral + `)` + `|` +
		`(` + signedDecimalLiteral + `)\:(` + mapKeywordIdentifier + `)` +
		`)` +
		`\]` +
		`)`) //+ intLiteral + `)` + //`(` + protobufIdentifierRegexp + `)?`) +
)

var parseRegexp = regexp.MustCompile(
	`^(?:` +
		`(\.)` + // group 1
		`|` +
		`(\d+)` + // group 2: field number
		`(?:\((` + protobufIdentifierRegexp + `)\))?` + // group 2: field name

		// The map key or repeated field index.
		slotGroups + `?` +
		//`(!ERROR)` +
		`)$`)

var pathPartRegexp = regexp.MustCompile(`^(` + pathPart + `)`)

func ParseString(wirePathLiteral string) (*WirePath, error) {
	wirePathLiteral = strings.TrimSpace(wirePathLiteral)
	if wirePathLiteral == "" {
		return nil, nil
	}
	// TODO(reddaly): Correctly parse !ERROR:"" string.
	var elemStrings []string

	var elems []*pb.WirePath
	rest := wirePathLiteral
	var topLevel, parent *pb.WirePath
	for rest != "" {
		elemString, x, err := nextPathElem(rest)
		if err != nil {
			return nil, fmt.Errorf("error parsing path component %d of %q: %w", len(elemStrings), wirePathLiteral, err)
		}
		rest = x

		elemStrings = append(elemStrings, elemString)
		elem, err := parsePathElem(elemString)
		if err != nil {
			return nil, fmt.Errorf("error parsing path component %d of %q: %w", len(elemStrings), wirePathLiteral, err)
		}
		elems = append(elems, elem)
		if parent == nil {
			parent = elem
			topLevel = elem
		} else {
			parent.Child = elem
		}
	}
	return Parse(topLevel)
}

func nextPathElem(literal string) (string, string, error) {
	matchLocs := pathPartRegexp.FindStringIndex(literal)
	if len(matchLocs) != 2 {
		glog.Infof("%q doesn't match", literal)
		return "", "", fmt.Errorf("failed to parse element of path from: %q", literal)
	}
	firstPart, rest := literal[matchLocs[0]:matchLocs[1]], literal[matchLocs[1]:]
	rest = strings.TrimPrefix(rest, "/")

	return firstPart, rest, nil
}

func parsePathElem(s string) (*pb.WirePath, error) {
	const (
		specialPathGroup       = 1
		fieldNumberGroup       = 2
		fieldNameGroup         = 3
		repeatedFieldSlotGroup = 4
		stringMapKeySlotGroup  = 5
		mapKeySlotGroup        = 6
		mapKeyTypeSlotGroup    = 7
	)
	groups := parseRegexp.FindStringSubmatch(s)
	if len(groups) == 0 {
		return nil, fmt.Errorf("bad path element: %q", s)
	}
	if groups[1] != "" {
		return &pb.WirePath{Element: &pb.WirePath_SpecialPath_{SpecialPath: pb.WirePath_SELF}}, nil
	}

	fieldNumber, err := strconv.ParseInt(groups[fieldNumberGroup], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("unsupported input: %q", s)
	}

	out := &pb.WirePath{
		Element:   &pb.WirePath_FieldNumber{FieldNumber: int32(fieldNumber)},
		FieldName: groups[fieldNameGroup],
	}

	if listIndexStr := groups[repeatedFieldSlotGroup]; listIndexStr != "" {
		listIndex, err := strconv.ParseInt(listIndexStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unsupported input: %q", s)
		}
		out.Slot = &pb.WirePath_RepeatedFieldOffset{RepeatedFieldOffset: listIndex}
	} else if stringLiteral := groups[stringMapKeySlotGroup]; stringLiteral != "" {
		mapKey, err := parseProtobufStringLiteral(stringLiteral)
		if err != nil {
			return nil, err
		}
		out.Slot = &pb.WirePath_MapKeyString{MapKeyString: mapKey}
	} else if mapKeyLiteral, mapKeyType := groups[mapKeySlotGroup], groups[mapKeyTypeSlotGroup]; mapKeyLiteral != "" {
		mapKeyType = strings.ToLower(mapKeyType)
		if err := setSlotBasedOnMapKeyLiteral(out, mapKeyLiteral, mapKeyType); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func MustParse(wirePathLiteral string) *WirePath {
	got, err := ParseString(wirePathLiteral)
	if err != nil {
		panic(err)
	}
	return got
}

func setSlotBasedOnMapKeyLiteral(out *pb.WirePath, literal, typeName string) error {
	typeName = strings.ToLower(typeName)
	switch typeName {
	case "bool":
		v := false
		switch literal {
		case "0":
			v = false
		case "1":
			v = true
		default:
			return fmt.Errorf("invalid bool map key: %q", literal)
		}
		out.Slot = &pb.WirePath_MapKeyBool{MapKeyBool: v}

	case "uint32", "fixed32":
		number, err := strconv.ParseUint(literal, 10, 32)
		if err != nil {
			return fmt.Errorf("map key literal parse error: %w", err)
		}
		switch typeName {
		case "uint32":
			out.Slot = &pb.WirePath_MapKeyUint32{MapKeyUint32: uint32(number)}
		case "fixed32":
			out.Slot = &pb.WirePath_MapKeyFixed32{MapKeyFixed32: uint32(number)}
		default:
			panic("bug")
		}
	case "uint64", "fixed64":
		number, err := strconv.ParseUint(literal, 10, 64)
		if err != nil {
			return fmt.Errorf("map key literal parse error: %w", err)
		}
		switch typeName {
		case "uint64":
			out.Slot = &pb.WirePath_MapKeyUint64{MapKeyUint64: uint64(number)}
		case "fixed64":
			out.Slot = &pb.WirePath_MapKeyFixed64{MapKeyFixed64: uint64(number)}
		default:
			panic("bug")
		}
	case "int32", "sint32", "sfixed32":
		number, err := strconv.ParseInt(literal, 10, 32)
		if err != nil {
			return fmt.Errorf("map key literal parse error: %w", err)
		}
		switch typeName {
		case "int32":
			out.Slot = &pb.WirePath_MapKeyInt32{MapKeyInt32: int32(number)}
		case "sint32":
			out.Slot = &pb.WirePath_MapKeySint32{MapKeySint32: int32(number)}
		case "sfixed32":
			out.Slot = &pb.WirePath_MapKeySfixed64{MapKeySfixed64: int64(number)}
		default:
			panic("bug")
		}
	case "int64", "sint64", "sfixed64":
		number, err := strconv.ParseInt(literal, 10, 64)
		if err != nil {
			return fmt.Errorf("map key literal parse error: %w", err)
		}
		switch typeName {
		case "int64":
			out.Slot = &pb.WirePath_MapKeyInt64{MapKeyInt64: int64(number)}
		case "sint64":
			out.Slot = &pb.WirePath_MapKeySint64{MapKeySint64: int64(number)}
		case "sfixed64":
			out.Slot = &pb.WirePath_MapKeySfixed64{MapKeySfixed64: int64(number)}
		default:
			panic("bug")
		}

	default:
		return fmt.Errorf("unknown map key type %q", typeName)
	}
	return nil
}
