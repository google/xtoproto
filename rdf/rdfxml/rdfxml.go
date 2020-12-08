package rdfxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/golang/glog"
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
	RDFNodeID      ntriples.IRI = RDF + "NodeID"
	RDFLI          ntriples.IRI = RDF + "li"
	RDFAbout       ntriples.IRI = RDF + "about"
	RDFType        ntriples.IRI = RDF + "type"
	RDFParseType   ntriples.IRI = RDF + "ParseType"
	RDFProperty    ntriples.IRI = RDF + "Property"

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

// ReadTriples decodes triples from an XML token stream.
func ReadTriples(reader xml.TokenReader, receiver func(t *ntriples.Triple) (IterationDecision, error)) error {
	p := &parser{reader, receiver, nil, 0}
	started, finished := false, false
	for {
		tok, err := reader.Token()
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
			if xmlNameToIRI(t.Name) == RDFRoot {
				if started {
					return p.errorf("got second RDF root element: %+v", t)
				}
				started = true
				_, err := p.handleGenericStartElem(t)
				if err != nil {
					return err
				}
				continue
			}
			if !started {
				return p.errorf("want root element rdf:RDF, got %+v", t.Name)
			}
			// See https://www.w3.org/TR/rdf-syntax-grammar/#nodeElement.
			if _, err := readNodeElem(p, t); err != nil {
				return err
			}
		case xml.EndElement:
			if xmlNameToIRI(t.Name) != RDFRoot {
				return p.errorf("internal xml parsing error - should have consumed EndElement elsewhere: %+v", t)
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

// ReadAllTriples returns a slice of triples based on all the XML contents of
// the input reader.
func ReadAllTriples(reader xml.TokenReader) ([]*ntriples.Triple, error) {
	all := []*ntriples.Triple{}
	err := ReadTriples(reader, func(t *ntriples.Triple) (IterationDecision, error) {
		all = append(all, t)
		return Continue, nil
	})
	return all, err
}

type parser struct {
	reader          xml.TokenReader
	tripleCallback  func(t *ntriples.Triple) (IterationDecision, error)
	baseURIStack    []ntriples.IRI
	nextBlankNodeID int
}

func (p *parser) errorf(format string, args ...interface{}) error {
	// TODO(reddaly): Add position within xml to error output.
	return fmt.Errorf(format, args...)
}

func (p *parser) pushBaseURI(baseURI ntriples.IRI) {
	// TODO(reddaly): Add position within xml to error output.
	p.baseURIStack = append(p.baseURIStack, baseURI)
}

func (p *parser) popBaseURI() {
	p.baseURIStack = p.baseURIStack[0 : len(p.baseURIStack)-1]
}

func (p *parser) baseURI() ntriples.IRI {
	if len(p.baseURIStack) == 0 {
		return ""
	}
	return p.baseURIStack[0]
}

func (p *parser) generateBlankNodeID() ntriples.BlankNodeID {
	p.nextBlankNodeID++
	return ntriples.BlankNodeID(fmt.Sprintf("%d", p.nextBlankNodeID))
}

// handleGenericStartElem updaters the parser's state based on generic
// attributes of the start element, like "xml:base" and returns a function that
// handles the closing of the element.
func (p *parser) handleGenericStartElem(elem xml.StartElement) (func() error, error) {
	var base *string
	for _, attr := range elem.Attr {
		if attr.Name.Space == xmlNS && attr.Name.Local == "base" {
			base = &attr.Value
		}
	}
	if base != nil {
		if _, err := url.Parse(*base); err != nil {
			return nil, p.errorf("xml:base value invalid: %w", err)
		}
		p.pushBaseURI(ntriples.IRI(*base))
		return func() error {
			p.popBaseURI()
			return nil
		}, nil
	}
	return func() error { return nil }, nil
}

// readNodeElem decodes triples from an XML token stream.
func readNodeElem(p *parser, start xml.StartElement) (*ntriples.Subject, error) {
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
	var subject *ntriples.Subject

	for _, attr := range start.Attr {
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
			subIRI, err := parseIRI(attr.Value)
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

	// If e.URI != rdf:Description then the following statement is added to the graph:
	//
	// e.subject.string-value <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> e.URI-string-value .
	if xmlNameToIRI(start.Name) != RDFDescription {
		triple := ntriples.NewTriple(subject, RDFType, ntriples.NewObjectIRI(xmlNameToIRI(start.Name)))
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
	for _, attr := range start.Attr {
		switch xmlNameToIRI(attr.Name) {
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
			triple := ntriples.NewTriple(subject, RDFType, ntriples.NewObjectLiteral(objectLiteral))
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
func readPropertyElems(p *parser, startElemName xml.Name, subject *ntriples.Subject) error {
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
				return p.errorf("unexpected end element while reading property elements of of %s %+v", subject, t.Name)
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
func readPropertyElem(p *parser, liCounter *int, subject *ntriples.Subject, propElem xml.StartElement) error {
	handleCloseElem, err := p.handleGenericStartElem(propElem)
	if err != nil {
		return err
	}
	elemURI := xmlNameToIRI(propElem.Name)
	t := propElem
	switch elemURI {
	case RDFLI:
		elemURI = ntriples.IRI(fmt.Sprintf("%s#_%d", string(RDF), *liCounter))
		*liCounter++
	}
	parseTypeAttr := findAttr(t, RDFParseType)
	if parseTypeAttr != nil {
		switch parseTypeAttr.Value {
		case "Resource":
			return p.errorf("unsupported parseType Resource")
		case "Collection":
			return p.errorf("unsupported parseType Collection")
		case "Literal":
			fallthrough
		default:
			contents, err := readElementContents(p)
			if err != nil {
				return p.errorf("failed to get literal contents from parseType=%q: %w", parseTypeAttr.Value, err)
			}
			return p.errorf("unsupported parseType %q with contents %q", parseTypeAttr.Value, string(contents))
		}
	}
	childInfo, err := readNextChildOfPropertyElement(p)
	if err != nil {
		return err
	}
	if childInfo.nodeElemStart != nil {
		valueSubject, err := readNodeElem(p, *childInfo.nodeElemStart)
		if err != nil {
			return p.errorf("failed to parse resourcePropertyElt %s property of %s: %w", xmlNameToIRI(childInfo.nodeElemStart.Name), subject, err)
		}
		if err := readWhitespaceUntilEndElem(p); err != nil {
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

		// TODO(reddaly): If the rdf:ID attribute a is given, the above statement is
		// reified with i := uri(identifier := resolve(e, concat("#",
		// a.string-value))) using the reification rules in section 7.3 and
		// e.subject := i.

	} else {
		// The end element was read
	}
	if err := handleCloseElem(); err != nil {
		return err
	}

	glog.Infof("got StartElement for subject %s: %q/%q %v /%v", subject, t.Name.Space, t.Name.Local, t, childInfo)
	return nil
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

func readNextChildOfPropertyElement(p *parser) (*propertyElemParseInfo, error) {
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
				isEmptyProperty: len(text) != 0,
			}, nil

		case xml.CharData:
			textBuilder.Write([]byte(t))
		case *xml.StartElement:
			text := textBuilder.String()
			if strings.TrimSpace(text) != "" {
				return nil, p.errorf("got start element %s for property and also non-whitespace element text %q", xmlNameToIRI(t.Name), text)
			}
			return &propertyElemParseInfo{
				nodeElemStart: t,
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
func resolve(p *parser, s string) (ntriples.IRI, error) {
	if s == "" {
		return p.baseURI(), nil
	}
	asURL, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("error parsing %q as IRI: %w", s, err)
	}
	base, err := url.Parse(string(p.baseURI()))
	if err != nil {
		return "", fmt.Errorf("error parsing base IRI %q as IRI: %w", p.baseURI(), err)
	}
	return ntriples.IRI(base.ResolveReference(asURL).String()), nil
}

func parseLiteral(p *parser, s string) (ntriples.Literal, error) {
	// TODO: Use the language of the element if available.
	return ntriples.NewLiteral(s, ntriples.XMLSchemaString, ""), nil
}

func parseIRI(s string) (ntriples.IRI, error) {
	value := ntriples.IRI(s)
	if err := checkIRI(value); err != nil {
		return "", err
	}
	return value, nil
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

func checkIRI(iri ntriples.IRI) error {
	_, err := url.Parse(string(iri))
	return err
}

func readElementContents(p *parser) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := xml.NewEncoder(buf)
	for {
		tok, err := p.reader.Token()
		if err != nil {
			return nil, p.errorf("XML read error: %w", err)
		}
		switch tok.(type) {
		case xml.EndElement:
			if err := enc.Flush(); err != nil {
				return nil, p.errorf("XML printing error: %w", err)
			}
			return buf.Bytes(), nil

		default:
			if err := enc.EncodeToken(tok); err != nil {
				return nil, p.errorf("XML printing error: %w", err)
			}
		}
	}
}

func readWhitespaceUntilEndElem(p *parser) error {
	for {
		tok, err := p.reader.Token()
		if err != nil {
			return p.errorf("XML read error: %w", err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
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
