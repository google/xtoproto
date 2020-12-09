package rdfxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/google/xtoproto/rdf/iri"
	"github.com/google/xtoproto/rdf/ntriples"
)

const (
	// RDF if the base IRI for RDF terms.
	RDF ntriples.IRI = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
)

// IRIs for terms defined in https://www.w3.org/TR/rdf-schema/.
const (
	RDFRoot        ntriples.IRI = RDF + "RDF"
	RDFDescription ntriples.IRI = RDF + "Description"
	RDFLangString  ntriples.IRI = RDF + "langString"
	RDFHTML        ntriples.IRI = RDF + "HTML"
	RDFXMLLiteral  ntriples.IRI = RDF + "XMLLiteral"
	RDFXMLProperty ntriples.IRI = RDF + "XMLProperty"
	RDFID          ntriples.IRI = RDF + "ID"
	RDFNodeID      ntriples.IRI = RDF + "nodeID"
	RDFLI          ntriples.IRI = RDF + "li"
	RDFAbout       ntriples.IRI = RDF + "about"
	RDFType        ntriples.IRI = RDF + "type"
	RDFParseType   ntriples.IRI = RDF + "parseType"
	RDFProperty    ntriples.IRI = RDF + "Property"
	RDFDatatype    ntriples.IRI = RDF + "datatype"
	RDFResource    ntriples.IRI = RDF + "resource"
	RDFSubject     ntriples.IRI = RDF + "subject"
	RDFPredicate   ntriples.IRI = RDF + "predicate"
	RDFObject      ntriples.IRI = RDF + "object"
	RDFStatement   ntriples.IRI = RDF + "Statement"
	RDFNil         ntriples.IRI = RDF + "nil"
	RDFRest        ntriples.IRI = RDF + "rest"
	RDFFirst       ntriples.IRI = RDF + "first"

	xmlNS string = "http://www.w3.org/XML/1998/namespace"
)

// IterationDecision indicates whether an iterating function should stop
// iterating.
type IterationDecision int

// Valid IterationDecision values.
const (
	Stop IterationDecision = iota
	Continue
)

// readTriples decodes triples from an XML token stream.
func readTriples(p *Parser, receiver func(t *ntriples.Triple) (IterationDecision, error)) error {
	p.tripleCallback = receiver
	return readRootOrNodeElem(p)
}

func readRootOrNodeElem(p *Parser) error {
	started, finished := false, false
	for {
		tok, err := p.reader.Token()
		if err == io.EOF {
			if finished {
				return nil
			}
			return fmt.Errorf("unexpected end of file while parsing RDF xml")
		}
		if err != nil {
			return fmt.Errorf("XML error: %v", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if started {
				return p.errorf("internal error? got second start element at root level %+v", t)
			}
			started = true

			if xmlNameToIRI(t.Name) == RDFRoot {
				_, err := p.handleGenericStartElem(t)
				if err != nil {
					return err
				}
				return readNodeElemsAndEndRootElement(p)
			}
			// Otherwise, treat this like reading a regular node element.
			// See https://www.w3.org/TR/rdf-syntax-grammar/#nodeElement.
			_, err := readNodeElem(p, t)
			return err
		case xml.EndElement:
			if xmlNameToIRI(t.Name) != RDFRoot {
				return p.errorf("internal parsing error - only expect an end element when handlng rdf:Root; probably forgot to consume an EndElement elsewhere: %+v", t)
			}
			finished = true
			// end elements should be consumed
		case xml.CharData:
			str := string(t)
			if strings.TrimSpace(str) != "" {
				return p.errorf("unexpected non-whitespace element text %q", str)
			}
		case xml.Comment: // ignore
		case xml.ProcInst: // ignore
		case xml.Directive: // ignore
		}
	}
}

func readNodeElemsAndEndRootElement(p *Parser) error {
	_, err := readNodeElemsAndEndElement(p, RDFRoot)
	return err
}

func readNodeElemsAndEndElement(p *Parser, wantEnd ntriples.IRI) ([]*ntriples.Subject, error) {
	var subjects []*ntriples.Subject
	for {
		tok, err := p.reader.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("unexpected end of file while parsing rdf:Root")
		}
		if err != nil {
			return nil, fmt.Errorf("XML error while parsing rdf:Root: %v", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			// Otherwise, treat this like reading a regular node element.
			// See https://www.w3.org/TR/rdf-syntax-grammar/#nodeElement.
			sub, err := readNodeElem(p, t)
			if err != nil {
				return nil, err
			}
			subjects = append(subjects, sub)
		case xml.EndElement:
			if got := xmlNameToIRI(t.Name); got != wantEnd {
				return nil, p.errorf("internal parsing error - got end element %s, want %q; probably forgot to consume an EndElement elsewhere: %+v", got, wantEnd, t)
			}
			return subjects, nil
		case xml.CharData:
			str := string(t)
			if strings.TrimSpace(str) != "" {
				return nil, p.errorf("unexpected non-whitespace element text %q", str)
			}
		case xml.Comment: // ignore
		case xml.ProcInst: // ignore
		case xml.Directive: // ignore
		}
	}
}

// ParserOptions configures a Parser.
type ParserOptions struct {
	// BaseURL to use for the document being parsed.
	BaseURL ntriples.IRI
}

// Parser is an RDF/XML document parser.
type Parser struct {
	reader             xml.TokenReader
	tripleCallback     func(t *ntriples.Triple) (IterationDecision, error)
	baseURIStack       []ntriples.IRI
	langStack          []string
	nextBlankNodeID    int
	observedBlankNodes map[ntriples.BlankNodeID]bool
}

// NewParser returns a new RDF/XML parser that reads from the given xml document
// tr.
func NewParser(tr xml.TokenReader, opts *ParserOptions) *Parser {
	p := &Parser{tr, nil, nil, nil, 0, map[ntriples.BlankNodeID]bool{}}
	if opts != nil && opts.BaseURL != "" {
		p.pushBaseURI(opts.BaseURL)
	}
	return p
}

// ReadAllTriples returns a slice of triples based on all the XML contents of
// the input reader.
func (p *Parser) ReadAllTriples() ([]*ntriples.Triple, error) {
	all := []*ntriples.Triple{}
	err := readTriples(p, func(t *ntriples.Triple) (IterationDecision, error) {
		all = append(all, t)
		return Continue, nil
	})
	return all, err
}

func (p *Parser) errorf(format string, args ...interface{}) error {
	// TODO(reddaly): Add position within xml to error output.
	return fmt.Errorf(format, args...)
}

func (p *Parser) pushBaseURI(baseURI ntriples.IRI) {
	// https://www.ietf.org/rfc/rfc3986.html#section-5.2
	//
	// RDF/XML spec: An empty same document reference "" resolves against the URI
	// part of the base URI; any fragment part is ignored. See Uniform Resource
	// Identifiers (URI) [RFC3986].
	p.baseURIStack = append(p.baseURIStack, removeFragment(baseURI))
}

func removeFragment(baseURI ntriples.IRI) ntriples.IRI {
	s := string(baseURI)
	if idx := strings.IndexRune(s, '#'); idx != -1 {
		return ntriples.IRI(s[0:idx])
	}
	return baseURI
}

func (p *Parser) popBaseURI() {
	p.baseURIStack = p.baseURIStack[0 : len(p.baseURIStack)-1]
}

func (p *Parser) pushLang(lang string) {
	p.langStack = append(p.langStack, lang)
}

func (p *Parser) popLang() {
	p.langStack = p.langStack[0 : len(p.langStack)-1]
}

func (p *Parser) baseURI() ntriples.IRI {
	if len(p.baseURIStack) == 0 {
		return ""
	}
	return p.baseURIStack[len(p.baseURIStack)-1]
}

func (p *Parser) language() string {
	if len(p.langStack) == 0 {
		return "" // according to 6.1, language is set to the empty string.
	}
	return p.langStack[len(p.langStack)-1]
}

func (p *Parser) generateBlankNodeID() ntriples.BlankNodeID {
	for {
		p.nextBlankNodeID++
		candidate := ntriples.BlankNodeID(fmt.Sprintf("gen-%d", p.nextBlankNodeID))
		if p.observedBlankNodes[candidate] {
			continue
		}
		return candidate
	}
}

func (p *Parser) generateBlankNodeIDWithHint(hint string) ntriples.BlankNodeID {
	for {
		p.nextBlankNodeID++
		candidate := ntriples.BlankNodeID(fmt.Sprintf("gen-%d-%s", p.nextBlankNodeID, hint))
		if p.observedBlankNodes[candidate] {
			continue
		}
		return candidate
	}
}

// handleGenericStartElem updaters the parser's state based on generic
// attributes of the start element, like "xml:base" and returns a function that
// handles the closing of the element.
func (p *Parser) handleGenericStartElem(elem xml.StartElement) (func() error, error) {
	var base, lang *string
	for _, attr := range elem.Attr {
		if attr.Name.Space == xmlNS && attr.Name.Local == "base" {
			v := attr.Value
			base = &v
		}
		if attr.Name.Space == xmlNS && attr.Name.Local == "lang" {
			v := attr.Value
			lang = &v
		}
	}
	if base != nil {
		baseIRI, err := parseIRI(*base)
		if err != nil {
			return nil, p.errorf("xml:base value invalid: %w", err)
		}
		p.pushBaseURI(baseIRI)
	}
	if lang != nil {
		p.pushLang(*lang)
	}
	return func() error {
		if base != nil {
			p.popBaseURI()
		}
		if lang != nil {
			p.popLang()
		}
		return nil
	}, nil
}

func readNodeElem(p *Parser, start xml.StartElement) (*ntriples.Subject, error) {
	return readNodeElemUsingSubject(p, start, nil)
}

// readNodeElemUsingSubject decodes triples from an XML token stream.
//
// The forceSubject argument may be nil, in which case the subject is
// determined based on the element and its attributes.
func readNodeElemUsingSubject(p *Parser, start xml.StartElement, forcedSubject *ntriples.Subject) (*ntriples.Subject, error) {
	handleCloseElem, err := p.handleGenericStartElem(start)
	if err != nil {
		return nil, err
	}
	// From the spec:
	//
	// If there is an attribute a with a.URI == rdf:ID, then e.subject :=
	// uri(identifier := resolve(e, concat("#", a.string-value))).
	//
	// If there is an attribute a with a.URI == rdf:nodeID, then e.subject :=
	// bnodeid(identifier:=a.string-value).
	//
	// If there is an attribute a with a.URI == rdf:about then e.subject :=
	// uri(identifier := resolve(e, a.string-value)).
	//
	// If e.subject is empty, then e.subject := bnodeid(identifier :=
	// generated-blank-node-id()).

	subject := forcedSubject

	attrs := rdfAttributes(start.Attr)
	for _, attr := range attrs {
		switch xmlNameToIRI(attr.Name) {
		case RDFID:
			subIRI, err := resolve(p, "#"+attr.Value)
			if err != nil {
				return nil, p.errorf("bad IRI for rdf:Description's rdf:ID attribute: %w", err)
			}
			newSubject := ntriples.NewSubjectIRI(subIRI)
			if subject != nil {
				return nil, p.errorf("ambiguous subject: %s and %s", subject, newSubject)
			}
			subject = ntriples.NewSubjectIRI(subIRI)
		case RDFNodeID:
			blankID, err := parseBlankNodeID(attr.Value)
			if err != nil {
				return nil, p.errorf("bad IRI for rdf:Description's rdf:NodeID attribute: %w", err)
			}
			newSubject := ntriples.NewSubjectBlankNodeID(blankID)
			if subject != nil {
				return nil, p.errorf("ambiguous subject: %s and %s", subject, newSubject)
			}
			subject = newSubject
		case RDFAbout:
			subIRI, err := resolve(p, attr.Value)
			if err != nil {
				return nil, p.errorf("bad IRI for rdf:Description's rdf:about attribute: %w", err)
			}
			newSubject := ntriples.NewSubjectIRI(subIRI)
			if subject != nil {
				return nil, p.errorf("ambiguous subject: %s and %s", subject, newSubject)
			}
			subject = newSubject
		default:
			// ignore
		}
	}
	if subject == nil {
		subject = ntriples.NewSubjectBlankNodeID(p.generateBlankNodeID())
	}

	eURI := xmlNameToIRI(start.Name)
	if forcedSubject != nil {
		eURI = RDFDescription
	}
	// If e.URI != rdf:Description then the following statement is added to the graph:
	//
	// e.subject.string-value <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> e.URI-string-value .
	if eURI != RDFDescription {
		triple := ntriples.NewTriple(subject, RDFType, ntriples.NewObjectIRI(eURI))
		ind, err := p.tripleCallback(triple)
		if err != nil {
			return nil, p.errorf("triple callback error: %w", err)
		}
		if ind == Stop {
			return subject, nil
		}
	}

	// For each attribute a matching propertyAttr (and not rdf:type), the Unicode
	// string a.string-value should be in Normal Form C [NFC], o :=
	// literal(literal-value := a.string-value, literal-language := e.language)
	// and the following statement is added to the graph:
	for _, attr := range attrs {
		switch attrIRI := xmlNameToIRI(attr.Name); attrIRI {
		case RDFID, RDFNodeID, RDFAbout:
			// already handled above
		case RDFType:
			objectIRI, err := resolve(p, attr.Value)
			if err != nil {
				return nil, p.errorf("error parsing rdf:type attribute value: %w", err)
			}
			triple := ntriples.NewTriple(subject, RDFType, ntriples.NewObjectIRI(objectIRI))
			ind, err := p.tripleCallback(triple)
			if err != nil {
				return nil, p.errorf("triple callback error: %w", err)
			}
			if ind == Stop {
				return subject, nil
			}
		default:
			objectLiteral, err := parseLiteral(p, attr.Value)
			if err != nil {
				return nil, p.errorf("bad literal for attribute %+v: %w", attr, err)
			}
			triple := ntriples.NewTriple(subject, attrIRI, ntriples.NewObjectLiteral(objectLiteral))
			ind, err := p.tripleCallback(triple)
			if err != nil {
				return nil, p.errorf("triple callback error: %w", err)
			}
			if ind == Stop {
				return subject, nil
			}
		}
	}
	if err := readPropertyElems(p, start.Name, subject); err != nil {
		return nil, err
	}
	if err := handleCloseElem(); err != nil {
		return nil, err
	}
	return subject, nil
}

// readPropertyElems parses the property elements of a nodeElem per
// https://www.w3.org/TR/rdf-syntax-grammar/#propertyElt.
func readPropertyElems(p *Parser, startElemName xml.Name, subject *ntriples.Subject) error {
	liCounter := 1
	for {
		tok, err := p.reader.Token()
		if err == io.EOF {
			return p.errorf("unexpected EOF while reading property elem")
		}
		if err != nil {
			return p.errorf("XML error: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			err := readPropertyElem(p, &liCounter, subject, t)
			if err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name != startElemName {
				return p.errorf("unexpected end element while reading property elements of %+v (%s): got unexpected end element %+v", startElemName, subject, t.Name)
			}
			return nil
		case xml.CharData:
		case xml.Comment: // ignore
		case xml.ProcInst: // ignore
		case xml.Directive: // ignore aside from the usual handling
		}
	}
}

// readPropertyElems parses the property elements of a nodeElem per
// https://www.w3.org/TR/rdf-syntax-grammar/#propertyElt.
func readPropertyElemInternal(p *Parser, liCounter *int, subject *ntriples.Subject, propElem xml.StartElement) error {
	elemURI := xmlNameToIRI(propElem.Name)
	t := propElem
	switch elemURI {
	case RDFLI:
		elemURI = ntriples.IRI(fmt.Sprintf("%s_%d", string(RDF), *liCounter))
		*liCounter++
	}
	parseTypeAttr := findAttr(t, RDFParseType)
	if parseTypeAttr != nil {
		switch parseTypeAttr.Value {
		case "Resource":
			// Section 7.2.18: https://www.w3.org/TR/rdf-syntax-grammar/#parseTypeResourcePropertyElt
			// n := bnodeid(identifier := generated-blank-node-id()).
			// Add the following statement to the graph:
			// e.parent.subject.string-value e.URI-string-value n.string-value .
			n := p.generateBlankNodeID()
			triple := ntriples.NewTriple(subject, elemURI, ntriples.NewObjectBlankNodeID(n))
			ind, err := p.tripleCallback(triple)
			if err != nil {
				return p.errorf("triple callback error: %w", err)
			}
			if ind == Stop {
				return nil
			}
			if attr := findAttr(propElem, RDFID); attr != nil {
				i, err := resolve(p, "#"+attr.Value)
				if err != nil {
					return err
				}
				ind, err := emitReifiedTriple(p, triple, ntriples.NewSubjectIRI(i))
				if err != nil {
					return p.errorf("triple callback error: %w", err)
				}
				if ind == Stop {
					return nil
				}
			}
			var badAttributes []string
			for _, attr := range rdfAttributes(propElem.Attr) {
				attrIRI := xmlNameToIRI(attr.Name)
				if attrIRI == RDFID || attrIRI == RDFParseType {
					continue
				}
				badAttributes = append(badAttributes, attr.Name.Local)
			}
			if len(badAttributes) != 0 {
				sort.Strings(badAttributes)
				return fmt.Errorf("unexpected attributes for rdf:parseType='Resource': only allowed to have an rdf:id and rdf:parseType attribute, got %d extras: %s", len(badAttributes), strings.Join(badAttributes, ","))
			}
			propElemNoAttrs := propElem.Copy()
			propElemNoAttrs.Attr = nil

			if _, err := readNodeElemUsingSubject(p, propElemNoAttrs, ntriples.NewSubjectBlankNodeID(n)); err != nil {
				return p.errorf("failed while processing parseTypeResourcePropertyElt: of %s: %w", subject, err)
			}

			return nil
		case "Collection":
			return readSequencePropertyElem(p, subject, propElem, elemURI)
		case "Literal":
			fallthrough
		default: // unrecognized parseType is treated as a "Literal" per spec
			contents, err := readElementContents(p)
			if err != nil {
				return p.errorf("failed to get literal contents from parseType=%q: %w", parseTypeAttr.Value, err)
			}
			literal := ntriples.NewLiteral(string(contents), RDFXMLLiteral, "")
			triple := ntriples.NewTriple(subject, elemURI, ntriples.NewObjectLiteral(literal))
			ind, err := p.tripleCallback(triple)
			if err != nil {
				return p.errorf("triple callback error: %w", err)
			}
			if ind == Stop {
				return nil
			}
			if attr := findAttr(propElem, RDFID); attr != nil {
				i, err := resolve(p, "#"+attr.Value)
				if err != nil {
					return err
				}
				ind, err := emitReifiedTriple(p, triple, ntriples.NewSubjectIRI(i))
				if err != nil {
					return p.errorf("triple callback error: %w", err)
				}
				if ind == Stop {
					return nil
				}
			}
		}
		return nil
	}
	childInfo, err := readNextChildOfPropertyElement(p)
	if err != nil {
		return err
	}
	if childInfo.nodeElemStart != nil {
		// 7.2.15: Production resourcePropertyElt
		valueSubject, err := readNodeElem(p, *childInfo.nodeElemStart)
		if err != nil {
			return p.errorf("failed to parse resourcePropertyElt %s property of %s: %w", xmlNameToIRI(childInfo.nodeElemStart.Name), subject, err)
		}
		if err := readWhitespaceUntilEndElem(p, &propElem); err != nil {
			return p.errorf("failed to parse resourcePropertyElt %s property of %s: %w", xmlNameToIRI(childInfo.nodeElemStart.Name), subject, err)
		}
		// e.parent.subject.string-value e.URI-string-value n.subject.string-value .

		triple := ntriples.NewTriple(subject, elemURI, ntriples.NewObjectFromSubject(valueSubject))
		ind, err := p.tripleCallback(triple)
		if err != nil {
			return p.errorf("triple callback error: %w", err)
		}
		if ind == Stop {
			return nil
		}

		// 7.2.15: If the rdf:ID attribute a is given, the above statement is
		// reified with i := uri(identifier := resolve(e, concat("#",
		// a.string-value))) using the reification rules in section 7.3 and
		// e.subject := i.
		if attr := findAttr(propElem, RDFID); attr != nil {
			i, err := resolve(p, "#"+attr.Value)
			if err != nil {
				return err
			}
			ind, err := emitReifiedTriple(p, triple, ntriples.NewSubjectIRI(i))
			if err != nil {
				return p.errorf("triple callback error: %w", err)
			}
			if ind == Stop {
				return nil
			}
		}
		return nil
	}

	if childInfo.isEmptyProperty {
		// 7.2.21 Production emptyPropertyElt
		attrs := rdfAttributes(propElem.Attr)
		if len(attrs) == 0 || (len(attrs) == 1 && xmlNameToIRI(attrs[0].Name) == RDFID) {
			// If there are no attributes or only the optional rdf:ID attribute i then
			// o := literal(literal-value:="", literal-language := e.language) and the
			// following statement is added to the graph:
			// e.parent.subject.string-value e.URI-string-value o.string-value .
			literal, err := parseLiteral(p, "")
			if err != nil {
				return p.errorf("error emitting empty literal property: %w", err)
			}
			triple := ntriples.NewTriple(subject, elemURI, ntriples.NewObjectLiteral(literal))
			ind, err := p.tripleCallback(triple)
			if err != nil {
				return p.errorf("triple callback error: %w", err)
			}
			if ind == Stop {
				return nil
			}
			// 7.2.21: f rdf:ID attribute i is given, the above statement is reified
			// with uri(identifier := resolve(e, concat("#", i.string-value))) using
			// the reification rules in section 7.3.
			if attr := findAttr(propElem, RDFID); attr != nil {
				i, err := resolve(p, "#"+attr.Value)
				if err != nil {
					return err
				}
				ind, err := emitReifiedTriple(p, triple, ntriples.NewSubjectIRI(i))
				if err != nil {
					return p.errorf("triple callback error: %w", err)
				}
				if ind == Stop {
					return nil
				}
			}
		} else {
			// If rdf:resource attribute i is present, then r := uri(identifier := resolve(e, i.string-value))
			// If rdf:nodeID attribute i is present, then r := bnodeid(identifier := i.string-value)
			// If neither, r := bnodeid(identifier := generated-blank-node-id())
			r, err := parseEmptyPropertyResource(p, attrs)
			if err != nil {
				return err
			}
			for _, propAttr := range filterAttrs(attrs, func(a xml.Attr) bool {
				return isPropertyAttr(a.Name)
			}) {
				var triple1 *ntriples.Triple
				propAsIRI := xmlNameToIRI(propAttr.Name)
				if propAsIRI == RDFType {
					typeIRI, err := resolve(p, propAttr.Value)
					if err != nil {
						return err
					}
					triple1 = ntriples.NewTriple(r, RDFType, ntriples.NewObjectIRI(typeIRI))
				} else {
					lit, err := parseLiteral(p, propAttr.Value)
					if err != nil {
						return err
					}
					triple1 = ntriples.NewTriple(r, propAsIRI, ntriples.NewObjectLiteral(lit))
				}
				ind, err := p.tripleCallback(triple1)
				if err != nil {
					return p.errorf("triple callback error: %w", err)
				}
				if ind == Stop {
					return nil
				}
			}

			triple := ntriples.NewTriple(subject, elemURI, ntriples.NewObjectFromSubject(r))
			ind, err := p.tripleCallback(triple)
			if err != nil {
				return p.errorf("triple callback error: %w", err)
			}
			if ind == Stop {
				return nil
			}
			// Reification. 7.2.21: If rdf:ID attribute i is given, the
			// above statement is reified with uri(identifier := resolve(e,
			// concat("#", i.string-value))) using the reification rules in section
			// 7.3.

			if attr := findAttr(propElem, RDFID); attr != nil {
				i, err := resolve(p, "#"+attr.Value)
				if err != nil {
					return err
				}
				ind, err := emitReifiedTriple(p, triple, ntriples.NewSubjectIRI(i))
				if err != nil {
					return p.errorf("triple callback error: %w", err)
				}
				if ind == Stop {
					return nil
				}
			}
		}
		return nil
	}

	// 7.2.16 Production literalPropertyElt
	var literal ntriples.Literal
	if dtAttr := findAttr(propElem, RDFDatatype); dtAttr != nil {
		datatype, err := parseIRI(dtAttr.Value)
		if err != nil {
			return err
		}
		literal = ntriples.NewLiteral(childInfo.text, datatype, "")
	} else {
		literal, err = parseLiteral(p, childInfo.text)
		if err != nil {
			return err
		}
	}
	triple := ntriples.NewTriple(subject, elemURI, ntriples.NewObjectLiteral(literal))
	ind, err := p.tripleCallback(triple)
	if err != nil {
		return p.errorf("triple callback error: %w", err)
	}
	if ind == Stop {
		return nil
	}
	if attr := findAttr(propElem, RDFID); attr != nil {
		i, err := resolve(p, "#"+attr.Value)
		if err != nil {
			return err
		}
		ind, err := emitReifiedTriple(p, triple, ntriples.NewSubjectIRI(i))
		if err != nil {
			return p.errorf("triple callback error: %w", err)
		}
		if ind == Stop {
			return nil
		}
	}
	// The end element was read

	return nil
}

func readSequencePropertyElem(p *Parser, subject *ntriples.Subject, propElem xml.StartElement, propElemURI ntriples.IRI) error {
	// Section 7.2.19
	emittedSubjects, err := readNodeElemsAndEndElement(p, xmlNameToIRI(propElem.Name))
	if err != nil {
		return err
	}
	if len(emittedSubjects) == 0 {
		triple := ntriples.NewTriple(subject, propElemURI, ntriples.NewObjectIRI(RDFNil))
		ind, err := p.tripleCallback(triple)
		if err != nil || ind == Stop {
			return err
		}
		if attr := findAttr(propElem, RDFID); attr != nil {
			i, err := resolve(p, "#"+attr.Value)
			if err != nil {
				return err
			}
			if ind, err := emitReifiedTriple(p, triple, ntriples.NewSubjectIRI(i)); err != nil || ind == Stop {
				return err
			}
		}
		return nil
	}
	seqID := p.generateBlankNodeIDWithHint("first-in-seq")
	{
		triple := ntriples.NewTriple(subject, propElemURI, ntriples.NewObjectBlankNodeID(seqID))
		ind, err := p.tripleCallback(triple)
		if err != nil || ind == Stop {
			return err
		}
		if attr := findAttr(propElem, RDFID); attr != nil {
			i, err := resolve(p, "#"+attr.Value)
			if err != nil {
				return err
			}
			if ind, err := emitReifiedTriple(p, triple, ntriples.NewSubjectIRI(i)); err != nil || ind == Stop {
				return err
			}
		}
	}

	isFirst := true
	for i, emittedSub := range emittedSubjects {
		nextSeqID := p.generateBlankNodeIDWithHint(fmt.Sprintf("cons-%d", i))
		if !isFirst {
			triple := ntriples.NewTriple(ntriples.NewSubjectBlankNodeID(seqID), RDFRest, ntriples.NewObjectBlankNodeID(nextSeqID))
			ind, err := p.tripleCallback(triple)
			if err != nil || ind == Stop {
				return err
			}
		}
		isFirst = false
		seqID = nextSeqID
		triple := ntriples.NewTriple(ntriples.NewSubjectBlankNodeID(seqID), RDFFirst, ntriples.NewObjectFromSubject(emittedSub))
		ind, err := p.tripleCallback(triple)
		if err != nil || ind == Stop {
			return err
		}
	}

	triple := ntriples.NewTriple(ntriples.NewSubjectBlankNodeID(seqID), RDFRest, ntriples.NewObjectIRI(RDFNil))
	_, err = p.tripleCallback(triple)
	return err
}

// readPropertyElems parses the property elements of a nodeElem per
// https://www.w3.org/TR/rdf-syntax-grammar/#propertyElt.
func readPropertyElem(p *Parser, liCounter *int, subject *ntriples.Subject, propElem xml.StartElement) error {
	handleCloseElem, err := p.handleGenericStartElem(propElem)
	if err != nil {
		return err
	}
	if err := readPropertyElemInternal(p, liCounter, subject, propElem); err != nil {
		return err
	}
	if err := handleCloseElem(); err != nil {
		return err
	}

	return nil
}

// readPropertyElems parses the property elements of a nodeElem per
// https://www.w3.org/TR/rdf-syntax-grammar/#propertyElt.
func emitReifiedTriple(p *Parser, tr *ntriples.Triple, tripleID *ntriples.Subject) (IterationDecision, error) {
	reifiedTriples := []*ntriples.Triple{
		// r.string-value <http://www.w3.org/1999/02/22-rdf-syntax-ns#subject> s .
		ntriples.NewTriple(tripleID, RDFSubject, ntriples.NewObjectFromSubject(tr.Subject())),
		// r.string-value <http://www.w3.org/1999/02/22-rdf-syntax-ns#predicate> p .
		ntriples.NewTriple(tripleID, RDFPredicate, ntriples.NewObjectIRI(tr.Predicate())),
		// r.string-value <http://www.w3.org/1999/02/22-rdf-syntax-ns#object> o .
		ntriples.NewTriple(tripleID, RDFObject, tr.Object()),
		// r.string-value <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <http://www.w3.org/1999/02/22-rdf-syntax-ns#Statement> .
		ntriples.NewTriple(tripleID, RDFType, ntriples.NewObjectIRI(RDFStatement)),
	}
	for _, x := range reifiedTriples {
		ind, err := p.tripleCallback(x)
		if err != nil {
			return Stop, p.errorf("triple callback error: %w", err)
		}
		if ind == Stop {
			return ind, nil
		}
	}
	return Continue, nil
}

func parseEmptyPropertyResource(p *Parser, attrs []xml.Attr) (*ntriples.Subject, error) {
	// From the spec:
	//
	// If rdf:resource attribute i is present, then r := uri(identifier := resolve(e, i.string-value))
	// If rdf:nodeID attribute i is present, then r := bnodeid(identifier := i.string-value)
	// If neither, r := bnodeid(identifier := generated-blank-node-id())
	var subject *ntriples.Subject

	for _, attr := range rdfAttributes(attrs) {
		switch xmlNameToIRI(attr.Name) {
		case RDFResource:
			subIRI, err := resolve(p, attr.Value)
			if err != nil {
				return nil, p.errorf("bad IRI for rdf:Description's rdf:ID attribute: %w", err)
			}
			newSubject := ntriples.NewSubjectIRI(subIRI)
			if subject != nil {
				return nil, p.errorf("ambiguous subject: %s and %s", subject, newSubject)
			}
			subject = ntriples.NewSubjectIRI(subIRI)
		case RDFNodeID:
			blankID, err := parseBlankNodeID(attr.Value)
			if err != nil {
				return nil, p.errorf("bad IRI for rdf:Description's rdf:NodeID attribute: %w", err)
			}
			newSubject := ntriples.NewSubjectBlankNodeID(blankID)
			if subject != nil {
				return nil, p.errorf("ambiguous subject: %s and %s", subject, newSubject)
			}
			subject = newSubject
		default:
			// ignore
		}
	}
	if subject == nil {
		subject = ntriples.NewSubjectBlankNodeID(p.generateBlankNodeID())
	}
	return subject, nil
}

// propertyElemParseInfo is used to hold the results of parsing a
// propertyElem's
type propertyElemParseInfo struct {
	// set to the first StartElement if one is encountered. Indicates that the
	// property elem is a resourcePropertyElt.
	nodeElemStart *xml.StartElement
	// concatenation of all CharData within the element.
	text string
	// true for the production emptyPropertyElt
	isEmptyProperty bool
}

func readNextChildOfPropertyElement(p *Parser) (*propertyElemParseInfo, error) {
	textBuilder := &strings.Builder{}
	for {
		tok, err := p.reader.Token()
		if err != nil {
			return nil, p.errorf("XML read error: %w", err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			text := textBuilder.String()
			return &propertyElemParseInfo{
				text:            text,
				isEmptyProperty: len(text) == 0,
			}, nil

		case xml.CharData:
			if _, err := textBuilder.Write([]byte(t)); err != nil {
				return nil, err
			}
		case xml.StartElement:
			text := textBuilder.String()
			if strings.TrimSpace(text) != "" {
				return nil, p.errorf("got start element %s for property and also non-whitespace element text %q", xmlNameToIRI(t.Name), text)
			}
			return &propertyElemParseInfo{
				nodeElemStart: &t,
			}, nil
		default:
			// Skip comments and other types
		}
	}
}

// resolve returns "a string created by interpreting string s as a relative IRI
// to the ·base-uri· accessor of 6.1.2 Element Event e as defined in Section 5.3
// Resolving URIs. The resulting string represents an IRI."
//
// The spec also says:
//
// RDF/XML supports XML Base [XMLBASE] which defines a ·base-uri· accessor for
// each ·root event· and ·element event·. Relative IRIs are resolved into IRIs
// according to the algorithm specified in [XMLBASE] (and RFC 2396). These
// specifications do not specify an algorithm for resolving a fragment
// identifier alone, such as #foo, or the empty string "" into an IRI. In
// RDF/XML, a fragment identifier is transformed into an IRI by appending the
// fragment identifier to the in-scope base URI. The empty string is transformed
// into an IRI by substituting the in-scope base URI.
func resolve(p *Parser, s string) (ntriples.IRI, error) {
	glog.Infof("resolve(%q) with baseURI %s and stack %+v", s, p.baseURI(), p.baseURIStack)
	if s == "" {
		return p.baseURI(), nil
	}
	sIRI, err := iri.Parse(s)
	if err != nil {
		return "", fmt.Errorf("error parsing %q as IRI: %w", s, err)
	}
	return p.baseURI().ResolveReference(sIRI).NormalizePercentEncoding(), nil
}

func parseLiteral(p *Parser, s string) (ntriples.Literal, error) {
	lang := p.language()
	if lang == "" {
		return ntriples.NewLiteral(s, ntriples.XMLSchemaString, ""), nil
	}
	return ntriples.NewLiteral(s, "", lang), nil
}

func parseIRI(s string) (ntriples.IRI, error) {
	unnormalized, err := iri.Parse(s)
	if err != nil {
		return "", err
	}
	return unnormalized.NormalizePercentEncoding(), err
}

func parseBlankNodeID(s string) (ntriples.BlankNodeID, error) {
	// TODO(reddaly): Check blank node id syntax.
	value := ntriples.BlankNodeID(s)
	return value, nil
}

func xmlNameToIRI(n xml.Name) ntriples.IRI {
	return ntriples.IRI(n.Space + n.Local)
}

func findAttr(elem xml.StartElement, iri ntriples.IRI) *xml.Attr {
	for i, attr := range elem.Attr {
		if xmlNameToIRI(attr.Name) == iri {
			return &elem.Attr[i]
		}
	}
	return nil
}

// readElementContents reads the text contents of the element until the
// end element is found.
func readElementContents(p *Parser) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := xml.NewEncoder(buf)
	depth := 0
	for {
		tok, err := p.reader.Token()
		if err != nil {
			return nil, p.errorf("XML read error: %w", err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			if depth == 0 {
				if err := enc.Flush(); err != nil {
					return nil, p.errorf("XML printing error: %w", err)
				}
				glog.Infof("readElementContents got text contents %q", string(buf.Bytes()))
				return buf.Bytes(), nil
			}
			depth--
		case xml.StartElement:
			glog.Infof("readElementContents depth %d, start %q", depth, t.Name.Local)
			depth++
		}
		if err := enc.EncodeToken(tok); err != nil {
			return nil, p.errorf("XML printing error: %w", err)
		}
	}
}

func readWhitespaceUntilEndElem(p *Parser, startElem *xml.StartElement) error {
	for {
		tok, err := p.reader.Token()
		if err != nil {
			return p.errorf("XML read error: %w", err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			if startElem != nil && t.Name != startElem.Name {
				return p.errorf("unexpected end elem %+v does not match start elem %+v", t.Name, startElem.Name)
			}
			return nil
		case xml.CharData:
			nonWS := strings.TrimSpace(string(t))
			if len(nonWS) != 0 {
				return p.errorf("expected only whitespace in contents, got %q", nonWS)
			}
		case xml.StartElement:
			return p.errorf("expected whitespace, got start of element %+v", t)
		default:
			// ignore
		}
	}
}

var startWithXMLRegexp = regexp.MustCompile(`^(?i)xml`)

// rdfAttributes returns all the attributes that should be considered as the
// attributes of an element according to the RDF/XML spec.
func rdfAttributes(attrs []xml.Attr) []xml.Attr {
	// Made from the value of element information item property [attributes] which
	// is a set of attribute information items.
	//
	// If this set contains an attribute information item xml:lang ( [namespace
	// name] property with the value "http://www.w3.org/XML/1998/namespace" and
	// [local name] property value "lang") it is removed from the set of attribute
	// information items and the ·language· accessor is set to the
	// [normalized-value] property of the attribute information item.
	//
	// All remaining reserved XML Names (see Name in XML 1.0) are now removed from
	// the set. These are, all attribute information items in the set with
	// property [prefix] beginning with xml (case independent comparison) and all
	// attribute information items with [prefix] property having no value and
	// which have [local name] beginning with xml (case independent comparison)
	// are removed. Note that the [base URI] accessor is computed by XML Base
	// before any xml:base attribute information item is deleted.
	//
	// The remaining set of attribute information items are then used to construct
	// a new set of Attribute Events which is assigned as the value of this
	// accessor.
	return filterAttrs(attrs, func(a xml.Attr) bool {
		return !isXMLAttr(a.Name)
	})
}

// filterAttrs returns all of the attributes that pass a predicate function
func filterAttrs(attrs []xml.Attr, pred func(xml.Attr) bool) []xml.Attr {
	var out []xml.Attr
	for _, a := range attrs {
		if !pred(a) {
			continue
		}
		out = append(out, a)
	}
	return out
}

var beginsWithXMLExpr = regexp.MustCompile(`^(?i)xml`)
var xmlnsRE = regexp.MustCompile(`^(?i)` + regexp.QuoteMeta(xmlNS))

func isXMLAttr(name xml.Name) bool {
	glog.Infof("isXMLAttr(%+v) = %v", name, xmlnsRE.MatchString(name.Space) || beginsWithXMLExpr.MatchString(name.Local) || beginsWithXMLExpr.MatchString(name.Space))
	return xmlnsRE.MatchString(name.Space) || beginsWithXMLExpr.MatchString(name.Local) || beginsWithXMLExpr.MatchString(name.Space)
}

func isPropertyAttr(name xml.Name) bool {
	term := xmlNameToIRI(name)
	return !coreSyntaxTerms[term] && !oldTerms[term]
}

var coreSyntaxTerms = map[ntriples.IRI]bool{
	RDFRoot:      true,
	RDFID:        true,
	RDFAbout:     true,
	RDFParseType: true,
	RDFResource:  true,
	RDFNodeID:    true,
	RDFDatatype:  true,
}

var oldTerms = map[ntriples.IRI]bool{
	RDF + "aboutEach":       true,
	RDF + "aboutEachPrefix": true,
	RDF + "bagID":           true,
}
