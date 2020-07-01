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
	"os"
	"time"

	"github.com/google/xtoproto/csvinfer"
	"github.com/google/xtoproto/recordinfer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	spb "github.com/google/xtoproto/proto/service"
)

// Infer infers a proto definition from a record-oriented data source. See the
// definition of InferRequest in service.proto for more details.
func (s *service) Infer(ctx context.Context, req *spb.InferRequest) (*spb.InferResponse, error) {

	tz := time.UTC
	if req.GetTimestampLocation() != "" {
		loc, err := time.LoadLocation(req.GetTimestampLocation())
		if err != nil {
			return nil, grpc.Errorf(codes.InvalidArgument, "invalid timestamp_location value %q: %v", req.GetTimestampLocation(), err)
		}
		tz = loc
	}
	opts := &recordinfer.Options{
		MessageName:       req.GetMessageName(),
		PackageName:       req.GetPackageName(),
		GoPackageName:     req.GetGoPackageName(),
		GoProtoImport:     req.GetGoProtoImport(),
		TimestampLocation: tz,
	}

	if got := len(req.GetExampleInputs()); got != 1 {
		return nil, grpc.Errorf(codes.InvalidArgument, "must provide exactly one entry in example_inputs, got %d", got)
	}
	input := req.GetExampleInputs()[0]
	var exampleBytes []byte
	if len(input.GetInputContent()) != 0 {
		exampleBytes = input.GetInputContent()
	} else if input.GetInputPath() != "" {
		contents, err := s.readFile(ctx, input.GetInputPath())
		if err != nil {
			return nil, fileErrToStatusErr(input.GetInputPath(), err)
		}
		exampleBytes = contents
	} else {
		return nil, grpc.Errorf(codes.InvalidArgument, "missing supported input content spec")
	}

	ip, err := csvinfer.InferProto(string(exampleBytes), opts)
	if err != nil {
		return nil, grpc.Errorf(codes.Unknown, "failed to infer proto definition: %v", err)
	}

	return &spb.InferResponse{
		BestMappingCandidate: &spb.MappingSet{
			TopLevelMapping: ip.Mapping(),
		},
	}, nil
}

func fileErrToStatusErr(path string, err error) error {
	if os.IsNotExist(err) {
		return grpc.Errorf(codes.NotFound, "specified file %q does not exist: %v", path, err)
	}
	if os.IsPermission(err) {
		return grpc.Errorf(codes.PermissionDenied, "permission defnied accessing file %q: %v", path, err)
	}
	return grpc.Errorf(codes.Unknown, "error reading or writing file %q: %v", path, err)
}
