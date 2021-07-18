package wirepath

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	pb "github.com/google/xtoproto/proto/wirepath"
)

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
	pathPart = `(?:(?:` + protobufStringLiteralDoubleQuoted + `|[^/"])+)`

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
	return FromProto(topLevel)
}

func nextPathElem(literal string) (string, string, error) {
	matchLocs := pathPartRegexp.FindStringIndex(literal)
	if len(matchLocs) != 2 {
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
