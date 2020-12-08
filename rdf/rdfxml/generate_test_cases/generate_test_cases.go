// Program generate_test_cases outputs a .go file with test cases based on
// the official w3c test case repository.
package main

import (
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var (
	manifestPath = flag.String("manifest", "third_party/w3c.org/rdf-testcases/Manifest.rdf", "Path to Manifest.rdf file with test case information.")
	outputPath   = flag.String("output", "rdf/rdfxml/rdftestcases/rdftestcases.go", "Path to generated .go code with test case data.")
)

func main() {
	flag.Parse()
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "fatal error: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	f, err := os.Open(*manifestPath)
	if err != nil {
		return err
	}
	defer f.Close()
	manifest, err := loadParserTestCases(ctx, xml.NewDecoder(f))
	if err != nil {
		return err
	}
	code, err := generateCode(ctx, manifest)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(*outputPath, []byte(code), 0664); err != nil {
		return err
	}
	return nil
}

func loadParserTestCases(ctx context.Context, dec *xml.Decoder) (*testManifest, error) {
	mf := &testManifest{}
	if err := dec.Decode(mf); err != nil {
		return nil, err
	}
	return mf, nil
}

func generateCode(ctx context.Context, mf *testManifest) (string, error) {
	positives, negatives, err := loadCases(ctx, mf)
	if err != nil {
		return "", err
	}

	code := `// Package rdftestcases contains go structs with the contents of the W3C's official RDF parser test cases.
package rdftestcases

// PositiveParserCase is a test case that should successfully parse.
type PositiveParserCase struct {
	// Name is the test case name.
	Name string

	// InputXML is the input XML file contents.
	InputXML string

	// DocumentURL is the base url to use for the document.
	DocumentURL string

	// OutputNTriples is the expected set of output triples.
	OutputNTriples string
}

// NegativeParserCase is a test case that should fail to parse.
type NegativeParserCase struct {
	// Name is the test case name.
	Name string

	// InputXML is the input XML file contents.
	InputXML string

	// DocumentURL is the base url to use for the document.
	DocumentURL string
}

`

	code += fmt.Sprintf("// Positives contains %d valid examples of RDF/XML.\n", len(positives))
	posStructStrings := []string{}
	for _, t := range positives {
		posStructStrings = append(posStructStrings,
			fmt.Sprintf(`  {
    Name:           %q,
    InputXML:       %q,
    DocumentURL:    %q,
    OutputNTriples: %q,
  },`,
				t.Name, t.InputXML, t.DocumentURL, t.OutputNTriples))
	}
	code += fmt.Sprintf("var Positives = []PositiveParserCase{\n%s\n}\n\n", strings.Join(posStructStrings, "\n"))

	code += fmt.Sprintf("// Negatives contains %d invalid examples of RDF/XML.\n", len(positives))
	negStructStrings := []string{}
	for _, t := range negatives {
		negStructStrings = append(negStructStrings,
			fmt.Sprintf("  {\n    Name:           %q,\n    InputXML:       %q,\n  },",
				t.Name, t.InputXML))
	}
	code += fmt.Sprintf("var Negatives = []NegativeParserCase{\n%s\n}\n\n", strings.Join(negStructStrings, "\n"))

	return code, nil
}

func loadCases(ctx context.Context, mf *testManifest) ([]*PositiveParserCase, []*NegativeParserCase, error) {
	var pos []*PositiveParserCase
	var neg []*NegativeParserCase
	for _, t := range mf.PositiveParserTest {
		if t.Status != "APPROVED" {
			continue
		}
		inputContents, err := loadTestFile(ctx, t.InputDocument.RDFXMLDocument.About)
		if err != nil {
			return nil, nil, err
		}
		outputContents, err := loadTestFile(ctx, t.OutputDocument.NTDocument.About)
		if err != nil {
			return nil, nil, err
		}
		pos = append(pos, &PositiveParserCase{
			Name:           t.About,
			InputXML:       inputContents,
			DocumentURL:    t.InputDocument.RDFXMLDocument.About,
			OutputNTriples: outputContents,
		})
	}
	for _, t := range mf.NegativeParserTest {
		if t.Status != "APPROVED" {
			continue
		}
		inputContents, err := loadTestFile(ctx, t.InputDocument.RDFXMLDocument.About)
		if err != nil {
			return nil, nil, err
		}
		neg = append(neg, &NegativeParserCase{
			Name:        t.About,
			InputXML:    inputContents,
			DocumentURL: t.InputDocument.RDFXMLDocument.About,
		})
	}
	return pos, neg, nil
}

func loadTestFile(ctx context.Context, url string) (string, error) {
	relativePath := strings.TrimPrefix(url, "http://www.w3.org/2000/10/rdf-tests/rdfcore/")
	pathElems := []string{filepath.Dir(*manifestPath)}
	pathElems = append(pathElems, strings.Split(relativePath, "/")...)
	data, err := ioutil.ReadFile(filepath.Join(pathElems...))
	if err != nil {
		return "", fmt.Errorf("failed to load test file %s: %w", url, err)
	}
	return string(data), nil
}

// PositiveParserCase is a test case that should successfully parse.
type PositiveParserCase struct {
	// Name is the test case name.
	Name string

	// InputXML is the input XML file contents.
	InputXML string

	// DocumentURL is the base url to use for the document.
	DocumentURL string

	// OutputNTriples is the expected set of output triples.
	OutputNTriples string
}

// NegativeParserCase is a test case that should fail to parse.
type NegativeParserCase struct {
	// Name is the test case name.
	Name string

	// InputXML is the input XML file contents.
	InputXML string

	// DocumentURL is the base url to use for the document.
	DocumentURL string
}

type testManifest struct {
	XMLName            xml.Name              `xml:"RDF"`
	Text               string                `xml:",chardata"`
	Rdf                string                `xml:"rdf,attr"`
	Rdfs               string                `xml:"rdfs,attr"`
	Test               string                `xml:"test,attr"`
	PositiveParserTest []*positiveParserTest `xml:"PositiveParserTest"`
	NegativeParserTest []*negativeParserTest `xml:"NegativeParserTest"`
}

type positiveParserTest struct {
	Text     string `xml:",chardata"`
	About    string `xml:"about,attr"`
	Status   string `xml:"status"`
	Approval struct {
		Text     string `xml:",chardata"`
		Resource string `xml:"resource,attr"`
	} `xml:"approval"`
	InputDocument struct {
		Text           string `xml:",chardata"`
		RDFXMLDocument struct {
			Text  string `xml:",chardata"`
			About string `xml:"about,attr"`
		} `xml:"RDF-XML-Document"`
	} `xml:"inputDocument"`
	OutputDocument struct {
		Text       string `xml:",chardata"`
		NTDocument struct {
			Text  string `xml:",chardata"`
			About string `xml:"about,attr"`
		} `xml:"NT-Document"`
	} `xml:"outputDocument"`
	Description string `xml:"description"`
	Issue       struct {
		Text     string `xml:",chardata"`
		Resource string `xml:"resource,attr"`
	} `xml:"issue"`
	Discussion struct {
		Text     string `xml:",chardata"`
		Resource string `xml:"resource,attr"`
	} `xml:"discussion"`
	Warning string `xml:"warning"`
}

type negativeParserTest struct {
	Text  string `xml:",chardata"`
	About string `xml:"about,attr"`
	Issue struct {
		Text     string `xml:",chardata"`
		Resource string `xml:"resource,attr"`
	} `xml:"issue"`
	Status   string `xml:"status"`
	Approval struct {
		Text     string `xml:",chardata"`
		Resource string `xml:"resource,attr"`
	} `xml:"approval"`
	Discussion struct {
		Text     string `xml:",chardata"`
		Resource string `xml:"resource,attr"`
	} `xml:"discussion"`
	Description   string `xml:"description"`
	InputDocument struct {
		Text           string `xml:",chardata"`
		RDFXMLDocument struct {
			Text  string `xml:",chardata"`
			About string `xml:"about,attr"`
		} `xml:"RDF-XML-Document"`
	} `xml:"inputDocument"`
}
