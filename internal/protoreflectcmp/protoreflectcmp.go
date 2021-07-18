// Package protoreflectcmp provides testing facilites for using the cmp package
// with protoreflect.
package protoreflectcmp

import (
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	protoMessageType = reflect.ValueOf(func(proto.Message) {}).Type().In(0)
	enumNumberType   = reflect.TypeOf(func(protoreflect.EnumNumber) {}).In(0)

	mainOpt = cmp.FilterValues(func(a, b interface{}) bool {
		asymmetric := func(a, b interface{}) bool {
			_, ok := a.(protoreflect.List)
			if !ok {
				return false
			}
			slice := makeProtoMessageSlice(b)
			if slice == nil {
				return false
			}
			// Check that the element type of the slice is assignable to proto.Message.
			// Now check that elements of a and b are both
			listElemIsProto := reflect.TypeOf(b).Elem().AssignableTo(protoMessageType)

			return listElemIsProto
		}
		return asymmetric(a, b) || asymmetric(b, a)
	}, reflectListTransform)

	reflectListTransform = cmp.Transformer("listToSlice", func(x interface{}) interface{} {
		list := x.(protoreflect.List)
		if !list.IsValid() {
			return nil
		}
		switch list.NewElement().Interface().(type) {
		case protoreflect.ProtoMessage:
			out := make([]proto.Message, list.Len())
			for i := 0; i < list.Len(); i++ {
				out[i] = list.Get(i).Message().Interface()
			}
			return out
		default:
			return list
		}
		// elemType := reflect.TypeOf(list.NewElement().Interface())
		// out := reflect.MakeSlice(reflect.SliceOf(elemType), list.Len(), list.Len())
		// for i := 0; i < list.Len(); i++ {
		// 	out.Index(i).Set(reflect.ValueOf(list.Get(i).Interface()))
		// }
		// return out.Interface()
	})
)

type protoMessageSlice struct {
	value interface{}
}

// makeProtoMessageSlice returns a *protoMessageSlice if the given argument is a
// slice with an element type that implements proto.Message. Otherwise, nil is returned.
func makeProtoMessageSlice(value interface{}) *protoMessageSlice {
	if value == nil {
		return nil
	}
	t := reflect.TypeOf(value)
	if t.Kind() != reflect.Slice {
		return nil
	}
	// Check that the element type of the slice is assignable to proto.Message.
	// Now check that elements of a and b are both
	if !t.Elem().AssignableTo(protoMessageType) {
		return nil
	}
	return &protoMessageSlice{}
}

func (s *protoMessageSlice) isValid() bool { return s != nil }

func (s *protoMessageSlice) transformListToMatch(list protoreflect.List) interface{} {
	if _, isMessage := list.NewElement().Interface().(protoreflect.Message); !isMessage {
		panic(fmt.Errorf("cannot transform list of non-messages to be comparable to a list of messages: %v", list))
	}
	out := make([]proto.Message, list.Len())
	for i := 0; i < list.Len(); i++ {
		out[i] = list.Get(i).Message().Interface()
	}
	return out
}

// func transformValueToMatchStructure(v protoreflect.Value, matchedValue interface{}) (interface{}, bool) {
// 	switch v.Interface().(type) {
// 	case protoreflect.List:
// 		if matchedValue == nil {
// 			return nil, false
// 		}
// 		sliceType := reflect.TypeOf(matchedValue)
// 		if sliceType.Kind() != reflect.Kind() {

// 		}
// 	}
// }

var opt3 = cmp.FilterValues(func(a, b interface{}) bool {
	asymmetric := func(a, b interface{}) bool {
		aAnalysis := analyzeValue(a)
		bAnalysis := analyzeValue(b)
		panic(fmt.Errorf("filterValues(%v, %v)", aAnalysis, bAnalysis))
		if a == nil || b == nil {
			return false
		}
		if list, ok := a.(protoreflect.List); ok {
			return transformListAndSliceToBeComparable(list, b) != nil
		}
		return false
	}
	return asymmetric(a, b) || asymmetric(b, a)
}, cmp.Transformer("transformListAndSliceToBeComparable", transformToComparableList))

var opt2 = cmp.FilterValues(func(a, b interface{}) bool {
	aAnalysis := analyzeValue(a)
	bAnalysis := analyzeValue(b)
	bothNormalizeToSlice := aAnalysis.NormalizedSlice != nil && bAnalysis.NormalizedSlice != nil
	if !bothNormalizeToSlice {
		return false
	}
	// Don't proceed unless at least one argument is a list... two slices are
	// already comparable.
	return aAnalysis.IsList || bAnalysis.IsList
}, cmp.Transformer("normalizeList", func(x interface{}) interface{} {
	return analyzeValue(x).NormalizedSlice
}))

func transformToComparableList(listOrSlice interface{}) *genericComparableList {
	if list, ok := listOrSlice.(protoreflect.List); ok {
		if !list.IsValid() {
			return nil
		}
		elemType := reflect.TypeOf(list.NewElement().Interface())
		listSlice := reflect.New(reflect.SliceOf(elemType))
		listSlice.SetLen(list.Len())
		for i := 0; i < list.Len(); i++ {
			listSlice.Index(i).Set(reflect.ValueOf(list.Get(i).Interface()))
		}
		return normalizeSlice(listSlice)
	}
	sliceType := reflect.TypeOf(listOrSlice)
	if sliceType.Kind() != reflect.Slice {
		return nil
	}

	return nil
}

type analysis struct {
	IsList           bool
	IsSlice          bool
	IsNil            bool
	SliceElementType reflect.Type
	NormalizedSlice  interface{}
	SliceMessageName protoreflect.FullName
}

var (
	boolType    = reflect.TypeOf(bool(false)) // protobuf type: bool
	int32Type   = reflect.TypeOf(int32(0))    // protobuf type: Int32Kind, Sint32Kind, Sfixed32Kind
	int64Type   = reflect.TypeOf(int64(0))    // protobuf type: Int64Kind, Sint64Kind, Sfixed64Kind
	uint32Type  = reflect.TypeOf(uint32(0))   // protobuf type: Uint32Kind, Fixed32Kind
	uint64Type  = reflect.TypeOf(uint64(0))   // protobuf type: Uint64Kind, Fixed64Kind
	float32Type = reflect.TypeOf(float32(0))  // protobuf type: FloatKind
	float64Type = reflect.TypeOf(float64(0))  // protobuf type: DoubleKind
	stringType  = reflect.TypeOf(string(""))  // protobuf type: StringKind
	bytesType   = reflect.TypeOf([]byte{})    // protobuf type: BytesKind
)

func analyzeValue(v interface{}) analysis {
	a := analysis{IsNil: v == nil}
	if v == nil {
		return a
	}
	if list, ok := v.(protoreflect.List); ok {
		a.IsList = true
		if list.IsValid() {
			prototypeInstance := list.NewElement().Interface()
			switch instance := prototypeInstance.(type) {
			case bool:
				a.SliceElementType = boolType
			case int32:
				a.SliceElementType = int32Type
			case int64:
				a.SliceElementType = int64Type
			case uint32:
				a.SliceElementType = uint32Type
			case uint64:
				a.SliceElementType = uint64Type
			case float32:
				a.SliceElementType = float32Type
			case float64:
				a.SliceElementType = float64Type
			case string:
				a.SliceElementType = stringType
			case []byte:
				a.SliceElementType = bytesType
			case protoreflect.EnumNumber: // protobuf tupe: EnumKind
				a.SliceElementType = enumNumberType
			case protoreflect.Message: // protobuf tupe: MessageKind, GroupKind
				a.SliceElementType = protoMessageType
				a.SliceMessageName = instance.Descriptor().FullName()
			default:
				panic("unsupported protoreflect.Value type")
			}

			normalizedSlice := reflect.MakeSlice(reflect.SliceOf(a.SliceElementType), list.Len(), list.Len())
			for i := 0; i < list.Len(); i++ {
				value := list.Get(i).Interface()
				var sliceValue reflect.Value
				switch castValue := value.(type) {
				case protoreflect.Message: // protobuf tupe: MessageKind, GroupKind
					var m proto.Message = castValue.Interface()
					sliceValue = reflect.ValueOf(m)
				default:
					sliceValue = reflect.ValueOf(value)
				}
				normalizedSlice.Index(i).Set(sliceValue)

			}
			a.NormalizedSlice = normalizedSlice.Interface()
		}
	} else {
		typeOfValue := reflect.TypeOf(v)
		typeOfValueKind := typeOfValue.Kind()
		if typeOfValueKind == reflect.Slice {
			a.IsSlice = true

			elementType := typeOfValue.Elem()
			if elementType.Implements(protoMessageType) {
				if elementType.Kind() != reflect.Interface {
					prototype := reflect.New(elementType).Elem().Interface().(proto.Message)
					a.SliceMessageName = prototype.ProtoReflect().Descriptor().FullName()
				} else {
					a.SliceMessageName = protoreflect.FullName("<proto.Message slice>")
				}
			}

			// Normalize the slice... turn []*mypb.MyMessage into []proto.Message.
			for _, candidate := range []reflect.Type{
				enumNumberType,
				protoMessageType,

				boolType,
				int32Type,
				int64Type,
				uint32Type,
				uint64Type,
				float32Type,
				float64Type,
				stringType,
				bytesType,
			} {
				if elementType.AssignableTo(candidate) {
					a.SliceElementType = candidate
					break
				}
			}
			if a.SliceElementType != nil {
				a.NormalizedSlice = makeSliceOfElementType(v, a.SliceElementType).Interface()
			}
		}
	}

	if a.SliceMessageName != "" {
		a.NormalizedSlice = repeatedField{
			MessageName: a.SliceMessageName,
			Values:      a.NormalizedSlice,
		}
	}

	return a
}

type repeatedField struct {
	MessageName protoreflect.FullName
	Values      interface{}
}

func makeSliceOfElementType(srcSlice interface{}, elemType reflect.Type) reflect.Value {
	if reflect.TypeOf(srcSlice).Kind() != reflect.Slice {
		panic("src is not a slice")
	}
	src := reflect.ValueOf(srcSlice)

	normalizedSlice := reflect.MakeSlice(reflect.SliceOf(elemType), src.Len(), src.Len())
	for i := 0; i < src.Len(); i++ {
		value := src.Index(i)
		if !value.Type().AssignableTo(elemType) {
			panic(fmt.Errorf("%v is not assignable to %v", value, elemType))
		}
		normalizedSlice.Index(i).Set(value)
	}
	return normalizedSlice
}

// normalizeSlice transforms slices of
func normalizeSlice(slice interface{}) *genericComparableList {
	sliceType := reflect.TypeOf(slice)
	if sliceType.Kind() != reflect.Slice {
		panic(fmt.Errorf("argument is not a slice:%v", slice))
	}
	panic("cannot normalize slice yet")
}

type genericComparableList struct {
	ElementType string
	Elements    interface{}
}

func transformListAndSliceToBeComparable(list protoreflect.List, slice interface{}) *listSliceComparison {
	if slice == nil || !list.IsValid() {
		return nil
	}
	sliceType := reflect.TypeOf(slice)
	if sliceType.Kind() != reflect.Slice {
		return nil
	}
	listElemPrototype := list.NewElement().Interface()
	// If the list element's value is assignable to the slice's element type,
	// make a new slice with the same type and assign each element of list to an
	// element of the new slice. Return the new slice and the unmodified second
	// argument.
	if reflect.ValueOf(listElemPrototype).Type().AssignableTo(sliceType.Elem()) {
		listSlice := reflect.New(sliceType)
		listSlice.SetLen(list.Len())
		for i := 0; i < list.Len(); i++ {
			listSlice.Index(i).Set(reflect.ValueOf(list.Get(i).Interface()))
		}
		return &listSliceComparison{
			comparisonType: fmt.Sprintf("protoreflect.List element is assignable to %v", sliceType.Elem()),
			a:              listSlice,
			b:              slice,
		}
	}
	return nil
}

type listSliceComparison struct {
	comparisonType string
	a, b           interface{}
}

// Transform returns a cmp.Option that will make protoreflect.List instances
// comparable to slices of proto messages.
func Transform() cmp.Option {
	return opt2
}
