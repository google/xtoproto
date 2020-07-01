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

package protocp

import (
	"context"

	"google.golang.org/protobuf/proto"
)

// compositeRecordWriter dispatches to a set of other record writers.
type compositeRecordWriter struct {
	writers []MessageWriter
}

// NewCompositeRecordWriter returns a MessageWriter that dispatches to several other record
// writers.
func NewCompositeRecordWriter(writers ...MessageWriter) MessageWriter {
	return &compositeRecordWriter{writers}
}

func (cw *compositeRecordWriter) addRow(ctx context.Context, message proto.Message) error {
	for _, w := range cw.writers {
		if err := w.addRow(ctx, message); err != nil {
			return err
		}
	}
	return nil
}

func (cw *compositeRecordWriter) finalize(ctx context.Context) error {
	var firstErr error
	for _, w := range cw.writers {
		if err := w.finalize(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
