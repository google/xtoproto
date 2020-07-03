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

// Package xmlinfer attempts to infer protocol buffer definitions from a set of
// XML examples.
package xmlinfer

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jhump/protoreflect/desc/builder"
	"github.com/jhump/protoreflect/desc/protoprint"
	"github.com/stoewer/go-strcase"
)

// Infer infers a protocol buffer definition from a stream of XML tokens.
func Infer(tr xml.TokenReader, options ...Option) (*InferResult, error) {
	s := &state{tr, false}
	for _, opt := range options {
		opt.applyToState(s)
	}
	result, err := s.inferTopLevel()
	if err != nil {
		return nil, err
	}
	return result, nil
}

// InferResult holds the results of inference.
type InferResult struct {
	roots []*structCandidate
}

func (ir *InferResult) String() string {
	return fmt.Sprintf("inferResult %v", ir.roots)
}

// ProtoFile returns protobuf code inferred from the XML examples.
func (ir *InferResult) ProtoFile() (string, error) {
	p := &protoprint.Printer{
		SortElements: true,
	}
	b := builder.NewFile("output.proto").SetProto3(true)
	for _, r := range ir.roots {
		msgs, err := r.inferredMessages()
		if err != nil {
			return "", fmt.Errorf("could not infer messages of root element %s: %w", r, err)
		}
		for _, msg := range msgs {
			for i := 1; b.GetMessage(msg.GetName()) != nil; i++ {
				msg.SetName(fmt.Sprintf("%s%d", msg.GetName(), i))
			}
			b.AddMessage(msg)
		}
	}
	fDesc, err := b.Build()
	if err != nil {
		return "", err
	}
	return p.PrintProtoToString(fDesc)
}

type inferenceOptions struct {
	includeExamples bool
}

type structCandidate struct {
	name           xml.Name
	chardataField  *chardataFieldCandidate
	attrFields     []*attrFieldCandidate
	elemFields     []*elementFieldCandidate
	occurenceCount int
}

func (sc *structCandidate) String() string {
	return fmt.Sprintf(`[structCandidate %q]`, xmlToMessageName(sc.name))
}

func (sc *structCandidate) hasNoAttributesOrChildElements() bool {
	return len(sc.elemFields) == 0 && len(sc.attrFields) == 0
}

func (sc *structCandidate) inferredMessages() ([]*builder.MessageBuilder, error) {
	if sc.hasNoAttributesOrChildElements() {
		return nil, nil
	}
	var elemFieldNames []string
	for _, ef := range sc.elemFields {
		elemFieldNames = append(elemFieldNames, xmlToMessageName(ef.sc.name))
	}

	b := builder.NewMessage(xmlToMessageName(sc.name))
	b.SetComments(builder.Comments{
		LeadingComment: fmt.Sprintf(" %d attrFields, %d elemFields: %s, based on %d examples",
			len(sc.attrFields), len(sc.elemFields), strings.Join(elemFieldNames, ", "), sc.occurenceCount),
	})
	for _, attr := range sc.attrFields {
		attrField, err := attr.fieldBuilder()
		if err != nil {
			return nil, err
		}
		b.AddField(attrField)
	}

	all := []*builder.MessageBuilder{b}

	for _, ef := range sc.elemFields {
		fieldDesc, mainChildElemMsg, otherChildElemMsgs, err := ef.fieldBuilder()
		if err != nil {
			return nil, err
		}
		b.AddField(fieldDesc)
		if mainChildElemMsg != nil {
			all = append(all, mainChildElemMsg)
		}
		all = append(all, otherChildElemMsgs...)
	}
	return all, nil
}

func (sc *structCandidate) getElement(name xml.Name) *elementFieldCandidate {
	for _, ef := range sc.elemFields {
		if ef.sc.name == name {
			return ef
		}
	}
	return nil
}

func (sc *structCandidate) getAttr(name xml.Name) *attrFieldCandidate {
	for _, attr := range sc.attrFields {
		if attr.name == name {
			return attr
		}
	}
	return nil
}

type attrFieldCandidate struct {
	name              xml.Name
	sampleValueCounts map[string]int
}

func newAttrFieldCandidate(n xml.Name) *attrFieldCandidate {
	return &attrFieldCandidate{n, make(map[string]int)}
}

func (ac *attrFieldCandidate) recordExampleValue(s string) {
	ac.sampleValueCounts[s]++
}

func (ac *attrFieldCandidate) fieldBuilder() (*builder.FieldBuilder, error) {
	fieldName := xmlToFieldName(ac.name)
	ft, err := inferFieldTypeFromExampleStrings(ac.sampleValueCounts)
	if err != nil {
		return nil, fmt.Errorf("failed to infer type for attribute %q: %w", fieldName, err)
	}
	b := builder.NewField(fieldName, ft)

	b.SetComments(builder.Comments{
		LeadingComment: topNExamplesComment(ac.sampleValueCounts),
	})

	return b, nil
}

func topNExamplesComment(m map[string]int) string {
	if len(m) == 0 {
		return "inferred type from 0 examples"
	}
	total := 0
	var strs []string
	for s, count := range m {
		strs = append(strs, s)
		total += count
	}
	sort.Slice(strs, func(i int, j int) bool {
		return m[strs[i]] > m[strs[j]]
	})

	if len(strs) > 5 {
		strs = strs[0:5]
	}
	for i, s := range strs {
		strs[i] = fmt.Sprintf("%q (%d)", s, m[s])
	}

	intro := fmt.Sprintf(
		" inferred type from %d examples, %d unique values (showing first %d):",
		total, len(m), len(strs))
	strs = append([]string{intro}, strs...)
	return strings.Join(strs, "\n - ")
}

type elementFieldCandidate struct {
	sc *structCandidate
	// cardinality is the number of appearances of the field within its parent element.
	// cardinalityCounts stores counts of cardinality based on examples of this element within
	// its parent.
	cardinalityCounts map[int]int
}

func (ef *elementFieldCandidate) recordCardinality(c int) {
	ef.cardinalityCounts[c]++
}

func (ef *elementFieldCandidate) fieldBuilder() (*builder.FieldBuilder, *builder.MessageBuilder, []*builder.MessageBuilder, error) {
	var b *builder.FieldBuilder
	var mainChild *builder.MessageBuilder
	var otherChildren []*builder.MessageBuilder
	fieldName := xmlToFieldName(ef.sc.name)
	if ef.sc.hasNoAttributesOrChildElements() {
		ft, err := inferFieldTypeFromExampleStrings(ef.sc.chardataField.sampleValueCounts)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to infer type for field %q: %w", fieldName, err)
		}
		b = builder.NewField(fieldName, ft)
		b.SetComments(builder.Comments{
			LeadingComment: topNExamplesComment(ef.sc.chardataField.sampleValueCounts),
		})
	} else {
		childStructs, err := ef.sc.inferredMessages()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error getting child structs of %q: %w", fieldName, err)
		}
		if len(childStructs) == 0 {
			return nil, nil, nil, fmt.Errorf("internal error: expected > 0 child struct definitions")
		}
		mainChild = childStructs[0]
		otherChildren = childStructs[1:]
		b = builder.NewField(fieldName, builder.FieldTypeMessage(mainChild))
		b.SetComments(builder.Comments{
			LeadingComment: fmt.Sprintf(" Cardinalities in parent: %v.", ef.cardinalityCounts),
		})
	}
	if ef.inferIsRepeated() {
		b.SetRepeated()
	}

	return b, mainChild, otherChildren, nil
}

func (ef *elementFieldCandidate) inferIsRepeated() bool {
	for card, count := range ef.cardinalityCounts {
		if card > 1 && count > 0 {
			return true
		}
	}
	return false
}

type chardataFieldCandidate struct {
	sampleValueCounts map[string]int
}

func newChardataFieldCandidate() *chardataFieldCandidate {
	return &chardataFieldCandidate{make(map[string]int)}
}

func (cdf *chardataFieldCandidate) recordExampleValue(s string) {
	cdf.sampleValueCounts[s]++
}

// IncludeExamplesOption returns an option that enables or disables showing
// example values in the generated protobuf.
func IncludeExamplesOption(include bool) Option {
	return &simpleOption{func(s *state) {
		s.includeExamples = include
	}}
}

// Option can be passed to Infer to alter inference behavior.
type Option interface {
	applyToState(s *state)
}

type simpleOption struct {
	applyFn func(*state)
}

func (so *simpleOption) applyToState(s *state) {
	so.applyFn(s)
}

type state struct {
	tr              xml.TokenReader
	includeExamples bool
}

func (s *state) inferTopLevel() (*InferResult, error) {
	ir := &InferResult{}

	for {
		tok, err := s.tr.Token()
		if err != nil {
			if err == io.EOF {
				return ir, nil
			}
			return nil, fmt.Errorf("failed to read top-level element start: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var root *structCandidate
			for _, r := range ir.roots {
				if r.name != t.Name {
					continue
				}
				root = r
				break
			}
			if root == nil {
				root = &structCandidate{
					name:          t.Name,
					chardataField: newChardataFieldCandidate(),
				}
				ir.roots = append(ir.roots, root)
			}
			if err := s.consumeElementTokens(t, root); err != nil {
				return nil, err
			}

		case xml.CharData, xml.Comment, xml.ProcInst, xml.Directive:
			// skip
		case xml.EndElement:
			return nil, fmt.Errorf("internal error: encountered EndElement %v", t)
		}
	}
}

func (s *state) consumeElementTokens(startTok xml.StartElement, sc *structCandidate) error {
	sc.occurenceCount++
	accumulatedCharData := ""
	for _, attr := range startTok.Attr {
		ac := sc.getAttr(attr.Name)
		if ac == nil {
			ac = newAttrFieldCandidate(attr.Name)
			sc.attrFields = append(sc.attrFields, ac)
		}
		ac.recordExampleValue(attr.Value)
	}
	elementCardinalities := make(map[xml.Name]int)
	updateCardinalities := func() {
		for elemName, cardinality := range elementCardinalities {
			sc.getElement(elemName).recordCardinality(cardinality)
		}
	}
	for {
		tok, err := s.tr.Token()
		if err != nil {
			return fmt.Errorf("failed parsing XML tokens within %q: %w", sc.name, err)
		}
		if tok == nil {
			// treat nil, nil as nothing happened and not EOF
			continue
		}
		switch t := tok.(type) {
		case xml.StartElement:
			elementCardinalities[t.Name]++
			field := sc.getElement(t.Name)
			if field == nil {
				field = &elementFieldCandidate{
					&structCandidate{
						name:          t.Name,
						chardataField: newChardataFieldCandidate(),
					},
					make(map[int]int),
				}
				sc.elemFields = append(sc.elemFields, field)
			}
			if err := s.consumeElementTokens(t, field.sc); err != nil {
				return err
			}
		case xml.CharData:
			accumulatedCharData += string(t)
		case xml.EndElement:
			if t.Name != sc.name {
				return fmt.Errorf("failed parsing end XML tokens of %s, got tag name %s", sc.name, t.Name)
			}
			updateCardinalities()
			sc.chardataField.recordExampleValue(accumulatedCharData)
			return nil
		case xml.Comment, xml.ProcInst, xml.Directive:
			// ignore
		}
	}
}

func xmlToMessageName(xn xml.Name) string {
	return strcase.UpperCamelCase(xn.Local)
}

func xmlToFieldName(xn xml.Name) string {
	return strcase.LowerCamelCase(xn.Local)
}
