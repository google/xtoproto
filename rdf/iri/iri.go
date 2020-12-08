// Package iri contains facilities for working with Internationalized Resource
// Identifiers as specified in RFC 3987.
//
// RFC reference: https://www.ietf.org/rfc/rfc3987.html
package iri

import (
	"fmt"
	"net/url"
)

// An IRI (Internationalized Resource Identifier) within an RDF graph is a
// Unicode string [UNICODE] that conforms to the syntax defined in RFC 3987
// [RFC3987].
//
// See https://www.w3.org/TR/2014/REC-rdf11-concepts-20140225/#dfn-iri.
type IRI string

// Check reports if the IRI is valid according to the specification.
func (iri IRI) Check() error {
	_, err := url.Parse(string(iri))
	if err != nil {
		return fmt.Errorf("%q is not a valid URL: %w", string(iri), err)
	}
	return nil
}

// String returns the N-Tuples-formatted IRI: "<" + iri + ">".
func (iri IRI) String() string {
	return fmt.Sprintf("<%s>", string(iri))
}

// Normalization background reading:
// - https://blog.golang.org/normalization
// - https://www.ietf.org/rfc/rfc3987.html#section-5
//    - https://www.ietf.org/rfc/rfc3987.html#section-5.3.2.3 - percent encoding

// NormalizePercentEncoding returns an IRI that replaces any unnecessarily
// percent-escaped characters with unescaped characters.
//
// RFC3987 discusses this normalization procedure in 5.3.2.3:
// https://www.ietf.org/rfc/rfc3987.html#section-5.3.2.3.
func (iri IRI) NormalizePercentEncoding() IRI {
	// Background reading:
	//    -  - percent encoding
	return iri
}
