package rdfxml

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/xtoproto/rdf/ntriples"
	"github.com/google/xtoproto/rdf/rdfxml/rdftestcases"
)

const (
	parseIsFatal = true
	logNTriples  = true
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
			parser := NewParser(xmlTokenizerFromString(tt.InputXML), &ParserOptions{BaseURL: ntriples.IRI(tt.DocumentURL)})
			got, err := parser.ReadAllTriples()
			if err != nil {
				if parseIsFatal {
					t.Fatalf("RDF/XML parse failed: %v", err)
				} else {
					t.Errorf("RDF/XML parse failed: %v", err)
				}
			}
			if logNTriples {
				for i, tr := range got {
					t.Logf("got triple[%02d]: %s", i, tr)
				}
			}
			got, want = canonicalizedTriples(got), canonicalizedTriples(want)
			if diff := diffTriples(t, want, got); diff != "" {
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

func diffTriples(t *testing.T, a, b []*ntriples.Triple) string {
	t.Helper()
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
	sortNodes(aSorted, false)
	bSorted := copyTriples(b)
	sortNodes(bSorted, false)
	diff2 := cmp.Diff(aSorted, bSorted, cmpOpts...)
	if diff2 == "" {
		return ""
	}

	diffs := []string{diff1, diff2}

	want, got := aSorted, bSorted
	if mapFn := naiveMappingFromSortedTriples(t, want, got); mapFn != nil {
		canonicalizedGot := applyBlankNodeMapping(got, mapFn)
		sortNodes(canonicalizedGot, true)
		wantSorted := copyTriples(want)
		sortNodes(wantSorted, true)
		diff3 := cmp.Diff(wantSorted, canonicalizedGot, cmpOpts...)
		if diff3 == "" {
			return ""
		}
		//diffs = append(diffs, diff3)
		return diff3
	}

	sort.Slice(diffs, func(i, j int) bool {
		return len(diffs[i]) < len(diffs[j])
	})
	return diffs[0]
}

func naiveMappingFromSortedTriples(t *testing.T, want, got []*ntriples.Triple) func(ntriples.BlankNodeID) ntriples.BlankNodeID {
	t.Helper()
	if len(want) != len(got) {
		t.Logf("can't generate naive mapping because of length mismatch (got %d, want %d)", len(got), len(want))
		return nil
	}
	m := map[ntriples.BlankNodeID]ntriples.BlankNodeID{}
	hasEntry := func(id ntriples.BlankNodeID) bool {
		_, ok := m[id]
		return ok
	}
	for i, canonicalTriple := range want {
		gotTriple := got[i]
		// Check that the non-blank node portions of the tuples match.
		if !canonicalTriple.Subject().IsBlankNode() && !ntriples.SubjectsEqual(gotTriple.Subject(), canonicalTriple.Subject()) {
			continue
		}
		if !canonicalTriple.Object().IsBlankNode() && !ntriples.ObjectsEqual(gotTriple.Object(), canonicalTriple.Object()) {
			continue
		}

		if canonicalTriple.Subject().IsBlankNode() && gotTriple.Subject().IsBlankNode() {
			id := gotTriple.Subject().BlankNodeID()
			if !hasEntry(id) {
				m[id] = canonicalTriple.Subject().BlankNodeID()
			}
		}
		if canonicalTriple.Object().IsBlankNode() && gotTriple.Object().IsBlankNode() {
			id := gotTriple.Object().BlankNodeID()
			if !hasEntry(id) {
				m[id] = canonicalTriple.Object().BlankNodeID()
			}
		}
	}
	t.Logf("created naive mapping with %d entries: %+v", len(m), m)
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

func sortNodes(sl []*ntriples.Triple, compareBlankNodes bool) {
	sort.Slice(sl, func(i, j int) bool {
		a, b := sl[i], sl[j]
		if a.Subject().IsBlankNode() && !b.Subject().IsBlankNode() {
			return true
		}
		if b.Subject().IsBlankNode() && !a.Subject().IsBlankNode() {
			return false
		}
		bothSubjectsAreBlank := a.Subject().IsBlankNode() && b.Subject().IsBlankNode()
		if !bothSubjectsAreBlank || compareBlankNodes {
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
		bothObjectsAreBlank := a.Object().IsBlankNode() && b.Object().IsBlankNode()
		if !bothObjectsAreBlank || compareBlankNodes {
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

func TestIsXMLAttr(t *testing.T) {
	for _, tt := range []struct {
		name xml.Name
		want bool
	}{
		{
			xml.Name{Space: "", Local: "xmlns"},
			true,
		},
	} {
		t.Run(fmt.Sprintf("%s %s", tt.name.Space, tt.name.Local), func(t *testing.T) {
			got := isXMLAttr(tt.name)
			if got != tt.want {
				t.Errorf("isXMLAttr(%+v) got %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func canonicalizedTriples(in []*ntriples.Triple) []*ntriples.Triple {
	var out []*ntriples.Triple
	for _, tr := range in {
		out = append(out, canonicalizedTriple(tr))
	}
	return out
}

func canonicalizedTriple(tr *ntriples.Triple) *ntriples.Triple {
	if !tr.Object().IsLiteral() {
		return tr
	}
	return ntriples.NewTriple(tr.Subject(), tr.Predicate(), ntriples.NewObjectLiteral(canonicalizedLiteral(tr.Object().Literal())))
}

func canonicalizedLiteral(lit ntriples.Literal) ntriples.Literal {
	if lit.Datatype() != RDFXMLLiteral {
		return lit
	}
	// read in the xml and then print it out again
	dec := xml.NewDecoder(strings.NewReader(lit.LexicalForm()))
	out := &strings.Builder{}
	enc := xml.NewEncoder(out)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return lit
		}
		if err := enc.EncodeToken(tok); err != nil {
			return lit
		}
	}
	return ntriples.NewLiteral(out.String(), RDFXMLLiteral, "")
}
