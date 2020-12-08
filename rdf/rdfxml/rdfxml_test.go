package rdfxml

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/xtoproto/rdf/ntriples"
	"github.com/google/xtoproto/rdf/rdfxml/rdftestcases"
)

var cmpOpts = []cmp.Option{
	cmp.Transformer("triple", func(tr *ntriples.Triple) stringifiedTriple {
		return stringifiedTriple{
			Subject:   tr.Subject().String(),
			Predicate: tr.Predicate().String(),
			Object:    tr.Object().String(),
		}
	}),
}

type stringifiedTriple struct {
	Subject, Predicate, Object string
}

// Test cases based on the {Positive, Negative} Parser Tests in
// https://www.w3.org/TR/rdf-testcases/.

func TestReadTriples_positive(t *testing.T) {
	for _, tt := range rdftestcases.Positives {
		t.Run(goFriendlyTestName(tt.Name), func(t *testing.T) {
			want, err := ntriples.ParseLines(cleanLines(strings.Split(tt.OutputNTriples, "\n")))
			if err != nil {
				t.Fatalf("failed to parse triples: %v", err)
			}
			t.Logf("want %d triples", len(want))
			parser := NewParser(xmlTokenizerFromString(tt.InputXML), &ParserOptions{BaseURL: ntriples.IRI(tt.DocumentURL)})
			got, err := parser.ReadAllTriples()
			if err != nil {
				t.Errorf("RDF/XML parse failed: %v", err)
			}
			if diff := cmp.Diff(want, got, cmpOpts...); diff != "" {
				t.Errorf("unexpected diff in parsed triples (-want, +got):\n  %s", diff)
			}
		})
	}
}

func cleanLines(lines []string) []string {
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		out = append(out, strings.TrimRight(l, "\r"))
	}
	return out
}

func xmlTokenizerFromString(xmlContents string) xml.TokenReader {
	return xml.NewDecoder(strings.NewReader(xmlContents))
}

func goFriendlyTestName(nameURL string) string {
	clean := strings.ReplaceAll(nameURL, "http://www.w3.org/2000/10/rdf-tests/rdfcore/", "")
	clean = strings.ReplaceAll(clean, "/Manifest.rdf#", "-")
	return clean
}
