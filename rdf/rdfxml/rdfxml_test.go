package rdfxml

import (
	"encoding/xml"
	"sort"
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
			if diff := diffTriples(want, got); diff != "" {
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

func diffTriples(a, b []*ntriples.Triple) string {
	want, got := a, b
	// See "Canonical Forms for Isomorphic and Equivalent RDF Graphs: Algorithms
	// for Leaning and Labelling Blank Nodes" for an algorithm that canonicalizes
	// blank nodes in an RDF graph.

	// We take a few diffs between a and b and return the shortest one, which we
	// assume means it is the most helpful.
	diff1 := cmp.Diff(a, b, cmpOpts...)
	if diff1 == "" {
		return ""
	}
	aSorted := copyTriples(a)
	sortIgnoringBlankNodes(aSorted)
	bSorted := copyTriples(b)
	sortIgnoringBlankNodes(bSorted)
	diff2 := cmp.Diff(aSorted, bSorted, cmpOpts...)
	if diff2 == "" {
		return ""
	}

	diffs := []string{diff1, diff2}

	if mapFn := mappingFromSortedTriples(want, got); mapFn != nil {
		canonicalizedGot := applyBlankNodeMapping(got, mapFn)
		diff3 := cmp.Diff(want, canonicalizedGot, cmpOpts...)
		if diff3 == "" {
			return ""
		}
		diffs = append(diffs, diff3)
	}

	sort.Slice(diffs, func(i, j int) bool {
		return len(diffs[i]) < len(diffs[j])
	})
	return diffs[0]
}

func mappingFromSortedTriples(want, got []*ntriples.Triple) func(ntriples.BlankNodeID) ntriples.BlankNodeID {
	if len(want) != len(got) {
		return nil
	}
	m := map[ntriples.BlankNodeID]ntriples.BlankNodeID{}
	for i, canonicalTriple := range want {
		gotTriple := got[i]
		if canonicalTriple.Subject().IsBlankNode() && gotTriple.Subject().IsBlankNode() {
			m[gotTriple.Subject().BlankNodeID()] = canonicalTriple.Subject().BlankNodeID()
		}
		if canonicalTriple.Object().IsBlankNode() && gotTriple.Object().IsBlankNode() {
			m[gotTriple.Object().BlankNodeID()] = canonicalTriple.Object().BlankNodeID()
		}
	}
	return func(id ntriples.BlankNodeID) ntriples.BlankNodeID {
		canonical, ok := m[id]
		if !ok {
			return id
		}
		return canonical
	}
}

func applyBlankNodeMapping(triples []*ntriples.Triple, mapping func(ntriples.BlankNodeID) ntriples.BlankNodeID) []*ntriples.Triple {
	mapSubject := func(s *ntriples.Subject) *ntriples.Subject {
		if !s.IsBlankNode() {
			return s
		}
		return ntriples.NewSubjectBlankNodeID(mapping(s.BlankNodeID()))
	}
	mapObject := func(o *ntriples.Object) *ntriples.Object {
		if !o.IsBlankNode() {
			return o
		}
		return ntriples.NewObjectBlankNodeID(mapping(o.BlankNodeID()))
	}
	mapTriple := func(tr *ntriples.Triple) *ntriples.Triple {
		if !tr.Subject().IsBlankNode() && !tr.Object().IsBlankNode() {
			return tr
		}
		return ntriples.NewTriple(mapSubject(tr.Subject()), tr.Predicate(), mapObject(tr.Object()))
	}
	var out []*ntriples.Triple
	for _, tr := range triples {
		out = append(out, mapTriple(tr))
	}
	return out
}

func sortIgnoringBlankNodes(sl []*ntriples.Triple) {
	sort.Slice(sl, func(i, j int) bool {
		a, b := sl[i], sl[j]
		if a.Subject().IsBlankNode() && !b.Subject().IsBlankNode() {
			return true
		}
		if b.Subject().IsBlankNode() && !a.Subject().IsBlankNode() {
			return false
		}
		if !a.Subject().IsBlankNode() && !b.Subject().IsBlankNode() {
			aStr, bStr := a.Subject().String(), b.Subject().String()
			if aStr != bStr {
				return aStr < bStr
			}
		}
		if sa, sb := a.Predicate().String(), b.Predicate().String(); sa != sb {
			return sa < sb
		}

		if a.Object().IsBlankNode() && !b.Object().IsBlankNode() {
			return true
		}
		if b.Object().IsBlankNode() && !a.Object().IsBlankNode() {
			return false
		}
		if !a.Object().IsBlankNode() && !b.Object().IsBlankNode() {
			aStr, bStr := a.Object().String(), b.Object().String()
			if aStr != bStr {
				return aStr < bStr
			}
		}
		return false
	})
}

func copyTriples(s []*ntriples.Triple) []*ntriples.Triple {
	var out []*ntriples.Triple
	out = append(out, s...)
	return out
}
