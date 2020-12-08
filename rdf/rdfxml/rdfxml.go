package rdfxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"regexp"
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
	RDFParseType   ntriples.IRI = RDF + "parseType"
	RDFProperty    ntriples.IRI = RDF + "Property"
	RDFDatatype    ntriples.IRI = RDF + "datatype"
	RDFResource    ntriples.IRI = RDF + "resource"
	RDFSubject     ntriples.IRI = RDF + "subject"
	RDFPredicate   ntriples.IRI = RDF + "predicate"
	RDFObject      ntriples.IRI = RDF + "object"
	RDFStatement   ntriples.IRI = RDF + "Statement"

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
	// TODO(reddaly): Add position within xml to error output.
	p.baseURIStack = append(p.baseURIStack, baseURI)
}

func (p *Parser) popBaseURI() {
	p.baseURIStack = p.baseURIStack[0 : len(p.baseURIStack)-1]
}

func (p *Parser) pushLang(lang string) {
	// TODO(reddaly): Add position within xml to error output.
	p.langStack = append(p.langStack, lang)
}

func (p *Parser) popLang() {
	p.langStack = p.langStack[0 : len(p.langStack)-1]
}

func (p *Parser) baseURI() ntriples.IRI {
	if len(p.baseURIStack) == 0 {
		return ""
	}
	return p.baseURIStack[0]
}

func (p *Parser) language() string {
	if len(p.langStack) == 0 {
		return "" // according to 6.1, language is set to the empty string.
	}
	return p.langStack[0]
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

// handleGenericStartElem updaters the parser's state based on generic
// attributes of the start element, like "xml:base" and returns a function that
// handles the closing of the element.
func (p *Parser) handleGenericStartElem(elem xml.StartElement) (func() error, error) {
	var base, lang *string
	for _, attr := range elem.Attr {
		if attr.Name.Space == xmlNS && attr.Name.Local == "base" {
			base = &attr.Value
		}
		if attr.Name.Space == xmlNS && attr.Name.Local == "lang" {
			lang = &attr.Value
		}
	}
	if base != nil {
		if _, err := url.Parse(*base); err != nil {
			return nil, p.errorf("xml:base value invalid: %w", err)
		}
		p.pushBaseURI(ntriples.IRI(*base))
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

// readNodeElem decodes triples from an XML token stream.
func readNodeElem(p *Parser, start xml.StartElement) (*ntriples.Subject, error) {
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

	for _, attr := range rdfAttributes(start.Attr) {
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
func readPropertyElem(p *Parser, liCounter *int, subject *ntriples.Subject, propElem xml.StartElement) error {
	handleCloseElem, err := p.handleGenericStartElem(propElem)
	if err != nil {
		return err
	}
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
			literal, err := parseLiteral(p, string(contents))
			if err != nil {
				return err
			}
			triple := ntriples.NewTriple(subject, elemURI, ntriples.NewObjectLiteral(literal))
			ind, err := p.tripleCallback(triple)
			if err != nil {
				return p.errorf("triple callback error: %w", err)
			}
			if ind == Stop {
				return nil
			}
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

	} else if childInfo.isEmptyProperty {
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
				return true
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

				triple2 := ntriples.NewTriple(subject, elemURI, ntriples.NewObjectFromSubject(r))
				ind, err = p.tripleCallback(triple2)
				if err != nil {
					return p.errorf("triple callback error: %w", err)
				}
				if ind == Stop {
					return nil
				}
			}
			// TODO(reddaly): Reification.
		}
	} else {
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
		// The end element was read
	}
	if err := handleCloseElem(); err != nil {
		return err
	}

	glog.Infof("got StartElement for subject %s: %q/%q %v /%v", subject, t.Name.Space, t.Name.Local, t, childInfo)
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
func resolve(p *Parser, s string) (ntriples.IRI, error) {
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

func parseLiteral(p *Parser, s string) (ntriples.Literal, error) {
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

func readElementContents(p *Parser) ([]byte, error) {
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

func readWhitespaceUntilEndElem(p *Parser) error {
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

func isXMLAttr(name xml.Name) bool {
	return name.Space != xmlNS && (name.Space == "" && startWithXMLRegexp.MatchString(name.Local))
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
