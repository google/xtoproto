// Package ntriples parses the RDF triples formatted according to the W3C N-Triples format.
//
// See https://www.w3.org/TR/n-triples/ for the specification.
package ntriples

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/xtoproto/rdf/iri"
)

// An IRI (Internationalized Resource Identifier) within an RDF graph is a
// Unicode string [UNICODE] that conforms to the syntax defined in RFC 3987
// [RFC3987].
//
// The underlying type of IRI is string, allowing convient const declaration.
type IRI = iri.IRI

const (
	// LangString is the type of a literal string that has a language annotation.
	// https://www.w3.org/TR/2014/REC-rdf11-concepts-20140225/#dfn-literal.
	LangString IRI = " http://www.w3.org/1999/02/22-rdf-syntax-ns#langString"

	// XMLSchemaString is the type of a literal string with no language
	// annotation.
	//
	// Please note that concrete syntaxes may support simple literals consisting
	// of only a lexical form without any datatype IRI or language tag. Simple
	// literals are syntactic sugar for abstract syntax literals with the datatype
	// IRI http://www.w3.org/2001/XMLSchema#string. Similarly, most concrete
	// syntaxes represent language-tagged strings without the datatype IRI because
	// it always equals http://www.w3.org/1999/02/22-rdf-syntax-ns#langString.
	XMLSchemaString IRI = "http://www.w3.org/2001/XMLSchema#string"
)

// BlankNodeID specifies a blank node.
//
// RDF blank nodes in N-Triples are expressed as _: followed by a blank node
// label which is a series of name characters. The characters in the label are
// built upon PN_CHARS_BASE, liberalized as follows:
//
// The characters _ and [0-9] may appear anywhere in a blank node label. The
// character . may appear anywhere except the first or last character. The
// characters -, U+00B7, U+0300 to U+036F and U+203F to U+2040 are permitted
// anywhere except the first character.
type BlankNodeID string

// Triple is an RDF triple.
type Triple struct {
	subject   *Subject
	predicate IRI
	object    *Object
}

// NewTriple returns a new Triple composed of the provided subject, predicate,
// and object.
func NewTriple(s *Subject, predicate IRI, o *Object) *Triple {
	return &Triple{s, predicate, o}
}

// Subject returns the subject of the triple.
func (t *Triple) Subject() *Subject { return t.subject }

// Predicate returns the predicate of the triple.
func (t *Triple) Predicate() IRI { return t.predicate }

// Object returns the object of the triple.
func (t *Triple) Object() *Object { return t.object }

// String returns the triple as an N-Triple line.
func (t *Triple) String() string {
	return fmt.Sprintf("%s %s %s .", t.subject, t.predicate, t.object)
}

// Subject is either an IRI or a blank node.
//
// See the definition at https://www.w3.org/TR/2014/REC-rdf11-concepts-20140225/#section-triples.
type Subject struct {
	iri         IRI
	blankNodeID BlankNodeID
}

// NewSubjectIRI returns a new *Subject with the given IRI as its value.
func NewSubjectIRI(iri IRI) *Subject {
	return &Subject{iri: iri}
}

// NewSubjectBlankNodeID returns a new *Subject with the given blank node identifier.
func NewSubjectBlankNodeID(blankNodeID BlankNodeID) *Subject {
	return &Subject{blankNodeID: blankNodeID}
}

// IsIRI reports if the term is an IRI.
func (s *Subject) IsIRI() bool { return s.iri != "" }

// IsBlankNode reports if the term is a literal.
func (s *Subject) IsBlankNode() bool { return s.blankNodeID != "" }

// IRI returns the IRI for the term. The result is undefined if IsIRI() is false.
func (s *Subject) IRI() IRI { return s.iri }

// BlankNodeID returns the blank node id for the term. The result is undefined if IsBlankNode() is false
func (s *Subject) BlankNodeID() BlankNodeID { return s.blankNodeID }

// String returns the N-Triple formatted term.
func (s *Subject) String() string {
	if s.IsBlankNode() {
		return fmt.Sprintf("_:%s", s.blankNodeID)
	}
	return s.iri.String()
}

// Object is an IRI, a blank node, or a literal.
//
// See the definition at https://www.w3.org/TR/2014/REC-rdf11-concepts-20140225/#section-triples.
type Object struct {
	iri         IRI
	lit         Literal
	blankNodeID BlankNodeID
}

// NewObjectLiteral returns an Object comprised of the given literal.
func NewObjectLiteral(lit Literal) *Object {
	return &Object{lit: lit}
}

// NewObjectIRI returns an Object comprised of the given internationalized resource identifier.
func NewObjectIRI(iri IRI) *Object {
	return &Object{iri: iri}
}

// NewObjectBlankNodeID returns an Object comprised of the given blank node identifier.
func NewObjectBlankNodeID(blankNodeID BlankNodeID) *Object {
	return &Object{blankNodeID: blankNodeID}
}

// NewObjectFromSubject returns an Object with the same identity as the provided
// subject.
func NewObjectFromSubject(s *Subject) *Object {
	if s.IsBlankNode() {
		return NewObjectBlankNodeID(s.BlankNodeID())
	}
	return NewObjectIRI(s.IRI())
}

// IsIRI reports if the term is an IRI.
func (o *Object) IsIRI() bool { return o.iri != "" }

// IsLiteral reports if the term is a literal.
func (o *Object) IsLiteral() bool { return !o.IsIRI() && !o.IsBlankNode() }

// IsBlankNode reports if the term is a literal.
func (o *Object) IsBlankNode() bool { return o.blankNodeID != "" }

// IRI returns the IRI for the term. The result is undefined if IsIRI() is false.
func (o *Object) IRI() IRI { return o.iri }

// BlankNodeID returns the blank node id for the term. The result is undefined if IsBlankNode() is false
func (o *Object) BlankNodeID() BlankNodeID { return o.blankNodeID }

// Literal returns the literal for the term. The result is undefined if IsLiteral() is false
func (o *Object) Literal() Literal { return o.lit }

// String returns the N-Triple formatted term.
func (o *Object) String() string {
	if o.IsBlankNode() {
		return fmt.Sprintf("_:%s", o.blankNodeID)
	}
	if o.IsIRI() {
		return fmt.Sprintf("<%s>", string(o.iri))
	}
	return LiteralString(o.lit)
}

// Literal is interface for an object that can act as an RDF literal.
type Literal interface {
	// LexicalForm returns the unicode lexical form of the literal per
	// https://www.w3.org/TR/2014/REC-rdf11-concepts-20140225/#dfn-literal.
	LexicalForm() string

	// Datatype returns an IRI identifying a datatype that determines how the
	// lexical form maps to a literal value.
	Datatype() IRI

	// LanguageTag returns a non-empty language tag as defined by [BCP47] if and
	// only if the datatype IRI is LangString. The language tag must be
	// well-formed according to section 2.2.9 of [BCP47].
	LanguageTag() string
}

// NewLiteral returns a new object that fulfills the Literal interface.
func NewLiteral(lexicalForm string, datatype IRI, langTag string) Literal {
	return &genericLiteral{lexicalForm, datatype, langTag}
}

type genericLiteral struct {
	lexicalForm string
	datatype    IRI
	langTag     string
}

// LexicalForm returns the unicode lexical form of the literal per
// https://www.w3.org/TR/2014/REC-rdf11-concepts-20140225/#dfn-literal.
func (gl *genericLiteral) LexicalForm() string {
	return gl.lexicalForm
}

// Datatype returns an IRI identifying a datatype that determines how the
// lexical form maps to a literal value.
func (gl *genericLiteral) Datatype() IRI {
	return gl.datatype
}

// LanguageTag returns a non-empty language tag as defined by [BCP47] if and
// only if the datatype IRI is LangString. The language tag must be
// well-formed according to section 2.2.9 of [BCP47].
func (gl *genericLiteral) LanguageTag() string {
	return gl.langTag
}

// LiteralString returns the lexical form of a Literal. This is the literal form
// that occurs in an NTriples statement, not a value that should be used as the
// string value f the literal.
func LiteralString(l Literal) string {
	lexForm := l.LexicalForm()
	if x, err := canonicalizeString(lexForm); err != nil {
		panic(err)
	} else {
		lexForm = x
	}
	quotedString := fmt.Sprintf("%q", lexForm)
	// TODO(reddaly): Obey "Production of Terminals section" of https://www.w3.org/TR/n-triples/#grammar-production-LANGTAG.
	if lang := l.LanguageTag(); lang != "" {
		return fmt.Sprintf("%s@%s", quotedString, lang)
	}
	if datatype := l.Datatype(); datatype != "" {
		return fmt.Sprintf("%s^^%s", quotedString, datatype)
	}
	return quotedString
}

// LiteralsEqual repots if two literals are identical, including the language tag.
func LiteralsEqual(a, b Literal) bool {
	return IRIsEqual(a.Datatype(), b.Datatype()) && a.LexicalForm() == b.LexicalForm() && a.LanguageTag() == b.LanguageTag()
}

// IRIsEqual reports if two IRIs are identical. This is the same as a == b.
func IRIsEqual(a, b IRI) bool {
	return a == b
}

// SubjectsEqual reports if two terms are identical.
func SubjectsEqual(a, b *Subject) bool {
	if a.IsIRI() && b.IsIRI() {
		return IRIsEqual(a.iri, b.iri)
	}
	if a.IsBlankNode() && b.IsBlankNode() {
		return a.blankNodeID == b.blankNodeID
	}
	return false
}

// ObjectsEqual reports if two terms are identical.
func ObjectsEqual(a, b *Object) bool {
	if a.IsIRI() && b.IsIRI() {
		return IRIsEqual(a.iri, b.iri)
	}
	if a.IsBlankNode() && b.IsBlankNode() {
		return a.blankNodeID == b.blankNodeID
	}
	if a.IsLiteral() && b.IsLiteral() {
		return LiteralsEqual(a.lit, b.lit)
	}
	return false
}

// TriplesEqual reports if two triples have equal subjects, predicates, and objects.
func TriplesEqual(a, b *Triple) bool {
	return SubjectsEqual(a.subject, b.subject) && ObjectsEqual(a.object, b.object) && IRIsEqual(a.predicate, b.predicate)
}

// triplePartsEqual reports if two triples have equal subjects, predicates, and objects.
func triplePartsEqual(a, b *Triple) (bool, bool, bool) {
	return SubjectsEqual(a.subject, b.subject), IRIsEqual(a.predicate, b.predicate), ObjectsEqual(a.object, b.object)
}

var termSep = regexp.MustCompile(`\s+`)

// ParseLine returns either a triple, a comment, or a parsing error.
func ParseLine(line string) (*Triple, *Comment, error) {
	if len(line) == 0 {
		return nil, nil, fmt.Errorf("invalid zero-length line")
	}
	if line[0] == '#' {
		return nil, &Comment{line[1:]}, nil
	}
	sub, rest, err := parseSubject(line)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse subject of line: %w", err)
	}
	pred, rest, err := parseIRI(strings.TrimLeft(rest, " \t"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse subject of line: %w", err)
	}
	obj, rest, err := parseObject(strings.TrimLeft(rest, " \t"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse object of line: %w", err)
	}
	rest = strings.Trim(rest, " \t")
	if rest != "." {
		return nil, nil, fmt.Errorf("last term must be `.`, got %q", rest)
	}

	return NewTriple(sub, pred, obj), nil, nil
}

// ParseLines returns all of the triples parsed from the input lines. Comments
// are excluded.
func ParseLines(lines []string) ([]*Triple, error) {
	var triples []*Triple
	for i, line := range lines {
		t, _, err := ParseLine(line)
		if err != nil {
			return nil, fmt.Errorf("%d: %w", i+1, err)
		}
		if t == nil {
			continue
		}
		triples = append(triples, t)
	}
	return triples, nil
}

func parseSubject(input string) (*Subject, string, error) {
	if len(input) == 0 {
		return nil, "", fmt.Errorf("invalid empty subject")
	}
	switch input[0] {
	case '_':
		got, rest, err := parseBlankNode(input)
		if err != nil {
			return nil, "", err
		}
		return NewSubjectBlankNodeID(got), rest, nil
	case '<':
		got, rest, err := parseIRI(input)
		if err != nil {
			return nil, "", err
		}
		return NewSubjectIRI(got), rest, nil
	default:
		return nil, "", fmt.Errorf("invalid subject: %q", input)
	}
}

func parseObject(input string) (*Object, string, error) {
	if len(input) == 0 {
		return nil, "", fmt.Errorf("invalid empty subject")
	}
	switch input[0] {
	case '_':
		got, rest, err := parseBlankNode(input)
		if err != nil {
			return nil, "", err
		}
		return NewObjectBlankNodeID(got), rest, nil
	case '<':
		got, rest, err := parseIRI(input)
		if err != nil {
			return nil, "", err
		}
		return NewObjectIRI(got), rest, nil
	case '"':
		got, rest, err := ParseLiteral(input)
		if err != nil {
			return nil, "", err
		}
		return NewObjectLiteral(got), rest, nil
	default:
		return nil, "", fmt.Errorf("invalid subject: %q", input)
	}
}

const (
	hex         = `[0-9A-Fa-f]`
	uchar       = `(?:\\u` + hex + hex + hex + hex + `|\\U` + hex + hex + hex + hex + hex + hex + hex + hex + `)`
	echar       = `\\[tbnrf"']`
	pnCharsBase = `A-Za-z\x{00C0}-\x{00D6}\x{00D8}-\x{00F6}\x{00F8}-\x{02FF}\x{0370}-\x{037D}\x{037F}-\x{1FFF}\x{200C}-\x{200D}\x{2070}-\x{218F}\x{2C00}-\x{2FEF}\x{3001}-\x{D7FF}\x{F900}-\x{FDCF}\x{FDF0}-\x{FFFD}\x{1000}0\#xEFFFF`
	pnCharsU    = pnCharsBase + `_\:`
	pnChars     = pnCharsU + `\-0-9\x{00B7}\x{0300}-\x{036F}\x{203F}-\x{2040}`

	stringLiteralQuote = `"((?:[^\x22\x5C\x{A}\x{D}]|` + echar + `|` + uchar + `)*)"`
	iriref             = `<((?:[^\x00-\x20<>"{}|^` + "`" + `\\]|` + uchar + `)*)>`
	langTag            = `@([a-zA-Z]+(?:\-[a-zA-Z0-9]+)?)`
)

var (
	irirefRegexp = regexp.MustCompile("^" + iriref)

	// 	BLANK_NODE_LABEL	::=	'_:' (PN_CHARS_U | [0-9]) ((PN_CHARS | '.')* PN_CHARS)?
	blankNodeLabel = regexp.MustCompile(`^_\:[` + pnCharsU + `0-9][` + pnChars + `\.]*[` + pnChars + `]?`)
	literalRegexp  = regexp.MustCompile(`^` + stringLiteralQuote + `(?:(?:\^\^` + iriref + `)|` + langTag + `)?`)
)

func parseIRI(input string) (IRI, string, error) {
	match := irirefRegexp.FindStringIndex(input)
	if len(match) == 0 {
		return "", "", fmt.Errorf("invalid IRIREF: %q", input)
	}
	// TODO(reddaly): Canonicalize IRI (e.g., make unicode hex all one case).
	return IRI(input[1 : match[1]-1]), input[match[1]:], nil
}

func parseBlankNode(input string) (BlankNodeID, string, error) {
	match := blankNodeLabel.FindStringIndex(input)
	if len(match) == 0 {
		return "", "", fmt.Errorf("invalid blank node: %q does not match %s", input, blankNodeLabel)
	}
	// TODO(reddaly): Canonicalize blank node by parsing the string.
	return BlankNodeID(input[2:match[1]]), input[match[1]:], nil
}

// ParseLiteral returns an RDF literal parsed from a string.
func ParseLiteral(input string) (Literal, string, error) {
	parts := literalRegexp.FindStringSubmatch(input)
	if parts == nil {
		return nil, "", fmt.Errorf("invalid literal: %s", input)
	}
	rest := input[len(parts[0]):]
	lexicalForm, iri, lang := parts[1], parts[2], parts[3]
	if iri == "" && lang == "" {
		return NewLiteral(lexicalForm, XMLSchemaString, ""), rest, nil
	} else if lang != "" {
		return NewLiteral(lexicalForm, LangString, lang), rest, nil
	}
	return NewLiteral(lexicalForm, IRI(iri), lang), rest, nil
}

// Comment is a line comment in an N-Tuples file.
type Comment struct {
	contents string
}

// Contents returns the comment literal without the leading '#' character.
func (c *Comment) Contents() string {
	return c.contents
}

// Line returns the comment literal with the leading '#' character.
func (c *Comment) Line() string {
	return fmt.Sprintf("#%s", c.contents)
}

var stringLiteralCharCapture = regexp.MustCompile(`(?:([^\x22\x5C\x{A}\x{D}])|(` + echar + `)|(` + uchar + `))*`)

func canonicalizeString(in string) (string, error) {
	canonical := strings.Builder{}
	for _, charMatch := range stringLiteralCharCapture.FindAllStringSubmatch(in, -1) {
		if len(charMatch[1]) != 0 {
			canonical.WriteString(charMatch[1])
		} else if len(charMatch[2]) != 0 {
			canonical.WriteString(charMatch[2])
		} else if len(charMatch[3]) != 0 {
			code, err := strconv.ParseInt(charMatch[3][2:], 16, 64)
			if err != nil {
				return "", err
			}
			canonical.WriteRune(rune(code))
		}
	}
	return canonical.String(), nil
}
