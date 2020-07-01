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
	"strings"
	"text/template"

	pb "github.com/google/xtoproto/proto/recordtoproto"

	"github.com/stoewer/go-strcase"
)

var goFileTemplate = template.Must(template.New("classdef").Parse(
	`package {{.package}}

import (
  "encoding/csv"
  "fmt"
  "io"

  "github.com/google/xtoproto/csvtoprotoparse"
  "github.com/google/xtoproto/protocp"
  "google.golang.org/protobuf/proto"

	pb "{{.proto_import}}"
)

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
}

// NewReader returns a {{.message_type}} reader based on the given generic CSV reader.
func NewReader(r io.Reader, options... csvtoprotoparse.ReaderOption) (*Reader, error) {
  reader := csv.NewReader(r)
	// Disregard the header
	_, err := reader.Read()
	if err != nil {
		return nil, err
	}
	return &Reader{reader, options}, nil
}

// NewMessageReader returns a protocp.MessageReader.
func NewMessageReader(r io.Reader, options... csvtoprotoparse.ReaderOption) (protocp.MessageReader, error) {
  return NewReader(r, options...)
}

func (r *Reader) Options() []csvtoprotoparse.ReaderOption {
  return r.options
}

// Read returns the next {{.message_type}} from the file.
func (r *Reader) Read() (*{{.message_type}}, error) {
	record, err := r.csvReader.Read()
	if err != nil {
		return nil, err
	}
	msg, errs := ParseRow(record)
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

// ReadMessage returns the next {{.message_type}} from the file.
func (r *Reader) ReadMessage() (proto.Message, error) {
	return r.Read()
}

// ParseRow returns a protobuf-version of a CSV row.
func ParseRow(row []string) (*{{.message_type}}, []error) {
	output := &{{.message_type}}{}
  var errs []error
	for _, parser := range fieldParsers {
		if err := parser(row, output); err != nil {
			errs = append(errs, err)
		}
	}
	return output, errs
}

{{.parser_definitions}}
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
	params["parser_definitions"] = fnsCode.definitions
	params["parser_symbols"] = strings.Join(fnsCode.fnNames, ", ") + ","
	if err := goFileTemplate.Execute(strBuilder, params); err != nil {
		return "", err
	}
	return strBuilder.String(), nil
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
	}, nil
}

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
