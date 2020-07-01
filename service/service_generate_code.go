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
	"fmt"
	"path"

	"github.com/google/xtoproto/csvtoproto"
	"github.com/stoewer/go-strcase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	spb "github.com/google/xtoproto/proto/service"
)

const defaultProtoFileName = "untitled_record.proto"
const defaultConverterGoFileName = "untitled_record_converter.go"

func (s *service) GenerateCode(ctx context.Context, req *spb.GenerateCodeRequest) (*spb.GenerateCodeResponse, error) {
	if req.GetMapping() == nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "missing input mapping")
	}

	// TODO(reddaly): Support the use case where the mapping .pbtxt file is stored
	// in the repository as the basis for the bazel rule that produces the
	// .proto file. In that case, we need to merge the request mapping with the
	// mapping in the repo.

	genProto := req.GetProtoDefinition() != nil
	genGo := req.GetConverter() != nil
	protoCode, goCode, err := csvtoproto.GenerateCode(req.GetMapping(), genProto, genGo)
	if err != nil {
		return nil, grpc.Errorf(codes.InvalidArgument, "failed to generate code: %v", err)
	}

	var outputProtoFile *spb.GenerateCodeResponse_File
	if genProto {
		codePath, codePathWSRelative, err := s.protoPath(req)
		if err != nil {
			return nil, grpc.Errorf(codes.InvalidArgument, "invalid output specification for .proto file: %v", err)
		}
		// TODO(reddaly): Consider configurable overwrite behavior.
		if err := s.writeFile(ctx, codePath, []byte(protoCode)); err != nil {
			return nil, fileErrToStatusErr(codePath, err)
		}
		outputProtoFile = &spb.GenerateCodeResponse_File{
			WorkspaceRelativePath: codePathWSRelative,
			NewContents:           []byte(protoCode),
		}
	}
	var outputGoFile *spb.GenerateCodeResponse_File
	if genGo {
		codePath, codePathWSRelative, err := s.converterGoPath(req)
		if err != nil {
			return nil, grpc.Errorf(codes.InvalidArgument, "invalid output specification for .go file: %v", err)
		}
		// TODO(reddaly): Consider configurable overwrite behavior.
		if err := s.writeFile(ctx, codePath, []byte(goCode)); err != nil {
			return nil, fileErrToStatusErr(codePath, err)
		}
		outputGoFile = &spb.GenerateCodeResponse_File{
			WorkspaceRelativePath: codePathWSRelative,
			NewContents:           []byte(goCode),
		}
	}
	// TODO(reddaly): Update BUILD rule.

	return &spb.GenerateCodeResponse{
		ProtoFile:       outputProtoFile,
		ConverterGoFile: outputGoFile,
	}, nil
}

// protoPath always returns a non-empty string if error is nil.
//
// The first return value is the path the the proto file including the path
// to the workspace directory. The second return value is the path of the
// proto file relative to the workspace root.
func (s *service) protoPath(req *spb.GenerateCodeRequest) (string, string, error) {
	fileName := func() string {
		if str := req.GetProtoDefinition().GetProtoFileName(); str != "" {
			return str
		}
		if req.GetMapping().GetMessageName() == "" {
			return defaultProtoFileName
		}
		return fmt.Sprintf("%s.proto", strcase.SnakeCase(req.GetMapping().GetMessageName()))
	}()
	fullPath, err := pathFromParts(s.workspacePathForRequest(req), req.GetProtoDefinition().GetDirectory(), fileName)
	if err != nil {
		return "", "", err
	}
	workspaceRelativePath := path.Join(req.GetProtoDefinition().GetDirectory(), fileName)
	return fullPath, workspaceRelativePath, nil
}

// converterGoPath returns the path to the generated go code file.
//
// The first return value is the path to the generated go file including the
// path to the workspace directory. The second return value is the path of the
// file relative to the workspace root.
func (s *service) converterGoPath(req *spb.GenerateCodeRequest) (string, string, error) {
	fileName := func() string {
		if str := req.GetConverter().GetGoFileName(); str != "" {
			return str
		}
		if req.GetMapping().GetMessageName() == "" {
			return defaultConverterGoFileName
		}
		return fmt.Sprintf("%s.go", strcase.SnakeCase(req.GetMapping().GetMessageName()))
	}()
	fullPath, err := pathFromParts(s.workspacePathForRequest(req), req.GetConverter().GetDirectory(), fileName)
	if err != nil {
		return "", "", err
	}

	return fullPath, path.Join(req.GetConverter().GetDirectory(), fileName), nil
}

func (s *service) workspacePathForRequest(req *spb.GenerateCodeRequest) string {
	if req.GetWorkspacePath() != "" {
		return req.GetWorkspacePath()
	}
	return s.defaultWorkspaceDir
}

func pathFromParts(workspace, dir, fileName string) (string, error) {
	if path.IsAbs(dir) || path.IsAbs(fileName) {
		return "", fmt.Errorf("directory %q and fileName %q must be relative to workspace %q", dir, fileName, workspace)
	}
	if fileName == "" {
		return "", fmt.Errorf("must supply an explicit output filename, got empty string")
	}
	return path.Join(workspace, dir, fileName), nil
}
