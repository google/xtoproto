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

// Package csvtoproto generates a .proto file and a .go file from a go/csv-to-proto mapping file.
package csvtoproto

import (
	"fmt"
	"sort"
	"strings"

	"github.com/golang/glog"
	wordwrap "github.com/mitchellh/go-wordwrap"

	pb "github.com/google/xtoproto/proto/recordtoproto"
)

// GenerateCode returns a .proto file based on the RecordProtoMapping.
func GenerateCode(mapping *pb.RecordProtoMapping, genProto, genGo bool) (string, string, error) {
	cg := &codeGenerator{mapping}
	protoCode, goCode := "", ""
	if genGo {
		var err error
		goCode, err = cg.goCode()
		if err != nil {
			return "", "", err
		}
	}
	if genProto {
		protoCode = cg.protoCode()
	}
	return protoCode, goCode, nil
}

type codeGenerator struct {
	mapping *pb.RecordProtoMapping
}

const fieldIndent = 2

func (cg *codeGenerator) protoCode() string {
	var imports []string
	fieldPrefix := strings.Repeat(" ", fieldIndent)
	var fieldDefs []*pb.FieldDefinition
	for _, field := range cg.mapping.ColumnToFieldMappings {
		if field.Ignored {
			continue
		}
		comment := fmt.Sprintf("csv field: %q", field.ColName)
		if field.Comment != "" {
			comment = fmt.Sprintf("%s\n\n%s", field.Comment, comment)
		}

		fieldDefs = append(fieldDefs, &pb.FieldDefinition{
			Comment:      comment,
			ProtoImports: field.ProtoImports,
			ProtoName:    field.ProtoName,
			ProtoTag:     field.ProtoTag,
			ProtoType:    field.ProtoType,
		})
	}
	fieldDefs = append(fieldDefs, cg.mapping.ExtraFieldDefinitions...)
	var fieldCodeSections []string
	for _, field := range fieldDefs {
		section := fmt.Sprintf("%s%s%s %s = %d;", formatProtoComment(field.Comment, fieldIndent), fieldPrefix, field.ProtoType, field.ProtoName, field.ProtoTag)
		imports = append(imports, field.ProtoImports...)
		fieldCodeSections = append(fieldCodeSections, section)
	}

	return fmt.Sprintf(`syntax = "proto3";

package %s;

%s

message %s {
%s
}
`, cg.mapping.PackageName, importStatements(imports), cg.mapping.MessageName, strings.Join(fieldCodeSections, "\n\n"))
}

const protoWrapColumn = 80

// formatProtoComment returns the empty string or a newline-terminated .proto comment string based
// on a given human readable content without any comment syntax (i.e. no "//" before each line).
//
// Any lines in the comment string will be respected. Newlines may be introduced if a line goes
// beyond the 80 column limit.
func formatProtoComment(comment string, indent int) string {
	if comment == "" {
		return ""
	}
	out := ""
	linePrefix := strings.Repeat(" ", indent) + "// "
	maxContentLineLength := uint(protoWrapColumn - len(linePrefix))
	comment = wordwrap.WrapString(comment, maxContentLineLength)

	for _, line := range strings.Split(comment, "\n") {
		formattedLine := strings.TrimRight(linePrefix+line, " ")

		if len(line) > protoWrapColumn {
			glog.Warningf("despite word wrapping, field comment %q results in line length %d, max recommended is %d", comment, len(line), protoWrapColumn)
		}
		out += formattedLine + "\n"
	}
	return out
}

func importStatements(paths []string) string {
	paths = sortImports(paths)

	var statements []string
	for _, path := range paths {
		statements = append(statements, fmt.Sprintf("import %q;", path))
	}
	return strings.Join(statements, "\n")
}

func sortImports(paths []string) []string {
	m := make(map[string]bool)
	for _, p := range paths {
		m[p] = true
	}
	var out []string
	for key, _ := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
