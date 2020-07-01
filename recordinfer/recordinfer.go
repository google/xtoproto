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

// Package recordinfer guesses the types of record columns and uses these to generate a
// RecordProtoMapping object that in turn may be used to generate a .proto definition and
// record-to-proto parser.
package recordinfer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stoewer/go-strcase"

	pb "github.com/google/xtoproto/proto/recordtoproto"
)

// InferredProto contains the results of a record oriented inference process. The structure
// contains type information to describe the record schema and how these fields map to
// proto message fields.
type InferredProto struct {
	packageName string
	messageName string
	columns     []*inferredColumn
	goOpts      *pb.GoOptions
}

// Code returns the source for a .proto file.
func (ip *InferredProto) Code() string {
	var imports []string
	body := ""
	for _, col := range ip.columns {
		body += fmt.Sprintf("%s\n", col.protoFieldCode())
		imports = append(imports, col.columnType.protoImports()...)
	}

	return fmt.Sprintf(`syntax = "proto3";

package %s;

%s

message %s {
%s
}`, ip.packageName, strings.Join(imports, "\n"), ip.messageName, body)
}

// Mapping returns a protobuf representation of the inferred mapping between Record and proto.
// This mapping may be manually adjusted by the user before a code generation step, if desired.
func (ip *InferredProto) Mapping() *pb.RecordProtoMapping {
	m := &pb.RecordProtoMapping{
		PackageName: ip.packageName,
		MessageName: ip.messageName,
		GoOptions:   ip.goOpts,
	}
	for _, col := range ip.columns {
		fieldMapping := &pb.ColumnToFieldMapping{
			ProtoImports: col.columnType.protoImports(),
			ColumnIndex:  int32(col.tag - 1),
			ColName:      col.csvColumnName,
			ProtoType:    col.columnType.protoType(),
			ProtoName:    col.fieldName,
			ProtoTag:     int32(col.tag),
			Comment:      col.comment,
		}
		col.columnType.updateMapping(fieldMapping)
		m.ColumnToFieldMappings = append(m.ColumnToFieldMappings, fieldMapping)
	}
	return m
}

// FormattedMapping returns a text proto formatted version of inferred mapping based
// on the given template.
func (ip *InferredProto) FormattedMapping(template *pb.RecordProtoMapping) string {
	out := &pb.RecordProtoMapping{}
	proto.Merge(out, template)
	proto.Merge(out, ip.Mapping())

	return fmt.Sprintf(`# proto-file: github.com/google/xtoproto/proto/recordtoproto/recordtoproto.proto
# proto-message: xtoproto.RecordProtoMapping

%s
`, proto.MarshalTextString(out))
}

// RecordBasedInferrer provides a builder interface to an InferredProto.
type RecordBasedInferrer struct {
	rows [][]string
	opts *Options
}

// AddRow appends a row to the builder's set of rows. Returns an error if the number of columns in the new row does not
// match the number of columns in the first row added.
func (b *RecordBasedInferrer) AddRow(row []string) error {
	if (len(b.rows) > 0) && (len(row) != len(b.rows[0])) {
		return fmt.Errorf("invalid row length; expected %d got %d for row %s", len(b.rows[0]), len(row), row)
	}

	b.rows = append(b.rows, row)

	return nil
}

// Build constructs an InferredProto using the builder's internal data.
func (b *RecordBasedInferrer) Build() (*InferredProto, error) {
	if len(b.rows) < 2 {
		return nil, fmt.Errorf("not enough rows to infer types: %d", len(b.rows))
	}
	numCols := len(b.rows[0])
	if numCols == 0 {
		return nil, fmt.Errorf("not enough columns to infer types: %d", numCols)
	}

	var gOpts *pb.GoOptions
	if b.opts.GoPackageName != "" || b.opts.GoProtoImport != "" {
		gOpts = &pb.GoOptions{
			GoPackageName: b.opts.GoPackageName,
			ProtoImport:   b.opts.GoProtoImport,
		}
	}

	result := &InferredProto{
		messageName: b.opts.MessageName,
		packageName: b.opts.PackageName,
		goOpts:      gOpts,
	}

	for i := 0; i < numCols; i++ {
		cv := &columnValues{i, b.rows}
		comment := cv.statisticalComment()
		colType, err := cv.inferType(b.opts)
		if err != nil {
			return nil, err
		}
		result.columns = append(result.columns, &inferredColumn{
			csvColumnName: cv.columnName(),
			fieldName:     columnNameToFieldName(cv.columnName()),
			columnType:    colType,
			tag:           i + 1,
			comment:       comment,
		})
	}

	return result, nil
}

// NewRecordBasedInferrer creates a new RecordBasedInferrer.
func NewRecordBasedInferrer(opts *Options) *RecordBasedInferrer {
	return &RecordBasedInferrer{
		opts: opts,
	}
}

type inferredColumn struct {
	fieldName     string
	csvColumnName string
	columnType    columnType
	tag           int
	comment       string
}

func (c *inferredColumn) protoFieldCode() string {
	return fmt.Sprintf("  %s %s = %d;", c.columnType.protoType(), c.fieldName, c.tag)
}

// Options contains inference configuration parameters.
type Options struct {
	// MessageName is the name of the output message. This name should be a short name,
	// not a fully qualified name.
	MessageName string

	// The value to use in the package statement of the output .proto file.
	PackageName string

	// An values to be used in the GoOptions output mapping.
	// See http://cs/symbol:csvtoproto.GoOptions.
	GoPackageName, GoProtoImport string

	// TimestampLocation is the time zone name used to parse timestamps that do not have an explicit timezone.
	TimestampLocation *time.Location
}

type columnValues struct {
	index int
	rows  [][]string
}

func (cv *columnValues) columnName() string {
	return cv.rows[0][cv.index]
}

func (cv *columnValues) rawValues() []string {
	var values []string
	for _, row := range cv.rows[1:] {
		values = append(values, row[cv.index])
	}
	return values
}

const valuesToDisplayInStatisticalComment = 5

// statisticalComment returns a human-readable description of the values of the column based
// on the values inspected.
func (cv *columnValues) statisticalComment() string {
	counts := make(map[string]int)
	rawValues := cv.rawValues()
	for _, rv := range rawValues {
		counts[rv]++
	}
	var keys []string
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ki, kj := keys[i], keys[j]
		ci, cj := counts[ki], counts[kj]
		if ci > cj {
			return true
		}
		if ci < cj {
			return false
		}
		// For stability, sort by string if frequency is the same.
		return ki < kj
	})
	dispKeys := keys
	if len(dispKeys) > valuesToDisplayInStatisticalComment {
		dispKeys = dispKeys[0:valuesToDisplayInStatisticalComment]
	}
	for i, key := range dispKeys {
		dispKeys[i] = fmt.Sprintf("%q (%d)", key, counts[key])
	}
	return fmt.Sprintf("Field type inferred from %d unique values in %d rows; %d most common: %s",
		len(counts), len(rawValues), len(dispKeys), strings.Join(dispKeys, "; "))
}

func (cv *columnValues) inferType(opts *Options) (columnType, error) {
	var inferrers []func(string) (columnType, error)
	inferrers = append(inferrers, timeFormatInferrers(opts.TimestampLocation)...)
	inferrers = append(inferrers, inferInt64Format, inferFloat32Format)
	// TODO(reddaly): Improve this algorithm to work for more input Records,
	// especially those with null values or those with ambiguous values.
	for _, inferer := range inferrers {
		var colType columnType
		everyValueIsColType := true
		for _, rawValue := range cv.rawValues() {
			var err error
			newColType, err := inferer(rawValue)
			if err != nil {
				return nil, err
			}
			if newColType == nil {
				everyValueIsColType = false
				break
			}
			if colType == nil {
				colType = newColType
				continue
			}
			if !columnTypesEqual(colType, newColType) {
				everyValueIsColType = false
				break
			}
		}
		if colType != nil && everyValueIsColType {
			return colType, nil
		}
	}
	return &stringColumnType{}, nil
}

type columnType interface {
	protoType() string
	protoImports() []string
	updateMapping(mapping *pb.ColumnToFieldMapping)
}

// columnTypesEqual reports if two columnTypes are equivalent.
func columnTypesEqual(a, b columnType) bool {
	return a.protoType() == b.protoType()
}

var (
	multipleUnderscores = regexp.MustCompile(`_+`)
	notFieldNameChar    = regexp.MustCompile(`[^_a-zA-Z0-9]`)
)

func columnNameToFieldName(colName string) string {
	s := strings.ReplaceAll(colName, " ", "_")
	s = strings.ReplaceAll(s, "(", "_")
	s = strings.ReplaceAll(s, ")", "_")
	s = notFieldNameChar.ReplaceAllString(s, "_")
	s = multipleUnderscores.ReplaceAllString(s, "_")
	s = strings.TrimPrefix(s, "_")
	s = strings.TrimSuffix(s, "_")
	return strcase.SnakeCase(s)
}
