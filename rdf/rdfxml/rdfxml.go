package rdfxml

import (
	"encoding/xml"

	"github.com/google/xtoproto/rdf/ntriples"
)

const (
	// RDF if the base IRI for RDF terms.
	RDF ntriples.IRI = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
)

// IRIs for terms defined in https://www.w3.org/TR/rdf-schema/.
const (
	RDFLangString  ntriples.IRI = RDF + "langString"
	RDFHTML        ntriples.IRI = RDF + "HTML"
	RDFXMLLiteral  ntriples.IRI = RDF + "XMLLiteral"
	RDFXMLProperty ntriples.IRI = RDF + "XMLProperty"
	RDFType        ntriples.IRI = RDF + "type"
	RDFProperty    ntriples.IRI = RDF + "Property"
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
func ReadTriples(reader xml.TokenReader, receiver func(t *ntriples.Triple) (IterationDecision, error)) {

}
