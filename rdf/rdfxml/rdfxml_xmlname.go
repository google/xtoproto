package rdfxml

import (
	"fmt"
	"regexp"
	"strings"
)

// This file contains functionality for validating an XML name.
// based on https://www.w3.org/TR/xml/.

func checkXMLName(candidate string) error {
	if !xmlNameRegexp.MatchString(candidate) {
		return fmt.Errorf("%q is not a valid XML Name per https://www.w3.org/TR/REC-xml/#sec-common-syn", candidate)
	}
	return nil
}

func checkXMLNCName(candidate string) error {
	if err := checkXMLName(candidate); err != nil {
		return err
	}
	if strings.Contains(candidate, ":") {
		return fmt.Errorf("%q is not a valid NC name bacue it contains a : character - see NCName in https://www.w3.org/TR/REC-xml/#sec-common-syn", candidate)
	}
	return nil
}

// xmlNCName	  =   	xmlName - (xmlChar* ':' xmlChar*)	/* An XML Name, minus the ":" */

const (
	// See https://www.w3.org/TR/xml/ 2.3
	xmlNameStartChar = (`[\:A-Za-z` +
		`\xC0-\xD6` +
		`\xD8-\xF6` +
		`\xF8-\x{2FF}` +
		`\x{370}-\x{37D}` +
		`\x{37F}-\x{1FFF}` +
		`\x{200C}-\x{200D}` +
		`\x{2070}-\x{218F}` +
		`\x{2C00}-\x{2FEF}` +
		`\x{3001}-\x{D7FF}` +
		`\x{F900}-\x{FDCF}` +
		`\x{FDF0}-\x{FFFD}` +
		`\x{10000}-\x{EFFFF}]`)
	xmlNameChar = (`(?:` +
		xmlNameStartChar + `|` +
		`[\-\.0-9\xB7` + `\x{0300}-\x{036F}` + `\x{203F}-\x{2040}])`)
	xmlName     = xmlNameStartChar + xmlNameChar + `*`
	xmlNames    = xmlName + `(?:\x20` + xmlName + `)*`
	xmlNMToken  = `(?:` + xmlNameChar + `)+`
	xmlNMTokens = xmlNMToken + `(?:\x20` + xmlNMToken + `)*`
)

var xmlNameRegexp = regexp.MustCompile("^" + xmlName + "$")
