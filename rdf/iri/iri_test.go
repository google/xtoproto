// Package iri contains facilities for working with Internationalized Resource
// Identifiers as specified in RFC 3987.
//
// RFC reference: https://www.ietf.org/rfc/rfc3987.html
package iri

import (
	"fmt"
	"regexp"
	"strings"
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
			// 181 is not a valid utf-8 octal. Check out https://www.utf8-chartable.de/.
			name: "Invalid UTF-8 code point 181",
			in:   `http://é.example.org/dog%20house/%B5`,
			want: `http://é.example.org/dog%20house/%B5`,
		},
		{
			name: "Preserve percent encoding when it is necessary",
			in:   `http://é.example.org/dog%20house/%c2%B5`,
			want: `http://é.example.org/dog%20house/µ`,
		},
		{
			name: "Example from https://github.com/google/xtoproto/issues/23",
			in:   "http://wiktionary.org/wiki/%E1%BF%AC%CF%8C%CE%B4%CE%BF%CF%82",
			want: `http://wiktionary.org/wiki/Ῥόδος`,
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
			name: "valid urn:uuid",
			in:   `urn:uuid:6c689097-8097-4421-9def-05e835f2dbb8`,
			want: `urn:uuid:6c689097-8097-4421-9def-05e835f2dbb8`,
		},
		{
			name: "valid urn:uuid:",
			in:   `urn:uuid:`,
			want: `urn:uuid:`,
		},
		{
			name: "valid a:b:c:",
			in:   `a:b:c:`,
			want: `a:b:c:`,
		},
		{
			name:    "http://example.org/#André then some whitespace",
			in:      "http://example.org/#André then some whitespace",
			want:    ``,
			wantErr: true,
		},
		{
			name:    "invalid utf-8 B5",
			in:      `http://é.example.org/dog%20house/%B5`,
			want:    ``,
			wantErr: true,
		},
		{
			name:    "invalid utf-8 B5",
			in:      `http://é.example.org/dog%20house/%20%b5`,
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

func TestResolveReference(t *testing.T) {
	tests := []struct {
		name      string
		base, ref IRI
		want      IRI
	}{
		{
			name: "prop1",
			base: `https://github.com/google/xtoproto/testing#prop1`,
			ref:  `#3`,
			want: "https://github.com/google/xtoproto/testing#3",
		},
		{
			name: "slash blah",
			base: `https://github.com/google/xtoproto/testing#prop1`,
			ref:  `/blah`,
			want: "https://github.com/blah",
		},
		{
			name: "empty ref",
			base: `https://github.com/google/xtoproto/testing#prop1`,
			ref:  ``,
			want: "https://github.com/google/xtoproto/testing#prop1",
		},
		{
			name: "different full iri",
			base: `https://github.com/google/xtoproto/testing#prop1`,
			ref:  `http://x`,
			want: "http://x",
		},
		{
			name: "blank fragment",
			base: `https://github.com/google/xtoproto/testing`,
			ref:  `#`,
			want: "https://github.com/google/xtoproto/testing#",
		},
		{
			name: "replace completely",
			base: "http://red@google.com:341",
			ref:  `http://example/q?abc=1&def=2`,
			want: `http://example/q?abc=1&def=2`,
		},
		// {
		// 	name: "An empty same document reference \"\" resolves against the URI part of the base URI; any fragment part is ignored. See Uniform Resource Identifiers (URI) [RFC3986]",
		// 	base: "http://bigbird@google.com/path#x-frag",
		// 	ref:  ``,
		// 	want: `http://bigbird@google.com/path`,
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.base.ResolveReference(tt.ref)
			if err := tt.base.Check(); err != nil {
				t.Errorf("base IRI %s is not a valid IRI: %v", tt.base, err)
			}
			if err := tt.ref.Check(); err != nil {
				t.Errorf("ref IRI %s is not a valid IRI: %v", tt.ref, err)
			}
			if got != tt.want {
				t.Errorf("ResolveReference(%s, %s) got\n  %s, want\n  %s", tt.base, tt.ref, got, tt.want)
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

func TestParts_ToIRI(t *testing.T) {
	tests := []struct {
		value IRI
	}{
		{""},
		{"example.com"},
		{"example.com:22"},
		{"example.com:22/path/to"},
		{"example.com:22/path/to?"},
		{"example.com:22/path/to?q=a"},
		{"example.com:22/path/to?q=a#b"},
		{"example.com:22/path/to?q=a#"},
		{"#"},
		{""},
		{"https://example.com"},
		{"https://example.com:22"},
		{"https://example.com:22/path/to"},
		{"https://example.com:22/path/to?q=a"},
		{"https://example.com:22/path/to?q=a#b"},
		{"https://example.com:22/path/to?q=a#"},
		{"https://#"},
		{`http://example/q?abc=1&def=2`},
	}
	for _, tt := range tests {
		t.Run(tt.value.String(), func(t *testing.T) {
			got := tt.value.parts().toIRI()
			if got != tt.value {
				t.Errorf(".parts().toIRI() roundtrip failed:\n  input:  %s\n  output: %s\n  parts:\n%s", tt.value, got, partsDescription(tt.value.parts()))
			}
		})
	}
}

func partsDescription(p *parts) string {
	s := &strings.Builder{}
	fmt.Fprintf(s, "    scheme:        %q\n", p.scheme)
	fmt.Fprintf(s, "    userInfo:      %q\n", p.userInfo)
	fmt.Fprintf(s, "    host:          %q\n", p.host)
	fmt.Fprintf(s, "    emptyAuth:     %v\n", p.emptyAuth)
	fmt.Fprintf(s, "    port:          %q\n", p.port)
	fmt.Fprintf(s, "    path:          %q\n", p.path)
	fmt.Fprintf(s, "    query:         %q\n", p.query)
	fmt.Fprintf(s, "    fragment:      %q\n", p.fragment)
	fmt.Fprintf(s, "    emptyFragment: %v\n", p.emptyFragment)
	return s.String()
}
