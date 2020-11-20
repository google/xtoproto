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
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.t.String(); got != tt.want {
				t.Errorf("\ngot  %s\n  !=\nwant %s", got, tt.want)
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

func TestParseLines(t *testing.T) {
	tests := []struct {
		input   string
		want    Literal
		wantErr bool
	}{
		{`"x"@en`, NewLiteral("x", LangString, "en"), false},
		{`"x"@en-US`, NewLiteral("x", LangString, "en-US"), false},
		{`"x"^^<https://google>`, NewLiteral("x", "https://google", ""), false},
		{`"x"`, NewLiteral("x", XMLSchemaString, ""), false},
		{`"x@en`, nil, true},
		{`"x\u0124"`, NewLiteral(`x\u0124`, XMLSchemaString, ""), false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLiteral(tt.input)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("got err = %v, wantErr = %v", err, tt.want)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(tt.want, got, literalCmpOpt); diff != "" {
				t.Errorf("ParseLiteral(%q) created unexpected diff (-want, +got): %s", tt.input, diff)
			}
		})
	}
}

func TestParseLiteral(t *testing.T) {
	tests := []struct {
		input   string
		want    Literal
		wantErr bool
	}{
		{`"x"@en`, NewLiteral("x", LangString, "en"), false},
		{`"x"@en-US`, NewLiteral("x", LangString, "en-US"), false},
		{`"x"^^<https://google>`, NewLiteral("x", "https://google", ""), false},
		{`"x"`, NewLiteral("x", XMLSchemaString, ""), false},
		{`"x@en`, nil, true},
		{`"x\u0124"`, NewLiteral(`x\u0124`, XMLSchemaString, ""), false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLiteral(tt.input)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("got err = %v, wantErr = %v", err, tt.want)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(tt.want, got, literalCmpOpt); diff != "" {
				t.Errorf("ParseLiteral(%q) created unexpected diff (-want, +got): %s", tt.input, diff)
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
