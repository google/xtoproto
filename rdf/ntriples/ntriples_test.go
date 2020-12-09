package ntriples

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTriplesEqual(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name string
		a, b *Triple
		want bool
	}{
		{
			"_:a <https://github.com/google/xtoproto/testing#prop1> _:b",
			NewTriple(NewSubjectBlankNodeID("a"), "https://github.com/google/xtoproto/testing#prop1", NewObjectBlankNodeID("b")),
			NewTriple(NewSubjectBlankNodeID("a"), "https://github.com/google/xtoproto/testing#prop1", NewObjectBlankNodeID("b")),
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TriplesEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("TriplesEqual(\n  %s\n  %s\n ) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		t    *Triple
		want string
	}{
		{
			NewTriple(NewSubjectBlankNodeID("a"), "https://github.com/google/xtoproto/testing#prop1", NewObjectBlankNodeID("b")),
			"_:a <https://github.com/google/xtoproto/testing#prop1> _:b .",
		},
		{
			NewTriple(NewSubjectBlankNodeID("a"), "https://github.com/google/xtoproto/testing#prop1", NewObjectLiteral(NewLiteral("xyz", LangString, "en-us"))),
			`_:a <https://github.com/google/xtoproto/testing#prop1> "xyz"@en-us .`,
		},
		{
			NewTriple(
				NewSubjectBlankNodeID("a"),
				"https://github.com/google/xtoproto/testing#prop1",
				NewObjectLiteral(NewLiteral("tab\tnewline\ncarriage return\rslash\\q1'q2\"", LangString, "en-us"))),
			`_:a <https://github.com/google/xtoproto/testing#prop1> "tab` + "\t" + `newline\ncarriage return\rslash\\q1'q2\""@en-us .`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.t.String(); got != tt.want {
				t.Errorf("\ngot  %s\n  !=\nwant %s", got, tt.want)
			}
		})
	}
}

func TestParseIRIRef(t *testing.T) {
	tests := []struct {
		t        *Triple
		in       string
		want     IRI
		wantRest string
		wantErr  bool
	}{
		{
			in:       `<https://github.com/google/xtoproto/testing#prop1>`,
			want:     "https://github.com/google/xtoproto/testing#prop1",
			wantRest: "",
		},
		{
			in:       `<https://github.com/google/xtoproto/testing#prop1> kasdfklsd!!`,
			want:     "https://github.com/google/xtoproto/testing#prop1",
			wantRest: " kasdfklsd!!",
		},
		{
			in:   `<http://r&#xE9;sum&#xE9;.example.org>`,
			want: `http://r&#xE9;sum&#xE9;.example.org`,
		},
		{
			in:   `<http://\u00E9.example.org>`,
			want: `http://é.example.org`,
		},
		{
			// Preserve percent encoding when it is necessary.
			in:   `<http://\u00E9.example.org/dog%20house/%B5>`,
			want: `http://é.example.org/dog%20house/µ`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, gotRest, err := parseIRIRef(tt.in)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("got err %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("parseIRIRef(%q)\n  got  %s\n  !=\n  want %s", tt.in, got, tt.want)
			}
			if gotRest != tt.wantRest {
				t.Errorf("parseIRIRef(%q)\n  got rest  %s\n  !=\n  want rest %s", tt.in, gotRest, tt.wantRest)
			}
		})
	}
}

func TestCanonicalizeIRILiteral(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{
			in:   `https://github.com/google/xtoproto/testing#prop1`,
			want: "https://github.com/google/xtoproto/testing#prop1",
		},
		{
			in:   `#prop1`,
			want: "#prop1",
		},
		{
			in:   `http://r&#xE9;sum&#xE9;.example.org`,
			want: "http://r&#xE9;sum&#xE9;.example.org",
		},
		{
			in:   `http://r&#xE9;sum&#xE9;.example.org`,
			want: "http://r&#xE9;sum&#xE9;.example.org",
		},
		{
			in:   `http://example.org/#André`,
			want: `http://example.org/#André`,
		},
		{
			in:   `http://example.org/#Andr\u00E9`,
			want: `http://example.org/#André`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := canonicalizeIRILiteral(tt.in)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Errorf("got err %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("canonicalizeIRILiteral(%q)\n  got  %s\n  !=\n  want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		t *Triple
	}{
		{NewTriple(NewSubjectBlankNodeID("a"), "https://github.com/google/xtoproto/testing#prop1", NewObjectBlankNodeID("b"))},
		{NewTriple(NewSubjectBlankNodeID("a"), "https://github.com/google/xtoproto/testing#prop1", NewObjectLiteral(NewLiteral("xyz", LangString, "en-us")))},
	}
	for _, tt := range tests {
		t.Run(tt.t.String(), func(t *testing.T) {
			got, _, err := ParseLine(tt.t.String())
			if err != nil {
				t.Fatalf("ParseLine got unexpected error: %v", err)
			}
			subEql, predEql, objEql := triplePartsEqual(got, tt.t)
			if !subEql || !predEql || !objEql {
				t.Errorf("ParseLine(%q)\ngot  %s\n  !=\nwant %s\n (%v %v %v)", tt.t.String(), got, tt.t, subEql, predEql, objEql)
			}
		})
	}
}

func TestRegexps(t *testing.T) {
	tests := []struct {
		re    *regexp.Regexp
		input string
		want  []string
	}{
		{regexp.MustCompile(iriref), `<http://blah#x>`, []string{"<http://blah#x>", "http://blah#x"}},
		{regexp.MustCompile(stringLiteralQuote), `"abc"`, []string{`"abc"`, `abc`}},
		{regexp.MustCompile(langTag), `@en-US`, []string{"@en-US", "en-US"}},
		{literalRegexp, `"abcd"`, []string{`"abcd"`, `abcd`, "", ""}},
		{literalRegexp, `"abcd"^^<http://blah>`, []string{`"abcd"^^<http://blah>`, "abcd", "http://blah", ""}},
		{literalRegexp, `"x"@en`, []string{`"x"@en`, "x", "", "en"}},
		{literalRegexp, `"x"@en-us`, []string{`"x"@en-us`, "x", "", "en-us"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := tt.re.FindStringSubmatch(tt.input)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("%q matched against %s got unexpected diff:\ngot  %s\nwant %s\ndiff: %s", tt.input, tt.re, mustFormatJSON(got), mustFormatJSON(tt.want), diff)
			}
		})
	}
}

var literalCmpOpt = cmp.Transformer("literal", func(lit Literal) map[string]string {
	return map[string]string{
		"LexicalForm": lit.LexicalForm(),
		"Datatype":    lit.Datatype().String(),
		"LangTag":     lit.LanguageTag(),
	}
})

func TestParseLiteral(t *testing.T) {
	tests := []struct {
		input    string
		want     Literal
		wantRest string
		wantErr  bool
	}{
		{`"x"@en`, NewLiteral("x", LangString, "en"), "", false},
		{`"x"@en-US`, NewLiteral("x", LangString, "en-US"), "", false},
		{`"x"^^<https://google>`, NewLiteral("x", "https://google", ""), "", false},
		{`"x"`, NewLiteral("x", XMLSchemaString, ""), "", false},
		{`"x@en`, nil, "", true},
		{`"x\u0124 Ĥ" <blah>`, NewLiteral("xĤ Ĥ" /* Ĥ = \u0124 */, XMLSchemaString, ""), " <blah>", false},
		{`"tab\t"`, NewLiteral("tab\t", XMLSchemaString, ""), "", false},
		{`"line feed\n"`, NewLiteral("line feed\n", XMLSchemaString, ""), "", false},
		{`"carriage return\n"`, NewLiteral("carriage return\n", XMLSchemaString, ""), "", false},
		{`"slash\\"`, NewLiteral("slash\\", XMLSchemaString, ""), "", false},
		{`"tab\tnewline\ncarriage return\rslash\\" <blah>`, NewLiteral("tab\tnewline\ncarriage return\rslash\\", XMLSchemaString, ""), " <blah>", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, rest, err := ParseLiteral(tt.input)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("got err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(tt.want, got, literalCmpOpt); diff != "" {
				t.Errorf("ParseLiteral(%q) created unexpected diff (-want, +got): %s", tt.input, diff)
			}
			if rest != tt.wantRest {
				t.Errorf("ParseLiteral(%q) returned _, %q, want _, %q", tt.input, rest, tt.wantRest)
			}
		})
	}
}

func TestParseLineRoundTrip(t *testing.T) {
	tests := []struct {
		input, want string
		wantErr     bool
	}{
		{
			`<http://object#name> <http://www.w3.org/2000/01/rdf-schema#subclassOf> "some value"@en .`,
			`<http://object#name> <http://www.w3.org/2000/01/rdf-schema#subclassOf> "some value"@en .`,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotTriple, _, err := ParseLine(tt.input)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("got err = %v, wantErr = %v", err, tt.want)
			}
			if tt.wantErr {
				return
			}
			if gotTriple == nil {
				t.Fatalf("expected triple, got comment")
			}
			gotStr := gotTriple.String()
			if diff := cmp.Diff(tt.want, gotStr); diff != "" {
				t.Errorf("ParseLine(%q).String() got %q; unexpected diff (-want, +got):\n  %s", tt.input, gotStr, diff)
			}
		})
	}
}

func mustFormatJSON(obj interface{}) string {
	got, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return string(got)
}
