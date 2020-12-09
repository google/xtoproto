// Package iri contains facilities for working with Internationalized Resource
// Identifiers as specified in RFC 3987.
//
// RFC reference: https://www.ietf.org/rfc/rfc3987.html
package iri

import (
	"regexp"
	"testing"
)

func TestIRI_NormalizePercentEncoding(t *testing.T) {
	tests := []struct {
		name string
		in   IRI
		want IRI
	}{
		{
			name: "a",
			in:   `https://github.com/google/xtoproto/testing#prop1`,
			want: "https://github.com/google/xtoproto/testing#prop1",
		},
		{
			name: "b",
			in:   `https://github.com/google/xtoproto/testing#prop1`,
			want: "https://github.com/google/xtoproto/testing#prop1",
		},
		{
			name: "c",
			in:   `http://r&#xE9;sum&#xE9;.example.org`,
			want: `http://r&#xE9;sum&#xE9;.example.org`,
		},
		{
			name: "non ascii é",
			in:   `http://é.example.org`,
			want: `http://é.example.org`,
		},
		{
			name: "Preserve percent encoding when it is necessary",
			in:   `http://é.example.org/dog%20house/%B5`,
			want: `http://é.example.org/dog%20house/µ`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.NormalizePercentEncoding(); got != tt.want {
				t.Errorf("NormalizePercentEncoding(%q) = \n  %s, want\n  %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    IRI
		wantErr bool
	}{
		{
			name: "prop1",
			in:   `https://github.com/google/xtoproto/testing#prop1`,
			want: "https://github.com/google/xtoproto/testing#prop1",
		},
		{
			name: "http://example.org/#André",
			in:   `http://example.org/#André`,
			want: `http://example.org/#André`,
		},
		{
			name:    "http://example.org/#André then some whitespace",
			in:      "http://example.org/#André then some whitespace",
			want:    ``,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.in)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("got err %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) got %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestRegExps(t *testing.T) {
	tests := []struct {
		name string
		re   *regexp.Regexp
		in   string
		want bool
	}{
		{
			name: "space is not a valid iri character",
			re:   iunreservedRE,
			in:   ` `,
			want: false,
		},
		{
			name: "þ is unreserved",
			re:   iunreservedRE,
			in:   "\u00FE",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.re.MatchString(tt.in)
			if got != tt.want {
				t.Errorf("%s.Match(%q) got %v, want %v", tt.re, tt.in, got, tt.want)
			}
		})
	}
}
