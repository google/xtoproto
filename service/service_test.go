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

package service

import (
	"context"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"

	pb "github.com/google/xtoproto/proto/recordtoproto"
	rpb "github.com/google/xtoproto/proto/recordtoproto"
	spb "github.com/google/xtoproto/proto/service"
)

var abMapping = &rpb.RecordProtoMapping{
	GoOptions: &rpb.GoOptions{
		GoPackageName: "my_message_converter",
		ProtoImport:   "path/to/my_message_go_proto",
	},
	MessageName: "MyMessage",
	PackageName: "my_package",
	ColumnToFieldMappings: []*rpb.ColumnToFieldMapping{
		&rpb.ColumnToFieldMapping{
			ColName:     "a",
			ColumnIndex: 0,
			ProtoType:   "int64",
			ProtoName:   "a",
			ProtoTag:    1,
		},
		&rpb.ColumnToFieldMapping{
			ColName:     "b",
			ColumnIndex: 1,
			ProtoType:   "string",
			ProtoName:   "b",
			ProtoTag:    2,
		},
	},
}

func Test_service_Infer(t *testing.T) {
	ctx := context.Background()
	unimplementedFileSysService := &service{
		defaultWorkspaceDir: "/dummy-workspace",
		readFile: func(ctx context.Context, path string) ([]byte, error) {
			return nil, nil
		},
		writeFile: func(ctx context.Context, path string, data []byte) error {
			return nil
		},
	}
	tests := []struct {
		name    string
		s       *service
		req     *spb.InferRequest
		want    *spb.InferResponse
		wantErr bool
	}{
		{
			name:    "empty request should cause an error",
			s:       unimplementedFileSysService,
			req:     &spb.InferRequest{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "a,b",
			s:    unimplementedFileSysService,
			req: &spb.InferRequest{
				ExampleInputs: []*spb.InputFile{
					makeInputFile([]byte("a,b\n1,thing\n")),
				},
				InputFormat:   spb.Format_CSV,
				MessageName:   "MyMessage",
				GoPackageName: "my_message_converter",
				GoProtoImport: "path/to/my_message_go_proto",
				PackageName:   "my_package",
			},
			want: &spb.InferResponse{
				BestMappingCandidate: &spb.MappingSet{
					TopLevelMapping: abMapping,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.s.Infer(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("service.Infer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(tt.want, got, protocmp.Transform(), cmpopts.EquateEmpty(),
				protocmp.IgnoreFields(proto.MessageV2(&pb.ColumnToFieldMapping{}), "comment")); diff != "" {
				t.Errorf("uexpected diff in service.Infer results (-want,+got): %s", diff)
			}
		})
	}
}

func Test_service_GenerateCode(t *testing.T) {
	ctx := context.Background()
	unimplementedFileSysService := &service{
		defaultWorkspaceDir: "/dummy-workspace",
		readFile: func(ctx context.Context, path string) ([]byte, error) {
			return nil, nil
		},
		writeFile: func(ctx context.Context, path string, data []byte) error {
			return nil
		},
	}
	tests := []struct {
		name    string
		s       *service
		req     *spb.GenerateCodeRequest
		want    *spb.GenerateCodeResponse
		wantErr bool
	}{
		{
			"a,b with no code generation requests",
			unimplementedFileSysService,
			&spb.GenerateCodeRequest{
				Mapping: abMapping,
			},
			&spb.GenerateCodeResponse{},
			false,
		},
		{
			"a,b with proto code generation request",
			unimplementedFileSysService,
			&spb.GenerateCodeRequest{
				Mapping:       abMapping,
				WorkspacePath: "/not/the/default",
				ProtoDefinition: &spb.GenerateCodeRequest_ProtoDefinition{
					Directory:        "code-path/proto",
					ProtoFileName:    "hello-world.proto",
					UpdateBuildRules: true,
				},
				Converter: &spb.GenerateCodeRequest_Converter{
					Directory:        "converters",
					UpdateBuildRules: true,
				},
			},
			&spb.GenerateCodeResponse{
				ProtoFile: &spb.GenerateCodeResponse_File{
					WorkspaceRelativePath: "code-path/proto/hello-world.proto",
					NewContents:           []byte(""),
				},
				ConverterGoFile: &spb.GenerateCodeResponse_File{
					WorkspaceRelativePath: "converters/my_message.go",
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.s.GenerateCode(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("service.GenerateCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			ignoreFileContents := protocmp.IgnoreFields(proto.MessageV2(&spb.GenerateCodeResponse_File{}), "new_contents")
			if diff := cmp.Diff(tt.want, got, protocmp.Transform(), cmpopts.EquateEmpty(), ignoreFileContents); diff != "" {
				t.Errorf("unexpected diff in service.Infer results (-want,+got): %s", diff)
			}
		})
	}
}

func makeInputFile(content []byte) *spb.InputFile {
	f := &spb.InputFile{
		Spec: &spb.InputFile_InputContent{InputContent: content},
	}
	return f
}
