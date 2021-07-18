// Package protoreflectcmp provides testing facilites for using the cmp package
// with protoreflect.
package protoreflectcmp

import (
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	genericMessageName = "<proto.Message interface - see protoreflectcmp.IgnoreElementType>"
)

// Transform returns a cmp.Option that will make protoreflect.List instances
// comparable to slices of proto messages.
//
// The slice type used for comparison should be a slice of some non-interface
// type. cmp.Equal(<some protoreflect.List instance>,
// []proto.Message{&mypb.X{}}) will not work, while cmp.Equal(<some
// protoreflect.List instance>, []*mypb.X{&mypb.X{}}) will.
func Transform(thisLibraryOpt ...Option) cmp.Option {
	var opts cmp.Options
	opts = append(opts, compareListsOpt)
	p := &params{
		checkListElementType: true,
	}
	for _, o := range thisLibraryOpt {
		o.setParams(p)
	}

	if !p.checkListElementType {
		opts = append(opts, cmpopts.IgnoreFields(repeatedField{}, "MessageName"))
	}

	return opts
}

// Option configures the Transform function.
type Option struct {
	setParams func(p *params)
}

type params struct {
	checkListElementType bool
}

// If specified, ignores differences between the element types of two lists.
// This may be needed if comparing []proto.Message{} to protoreflect.List.
func IgnoreElementType() Option {
	return Option{
		setParams: func(p *params) {
			p.checkListElementType = false
		},
	}
}

var (
	protoMessageType = reflect.ValueOf(func(proto.Message) {}).Type().In(0)
	enumNumberType   = reflect.TypeOf(func(protoreflect.EnumNumber) {}).In(0)
)

var compareListsOpt = cmp.FilterValues(func(a, b interface{}) bool {
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
					a.SliceMessageName = protoreflect.FullName(genericMessageName)
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
