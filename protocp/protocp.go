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

// Package protocp transforms one record-oriented format into another
// record-oriented format where the records are protocol buffers.
package protocp

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
)

const csvReaderBufferSize = 1024 * 1024 * 10

// Copier converts an input record-oriented stream to another record-oriented
// stream.
type Copier struct {
	newMessageReader func(io.Reader) (MessageReader, error)
}

// MessageReader iterates through proto messages.
type MessageReader interface {
	// ReadMessage returns the next message in the stream.
	ReadMessage() (proto.Message, error)
}

// MessageWriter is a generic interface for writing output protos to some record-oriented format.
type MessageWriter interface {
	addRow(ctx context.Context, message proto.Message) error
	finalize(ctx context.Context) error
}

// NewCopier returns a CSV converter for the given CSV path and reader/writer generators.
//
// newMessageReader returns a function for iterating through csv records as proto.Message instances.
// newMessageWriter returns a MessageWriter for writing the messages to some output sink.
func NewCopier(newMessageReader func(io.Reader) (MessageReader, error)) *Copier {
	return &Copier{
		newMessageReader,
	}
}

// Copy translates each CSV line into a proto.Message and outputs all the protos to a
// record-oriented writer.
func (cp *Copier) Copy(ctx context.Context, r io.Reader, writer MessageWriter) error {
	msgReader, err := cp.newMessageReader(r)
	if err != nil {
		return err
	}

	for i := 1; ; i++ {
		msg, err := msgReader.ReadMessage()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := writer.addRow(ctx, msg); err != nil {
			return fmt.Errorf("problem adding row %d: %w", i, err)
		}
	}
	return writer.finalize(ctx)
}

// CopyFile opens a file, translates each record of the file into a
// proto.Message, and outputs all the protos to an output MessageWriter.
func (cp *Copier) CopyFile(ctx context.Context, fs FileSystem, fileName string, writer MessageWriter) (finalErr error) {
	fio, err := fs.OpenRead(ctx, fileName)
	if err != nil {
		return err
	}
	defer func() {
		if err := fio.Close(); err != nil && finalErr == nil {
			finalErr = err
		}
	}()

	return cp.Copy(ctx, bufio.NewReaderSize(fio, csvReaderBufferSize), writer)
}

// FileSystem provides a file system abstraction in the context of protocp.
type FileSystem interface {
	OpenRead(context.Context, string) (io.ReadCloser, error)
}
