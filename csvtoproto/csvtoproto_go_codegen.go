// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package csvtoproto

import (
	"fmt"
	"go/format"
	"strings"
	"text/template"

	pb "github.com/google/xtoproto/proto/recordtoproto"

	"github.com/stoewer/go-strcase"
)

var goFileTemplate = template.Must(template.New("classdef").Parse(
	`package {{.package}}

import (
	"encoding/csv"
	"io"
	"reflect"
	"time"
	"fmt"

	"github.com/google/xtoproto/csvtoprotoparse"
	"github.com/google/xtoproto/protocp"
	"google.golang.org/protobuf/proto"
	"github.com/google/xtoproto/csvcoder"
	"github.com/google/xtoproto/textcoder"

	pb "{{.proto_import}}"
)

// Unused vars to ensure the imports are used.
var (
	_ = time.Now
	_ = textcoder.NewRegistry
	_ = fmt.Sprintf
)

{{.record_struct_definition}}

func newRecord() *{{.struct_name}} { return &{{.struct_name}}{} }


var fieldParsers = []func(row []string, output *{{.message_type}}) error {
	{{.parser_symbols}}
}

var parseRowReaderHooks []func(reader *Reader, parsedMsg *{{.message_type}}, parseErrors []error) error

// AddReaderParseRowHook should be used by custom parsing libraries to add post-read parsing
// procedures that should run after standard parsers have run.
//
// Warning: The function passed will only be called when rows are being read by a Reader; it will
// NOT be called when ParseRow is called independently.
func AddReaderParseRowHook(fn func(reader *Reader, parsedMsg *{{.message_type}}, parseErrors []error) error) {
  parseRowReaderHooks = append(parseRowReaderHooks, fn)
}

// Sample is an empty protobuf for the record type parsed by this library.
var Sample = &{{.message_type}}{}

// Reader is a layer on top of csv.Reader for {{.message_type}} messages.
type Reader struct {
	csvReader *csv.Reader
	options []csvtoprotoparse.ReaderOption
	fileParser *csvcoder.FileParser
}

// NewReader returns a {{.message_type}} reader based on the given generic CSV reader.
func NewReader(r io.Reader, options... csvtoprotoparse.ReaderOption) (*Reader, error) {
	reader := csv.NewReader(r)

	fileParser, err := csvcoder.NewFileParser(reader, "input.csv", newRecord())
	if err != nil {
		return nil, err
	}
	return &Reader{reader, options, fileParser}, nil
}

func (r *Reader) Options() []csvtoprotoparse.ReaderOption {
  return r.options
}

// Read returns the next {{.message_type}} from the file.
func (r *Reader) Read() (*{{.message_type}}, error) {
	goRec, err := r.fileParser.Read()
	if err != nil {
		return nil, err
	}
	msg, err := goRec.(*{{.struct_name}}).Proto()
	errs := []error{}
	if err != nil {
		errs = []error{err}
	}
	for _, hook := range parseRowReaderHooks {
		if err := hook(r, msg, errs); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return msg, errs[0]
	}
	return msg, nil
}


// ReadAll returns the remaining {{.message_type}} values from the file.
func (r *Reader) ReadAll() (records []*{{.message_type}}, err error) {
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return records, nil
		} else if err != nil {
			return records, err
		}
		records = append(records, rec)
	}
}

// ReadMessage returns the next {{.message_type}} from the file. It is like Read() but returns
// a generic proto.Message instead of a specialized *{{.message_type}}.
func (r *Reader) ReadMessage() (proto.Message, error) {
	return r.Read()
}

// NewMessageReader returns a protocp.MessageReader.
func NewMessageReader(r io.Reader, options... csvtoprotoparse.ReaderOption) (protocp.MessageReader, error) {
  return NewReader(r, options...)
}
`))

var parseFnTemplate = template.Must(template.New("classdef").Parse(
	`const {{.column_index_const}} = {{.column_index}}

// {{.func_name}} parses the {{.column_name}} field of the CSV row into output.
func {{.func_name}}(row []string, output *{{.message_type}}) error {
  if colCount := len(row); {{.column_index_const}} >= colCount {
    return fmt.Errorf("row must have at least %d columns, got %d", {{.column_index_const}} + 1, colCount)
  }
	rawValue :=  row[{{.column_index_const}}]
	parsedValue, err := {{.parse_value_funcall}}
	if err != nil {
		return err
	}
	output.{{.go_field_name}} = parsedValue
	return nil
}
`))

func (cg *codeGenerator) recordStructTypeName() string {
	return strcase.LowerCamelCase(cg.mapping.MessageName) + "Record"
}

func (cg *codeGenerator) goCode() (string, error) {
	strBuilder := &strings.Builder{}
	params, err := cg.sharedTemplateParams()
	if err != nil {
		return "", err
	}

	fnsCode, err := cg.parseFns()
	if err != nil {
		return "", err
	}
	structCode, err := cg.makeStructCode()
	if err != nil {
		return "", err
	}

	params["record_struct_definition"] = structCode.structDef
	params["to_proto_impl"] = "return nil, fmt.Errorf(`problem`)"
	params["struct_name"] = cg.recordStructTypeName()
	params["parser_definitions"] = fnsCode.definitions
	params["parser_symbols"] = strings.Join(fnsCode.fnNames, ", ") + ","
	params["parser_symbols"] = ""
	if err := goFileTemplate.Execute(strBuilder, params); err != nil {
		return "", err
	}

	preformat := strBuilder.String()

	formatted, err := format.Source([]byte(preformat))
	if err != nil {
		return "", err
	}

	return string(formatted), nil
}

func (cg *codeGenerator) sharedTemplateParams() (map[string]string, error) {
	if cg.mapping.GetGoOptions() == nil {
		return nil, fmt.Errorf("must specify go_options field in CSVProtoMapping")
	}
	if cg.mapping.GetGoOptions().GetGoPackageName() == "" {
		return nil, fmt.Errorf("must specify non-empty package in go_options field of CSVProtoMapping")
	}
	return map[string]string{
		"package":      cg.mapping.GoOptions.GoPackageName,
		"proto_import": cg.mapping.GoOptions.ProtoImport,
		"message_type": fmt.Sprintf("pb.%s", cg.mapping.MessageName),
		"struct_name":  cg.recordStructTypeName(),
	}, nil
}

type structCode struct {
	structDef string
}

func (cg *codeGenerator) makeStructCode() (*structCode, error) {
	structName := cg.recordStructTypeName()
	params, err := cg.sharedTemplateParams()
	if err != nil {
		return nil, err
	}

	topLevelLines := []string{}
	fieldLines := []string{""}
	var toProtoInitStatements, protoFieldLiterals []string

	for i, c2f := range cg.mapping.ColumnToFieldMappings {
		if c2f.Ignored {
			continue
		}

		fieldName := strcase.UpperCamelCase(c2f.ProtoName)
		fieldType, err := getFieldTypeCode(c2f)
		if err != nil {
			return nil, fmt.Errorf("failed to generate code for mapping[%d] = %v: %w", i, c2f, err)
		}
		if fieldType.topLevelCode != "" {
			topLevelLines = append(topLevelLines, fieldType.topLevelCode)
		}
		fieldLines = append(fieldLines, fmt.Sprintf("%s %s `csv:%q`", fieldName, fieldType.typeName, c2f.GetColName()))

		expr, err := getGoToProtoFieldExpression(
			fmt.Sprintf("r.%s", fieldName),
			strcase.LowerCamelCase("parsed_"+c2f.GetProtoName()),
			c2f.GetProtoType())
		if err != nil {
			return nil, fmt.Errorf("failed to handle proto field %q", c2f.GetProtoName())
		}
		if expr.parseStatements != "" {
			toProtoInitStatements = append(toProtoInitStatements, expr.parseStatements)
		}

		protoFieldLiterals = append(protoFieldLiterals, fmt.Sprintf("%s: %s,", strcase.UpperCamelCase(c2f.ProtoName), expr.valueExpr))
	}

	structDef := fmt.Sprintf("type %s struct{%s\n}", structName, strings.Join(fieldLines, "\n  "))

	params["parse_section"] = strings.Join(toProtoInitStatements, "\n")
	params["field_type_declarations"] = strings.Join(topLevelLines, "\n")
	params["field_literals_section"] = strings.Join(protoFieldLiterals, "\n")

	b := &strings.Builder{}
	if err := toProtoTemplate.Execute(b, params); err != nil {
		return nil, err
	}

	return &structCode{structDef + b.String()}, nil
}

var toProtoTemplate = template.Must(template.New("toProtoTemplate").Parse(
	`
func (r *{{.struct_name}}) Proto() (*{{.message_type}}, error) {
	var err error
	{{.parse_section}}
	return &{{.message_type}}{
		{{.field_literals_section}}
	}, err
}

{{.field_type_declarations}}

func init() {
	csvcoder.RegisterRowStruct(reflect.TypeOf(newRecord()))
}

`))

type parseFnsCode struct {
	fnNames     []string
	definitions string
}

func (cg *codeGenerator) parseFns() (*parseFnsCode, error) {
	fnsCode := &parseFnsCode{}
	for _, c2f := range cg.mapping.ColumnToFieldMappings {
		if c2f.Ignored {
			continue
		}
		singleFnCode, err := cg.parseFn(c2f)
		if err != nil {
			return nil, err
		}
		fnsCode.fnNames = append(fnsCode.fnNames, singleFnCode.fnName)
		fnsCode.definitions += singleFnCode.definition + "\n\n"
	}
	return fnsCode, nil
}

type parseFnCode struct {
	fnName, definition string
}

func (cg *codeGenerator) parseFn(c2f *pb.ColumnToFieldMapping) (*parseFnCode, error) {
	params, err := cg.sharedTemplateParams()
	if err != nil {
		return nil, err
	}
	fnName := fmt.Sprintf("parse%s", strcase.UpperCamelCase(c2f.ProtoName))
	params["func_name"] = fnName
	params["go_field_name"] = strcase.UpperCamelCase(c2f.ProtoName)
	params["column_index"] = fmt.Sprintf("%d", c2f.ColumnIndex)
	params["column_index_const"] = fmt.Sprintf("colIndex%s", strcase.UpperCamelCase(c2f.ProtoName))
	params["column_name"] = c2f.ColName
	parseValueFuncall := ""
	switch c2f.ProtoType {
	case "int32":
		parseValueFuncall = "csvtoprotoparse.ParseInt32(rawValue)"
	case "int64":
		parseValueFuncall = "csvtoprotoparse.ParseInt64(rawValue)"
	case "float":
		parseValueFuncall = "csvtoprotoparse.ParseFloat(rawValue)"
	case "double":
		parseValueFuncall = "csvtoprotoparse.ParseDouble(rawValue)"
	case "string":
		parseValueFuncall = "csvtoprotoparse.ParseString(rawValue)"
	case "google.protobuf.Timestamp":
		parseValueFuncall = fmt.Sprintf("csvtoprotoparse.ParseTimestamp(rawValue, %q)", c2f.GetTimeFormat().GoLayout)
	default:
		return nil, fmt.Errorf("unexpected type: %q", c2f.ProtoType)
	}
	params["parse_value_funcall"] = parseValueFuncall

	codeBuilder := &strings.Builder{}
	if err := parseFnTemplate.Execute(codeBuilder, params); err != nil {
		return nil, err
	}
	return &parseFnCode{fnName, codeBuilder.String()}, nil
}

type fieldTypeCode struct {
	// Go code to be inserted at the top level of the file.
	topLevelCode, typeName string
}

func getFieldTypeCode(c2f *pb.ColumnToFieldMapping) (*fieldTypeCode, error) {
	switch protoType := c2f.GetProtoType(); protoType {
	case "int32":
		return &fieldTypeCode{"", "int32"}, nil
	case "int64":
		return &fieldTypeCode{"", "int64"}, nil
	case "float":
		return &fieldTypeCode{"", "float32"}, nil
	case "double":
		return &fieldTypeCode{"", "float64"}, nil
	case "string":
		return &fieldTypeCode{"", "string"}, nil
	case "google.protobuf.Timestamp":
		typeName := strcase.LowerCamelCase(c2f.GetProtoName() + "Time")
		tz := c2f.GetTimeFormat().GetTimeZoneName()
		if tz == "" {
			tz = "UTC"
		}
		code, err := templateExecString(timeTypeTemplate, map[string]string{
			"T":           typeName,
			"time_layout": c2f.GetTimeFormat().GetGoLayout(),
			"tz":          tz,
		})
		if err != nil {
			return nil, err
		}
		return &fieldTypeCode{code, typeName}, nil
	default:
		return nil, fmt.Errorf("unexpected type: %q", protoType)
	}
}

var timeTypeTemplate = template.Must(template.New("timeType").Parse(`
type {{.T}} time.Time

// time returns the underlying time.Time of a {{.T}} object.
func (t {{.T}}) time() time.Time {
	return time.Time(t)
}

func init() {
	const layout = {{.time_layout | printf "%q"}}
	location := csvtoprotoparse.MustLoadLocation({{.tz | printf "%q"}})
	textcoder.Register(
		reflect.TypeOf({{.T}}{}),
		func(t {{.T}}) (string, error) {
			return t.time().In(location).Format(layout), nil
		},
		func(s string, dst *{{.T}}) error {
			t, err := time.ParseInLocation(layout, s, location)
			if err != nil {
				return fmt.Errorf("error parsing {{.T}}: %w", err)
			}
			*dst = {{.T}}(t)
			return nil
		},

	)
}
`))

type transformExpr struct {
	// Go statements to execute before the valueExpr is valid.
	parseStatements string
	// Go code that may be used where the an expression is needed with the same
	// type as the output type.
	valueExpr string
}

// inExpr is an expression of the input value. outVar is a variable the
// transformExpr may use to store the output
func getGoToProtoFieldExpression(inExpr, outVar, protoType string) (*transformExpr, error) {
	switch protoType {
	case "int32", "int64", "float", "double", "string":
		return &transformExpr{"", inExpr}, nil
	case "google.protobuf.Timestamp":
		return &transformExpr{
			fmt.Sprintf(`
%s, err := csvtoprotoparse.TimeToTimestamp(%s.time())
if err != nil {
	return nil, err
}
`, outVar, inExpr),
			outVar,
		}, nil
	default:
		return nil, fmt.Errorf("unexpected type: %q", protoType)
	}
}

func templateExecString(t *template.Template, data interface{}) (string, error) {
	b := &strings.Builder{}
	if err := t.Execute(b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}
