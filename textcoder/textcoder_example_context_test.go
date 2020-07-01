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

package textcoder

import (
	"fmt"
	"reflect"
	"strings"
)

type bulletList struct {
	bullet string
	items  []interface{}
}

type noEncoderType struct{}

func ExampleContext() {

	MustRegister(
		reflect.TypeOf(""),
		func(v string) (string, error) { return v, nil },
		func(text string, dst *string) error { *dst = text; return nil })

	MustRegister(
		reflect.TypeOf(&bulletList{}),
		func(ctx *Context, v *bulletList) (string, error) {
			indent := ""
			if v, ok := ctx.Value("indent"); ok {
				indent = v.(string)
			}
			var bullets []string
			indentCtx := ctx.WithValue("indent", indent+"  ")
			for _, item := range v.items {
				itemEncoder := ctx.Registry().GetEncoder(reflect.TypeOf(item))
				var itemText string
				if itemEncoder == nil {
					itemText = fmt.Sprintf("missing encoder for %s", reflect.TypeOf(item))
				} else {
					txt, err := itemEncoder.EncodeText(indentCtx, item)
					if err != nil {
						return "", fmt.Errorf("error encoding list: %w", err)
					}
					itemText = txt
				}

				prefix := fmt.Sprintf("%s%s ", indent, v.bullet)
				if _, ok := item.(*bulletList); ok {
					prefix = indent
				}
				bullets = append(bullets, prefix+itemText)
			}
			return strings.Join(bullets, "\n"), nil
		},
		func(s string, dst **bulletList) error {
			return fmt.Errorf("decoding not supported")
		})

	v := &bulletList{
		bullet: "-",
		items: []interface{}{
			"a",
			"b",
			noEncoderType{},
			&bulletList{
				bullet: "*",
				items: []interface{}{
					"c",
					"d",
				},
			},
		},
	}
	s, _ := Marshal(v)
	fmt.Printf("%s\n", s)

	// Output:
	// - a
	// - b
	// - missing encoder for textcoder.noEncoderType
	//   * c
	//   * d
}
