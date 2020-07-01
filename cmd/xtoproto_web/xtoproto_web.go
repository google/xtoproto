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

// +build js wasm

// Program xtoproto_web is a web application intended to be compiled with WASM
// that infers .proto definitions from record-oriented files (CSV, XML, etc.).
//
// The application can be run with the following command:
//
//     GOOS=js GOARCH=wasm go build  -o main.wasm xtoproto_web.go && goexec 'http.ListenAndServe(`:8080`, http.FileServer(http.Dir(`.`)))'
//
// With bazel:
//
//     bazel build --platforms=@io_bazel_rules_go//go/toolchain:js_wasm //cmd/...
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"syscall/js"
	"time"

	"github.com/google/xtoproto/service"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	spb "github.com/google/xtoproto/proto/service"
)

const (
	outDirMode  os.FileMode = 0770
	outFileMode os.FileMode = 0660
)

var (
	cfg = &config{
		defaultWorkspaceDir: "/tmp/example-workspace",
		csvContents: `hello,name
5,5
f,x`,
	}

	readFile service.FileReaderFunc = func(_ context.Context, path string) ([]byte, error) {
		return nil, fmt.Errorf("reading files not supported on web (%q)", path)
	}
	writeFile service.FileWriterFunc = func(_ context.Context, path string, data []byte) error {
		// writing files not supported on web
		return nil
	}

	defaultInferRequest = func() *spb.InferRequest {
		return &spb.InferRequest{
			GoPackageName: "example",
			GoProtoImport: "generated/example_go_proto",
			InputFormat:   spb.Format_CSV,
			MessageName:   "ExampleRecord",
			PackageName:   "mycompany.mypackage",
		}
	}
	defaultGenerateCodeRequest = func() *spb.GenerateCodeRequest {
		return &spb.GenerateCodeRequest{
			ProtoDefinition: &spb.GenerateCodeRequest_ProtoDefinition{
				Directory:        "generated",
				ProtoFileName:    "example.proto",
				UpdateBuildRules: true,
			},
			Converter: &spb.GenerateCodeRequest_Converter{
				Directory:        "generated/go",
				GoFileName:       "exampleconv.go",
				UpdateBuildRules: true,
			},
		}
	}
)

type config struct {
	defaultWorkspaceDir string
	csvContents         string
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Printf("fatal error: %v", err)
	}
	time.Sleep(time.Hour * 10000)
}

func run(ctx context.Context) error {
	registerJSEntryPoints(service.New(cfg.defaultWorkspaceDir, readFile, writeFile))
	return nil
}

type jsRequest struct {
	InferRequest        string `json:"infer_request"`
	GenerateCodeRequest string `json:"codegen_request"`
	CSV                 string `json:"csv"`
}

type jsResponse struct {
	Error            string                    `json:"error"`
	InferResponse    *spb.InferResponse        `json:"infer_response"`
	CodeGenResponse  *spb.GenerateCodeResponse `json:"codegen_response"`
	EffectiveRequest *jsRequest                `json:"request"`
}

func unmarshalJSONMerge(str string, dst proto.Message) error {
	dst1 := proto.Clone(dst)
	if err := prototext.Unmarshal([]byte(str), dst1); err != nil {
		return err
	}
	proto.Merge(dst, dst1)
	return nil
}

func handleJSRequest(s spb.XToProtoServiceServer, req *jsRequest) *jsResponse {
	ctx := context.Background()
	req1 := defaultInferRequest()
	req2 := defaultGenerateCodeRequest()

	if err := unmarshalJSONMerge(req.InferRequest, req1); err != nil {
		return &jsResponse{Error: err.Error()}
	}
	if err := unmarshalJSONMerge(req.GenerateCodeRequest, req2); err != nil {
		return &jsResponse{Error: err.Error()}
	}
	req1.ExampleInputs = []*spb.InputFile{
		{Spec: &spb.InputFile_InputContent{InputContent: []byte(req.CSV)}},
	}

	resp1, err := s.Infer(ctx, req1)
	if err != nil {
		return &jsResponse{Error: err.Error()}
	}

	req2.Mapping = resp1.BestMappingCandidate.GetTopLevelMapping()
	resp2, err := s.GenerateCode(ctx, req2)
	if err != nil {
		return &jsResponse{
			InferResponse: resp1,
			Error:         err.Error(),
		}
	}

	effectiveRequest := func() *jsRequest {
		req1 := proto.Clone(req1).(*spb.InferRequest)
		req2 := proto.Clone(req2).(*spb.GenerateCodeRequest)
		// Zero out the values that are filled in automatically above.
		req1.ExampleInputs = nil
		req2.Mapping = nil
		return &jsRequest{
			CSV:                 req.CSV,
			InferRequest:        prototext.Format(req1),
			GenerateCodeRequest: prototext.Format(req2),
		}
	}

	return &jsResponse{
		InferResponse:    resp1,
		CodeGenResponse:  resp2,
		EffectiveRequest: effectiveRequest(),
	}
}

func registerJSEntryPoints(s spb.XToProtoServiceServer) error {
	js.Global().Call("xtoproto-service-available", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) != 3 {
			panic(fmt.Errorf("bad number of arguments"))
		}
		req := &jsRequest{
			InferRequest:        args[0].String(),
			GenerateCodeRequest: args[1].String(),
			CSV:                 args[2].String(),
		}

		resp := handleJSRequest(s, req)
		respJSON, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			panic(fmt.Errorf("JSON encoding error: %v", err))
		}
		return string(respJSON)
	}))
	return nil
}
