// Package iri contains facilities for working with Internationalized Resource
// Identifiers as specified in RFC 3987.
//
// RFC reference: https://www.ietf.org/rfc/rfc3987.html
package iri

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// An IRI (Internationalized Resource Identifier) within an RDF graph is a
// Unicode string [UNICODE] that conforms to the syntax defined in RFC 3987
// [RFC3987].
//
// See https://www.w3.org/TR/2014/REC-rdf11-concepts-20140225/#dfn-iri.
type IRI string

// Parse parses a string into an IRI and checks that it conforms to RFC 3987.
func Parse(s string) (IRI, error) {
	match := uriRE.FindStringSubmatch(s)
	if len(match) == 0 {
		return "", fmt.Errorf("%q is not a valid IRI - does not match regexp %s", s, uriRE)
	}
	scheme := match[uriRESchemeGroup]
	auth := match[uriREAuthorityGroup]
	path := match[uriREPathGroup]
	query := match[uriREQueryGroup]
	fragment := match[uriREFragmentGroup]
	if scheme != "" && !schemeRE.MatchString(scheme) {
		return "", fmt.Errorf("%q is not a valid IRI: invalid scheme %q does not match regexp %s", s, scheme, schemeRE)
	}
	if auth != "" && !iauthorityRE.MatchString(auth) {
		return "", fmt.Errorf("%q is not a valid IRI: invalid auth %q does not match regexp %s", s, auth, iauthorityRE)
	}
	if path != "" && !ipathRE.MatchString(path) {
		return "", fmt.Errorf("%q is not a valid IRI: invalid path %q does not match regexp %s", s, path, ipathRE)
	}
	if query != "" && !iqueryRE.MatchString(query) {
		return "", fmt.Errorf("%q is not a valid IRI: invalid query %q does not match regexp %s", s, query, iqueryRE)
	}
	if fragment != "" && !ifragmentRE.MatchString(fragment) {
		return "", fmt.Errorf("%q is not a valid IRI: invalid fragment %q does not match regexp %s", s, fragment, ifragmentRE)
	}
	parsed := IRI(s)

	if _, err := parsed.normalizePercentEncoding(); err != nil {
		return "", fmt.Errorf("%q is not a valid IRI: invalid percent encoding: %w", s, err)
	}

	return parsed, nil
}

// Check returns an error if the IRI is invalid.
func (iri IRI) Check() error {
	_, err := Parse(string(iri))
	return err
}

// String returns the N-Tuples-formatted IRI: "<" + iri + ">".
func (iri IRI) String() string {
	return fmt.Sprintf("<%s>", string(iri))
}

// ResolveReference resolves an IRI reference to an absolute IRI from an absolute
// base IRI u, per RFC 3986 Section 5.2. The IRI reference may be relative or
// absolute.
func (iri IRI) ResolveReference(other IRI) IRI {
	return resolveReference(iri.parts(), other.parts()).toIRI()
}

// parts returns the components of the URI or nil if there is a parsing error.
func (iri IRI) parts() *parts {
	match := uriRE.FindStringSubmatch(string(iri))
	if len(match) == 0 {
		return nil
	}
	auth := match[uriREAuthorityGroup]
	authMatch := iauthorityCaptureRE.FindStringSubmatch(auth)
	var userInfo, host, port string
	if len(authMatch) != 0 {
		userInfo = authMatch[iauthorityUserInfoGroup]
		host = authMatch[iauthorityHostGroup]
		port = authMatch[iauthorityPortGroup]
	}
	return &parts{
		scheme:        match[uriRESchemeGroup],
		emptyAuth:     len(match[uriREAuthorityWithSlashSlahGroup]) != 0 && (userInfo == "" && host == "" && port == ""),
		userInfo:      userInfo,
		host:          host,
		port:          port,
		path:          match[uriREPathGroup],
		query:         match[uriREQueryWithMarkGroup],
		fragment:      match[uriREFragmentGroup],
		emptyFragment: match[uriREFragmentWithHashGroup] != "",
	}
}

type parts struct {
	scheme        string
	emptyAuth     bool // true if the iri is something like `///path` but if iri is `//hostname/path`
	userInfo      string
	host          string
	port          string
	path          string
	query         string // with the ?
	emptyFragment bool
	fragment      string
}

func (p *parts) toIRI() IRI {
	s := ""
	if p.scheme != "" {
		s += p.scheme + ":"
	}
	if p.emptyAuth || p.userInfo != "" || p.host != "" || p.port != "" {
		s += "//"
	}
	if p.userInfo != "" { // TODO(reddaly): Deal with blank userInfo
		s += p.userInfo
	}
	if p.host != "" {
		s += p.host
	}

	if p.port != "" { // TODO(reddaly): Deal with blank
		s += ":" + p.port
	}
	if p.path != "" { // TODO(reddaly): Deal with blank
		s += p.path
	}
	if p.query != "" { // TODO(reddaly): Deal with blank
		s += p.query
	}
	if p.fragment != "" {
		s += "#" + p.fragment
	} else if p.emptyFragment {
		s += "#"
	}
	return IRI(s)
}

// Normalization background reading:
// - https://blog.golang.org/normalization
// - https://www.ietf.org/rfc/rfc3987.html#section-5
//    - https://www.ietf.org/rfc/rfc3987.html#section-5.3.2.3 - percent encoding

// Regular expression const strings, mostly derived from
// https://www.ietf.org/rfc/rfc3987.html#section-2.2.
const (
	hex        = `[0-9A-Fa-f]`
	alphaChars = "[a-zA-Z]" // see https://tools.ietf.org/html/rfc5234 B.1. "ALPHA"
	digitChars = `\d`       // see https://tools.ietf.org/html/rfc5234 B.1. "DIGIT"
	ucschar    = (`[\xA0-\x{D7FF}` +
		`\x{F900}-\x{FDCF}` +
		`\x{FDF0}-\x{FFEF}` +
		`\x{10000}-\x{1FFFD}` +
		`\x{20000}-\x{2FFFD}` +
		`\x{30000}-\x{3FFFD}` +
		`\x{40000}-\x{4FFFD}` +
		`\x{50000}-\x{5FFFD}` +
		`\x{60000}-\x{6FFFD}` +
		`\x{70000}-\x{7FFFD}` +
		`\x{80000}-\x{8FFFD}` +
		`\x{90000}-\x{9FFFD}` +
		`\x{A0000}-\x{AFFFD}` +
		`\x{B0000}-\x{BFFFD}` +
		`\x{C0000}-\x{CFFFD}` +
		`\x{D0000}-\x{DFFFD}` +
		`\x{E1000}-\x{EFFFD}]`)
	unreserved  = (`(?:` + alphaChars + "|" + digitChars + `|[\-\._~]` + `)`)
	iunreserved = (`(?:` + alphaChars + "|" + digitChars + `|[\-\._~]|` + ucschar + `)`)

	subDelims           = `[!\$\&\'\(\)\*\+\,\;\=]`
	pctEncoded          = `%` + hex + hex
	pctEncodedOneOrMore = `(?:(?:` + pctEncoded + `)+)`

	pchar  = "(?:" + unreserved + "|" + pctEncoded + "|" + subDelims + ")"
	ipchar = "(?:" + iunreserved + "|" + pctEncoded + "|" + subDelims + `|[\:@])`

	scheme = "(?:" + alphaChars + "(?:" + alphaChars + "|" + digitChars + `|[\+\-\.])*)`

	iauthority              = `(?:` + iuserinfo + "@)?" + ihost + `(?:\:` + port + `)?`
	iauthorityCapture       = `(?:(` + iuserinfo + "@)?(" + ihost + `)(?:\:(` + port + `))?)`
	iauthorityUserInfoGroup = 1
	iauthorityHostGroup     = 2
	iauthorityPortGroup     = 3
	iuserinfo               = `(?:(?:` + iunreserved + `|` + pctEncoded + `|` + subDelims + `)*)`
	port                    = `(?:\d*)`
	ihost                   = `(?:` + ipLiteral + `|` + ipV4Address + `|` + iregName + `)`
	iregName                = "(?:(?:" + iunreserved + "|" + pctEncoded + "|" + subDelims + ")*)" // *( iunreserved / pctEncoded / subDelims )

	ipath = (`(?:` + ipathabempty + // begins with "/" or is empty
		`|` + ipathabsolute + // begins with "/" but not "//"
		`|` + ipathnoscheme + // begins with a non-colon segment
		`|` + ipathrootless + // begins with a segment
		`|` + ipathempty + `)`) // zero characters

	ipathabempty  = `(?:(?:\/` + isegment + `)*)`
	ipathabsolute = `(?:\/(?:` + isegmentnz + `(?:\/` + isegment + `)*` + `)?)`
	ipathnoscheme = `(?:` + isegmentnznc + `(?:\/` + isegment + `)*)`
	ipathrootless = `(?:` + isegmentnz + `(?:\/` + isegment + `)*)`
	ipathempty    = `(?:)` // zero characters

	isegment   = `(?:` + ipchar + `*)`
	isegmentnz = `(?:` + ipchar + `+)`
	// non-zero-length segment without any colon ":"
	isegmentnznc = `(?:` + iunreserved + `|` + pctEncoded + `|` + subDelims + `|` + `[@])`

	iquery = `(?:(?:` + ipchar + `|` + iprivate + `|` + `\/\?` + `)*)`

	ifragment = `(?:(?:` + ipchar + `|` + `[\/\?]` + `)*)`

	iprivate = `[\x{E000}-\x{F8FF}` + `\x{F0000]-\x{FFFFD}` + `\x{100000}-\x{10FFFD}]`
)

// IP Address related
const (
	ipLiteral = `\[(?:` + ipV6Address + `|` + ipVFuture + `)\]`

	ipVFuture = `v` + hex + `\.(?:` + unreserved + `|` + subDelims + `|\:)*`

	// see https://stackoverflow.com/questions/3032593/using-explicitly-numbered-repetition-instead-of-question-mark-star-and-plus
	ipV6Address = (`(?:` +
		`(?:(?:` + h16 + `\:){6}` + ls32 + `)` + //          6( h16 `:` ) ls32
		// TODO(reddaly): below lines
		//  + `|` +                       `::` 5( h16 `:` ) ls32
		//  + `|` + [               h16 ] `::` 4( h16 `:` ) ls32
		//  + `|` + [ *1( h16 `:` ) h16 ] `::` 3( h16 `:` ) ls32
		//  + `|` + [ *2( h16 `:` ) h16 ] `::` 2( h16 `:` ) ls32
		//  + `|` + [ *3( h16 `:` ) h16 ] `::`    h16 `:`   ls32
		//  + `|` + [ *4( h16 `:` ) h16 ] `::`              ls32
		//  + `|` + [ *5( h16 `:` ) h16 ] `::`              h16
		//  + `|` + [ *6( h16 `:` ) h16 ] `::`
		`)`)

	h16         = `(?:` + hex + hex + hex + hex + `)`
	ls32        = `(?:` + h16 + `\:` + h16 + `|` + ipV4Address + `)`
	ipV4Address = `(?:` + decOctet + `.` + decOctet + `.` + decOctet + `.` + decOctet + `)`

	decOctet = (`(?:\d` + `|` + // 0-9
		`[1-9]\d` + `|` + // 10-99
		`1\d\d` + `|` + // 100-199
		`2[0-4]\d` + `|` + // 200-249
		`25[0-5]` + `)`) // 250-255
)

var (
	ipLiteralRE = mustCompileNamed("ipLiteralRE", `(?:`+ipLiteral+`)`)
)

var (
	schemeRE            = mustCompileNamed("schemeRE", "^"+scheme+"$")
	iauthorityRE        = mustCompileNamed("iauthorityRE", "^"+iauthority+"$")
	iauthorityCaptureRE = mustCompileNamed("iauthorityCaptureRE", "^"+iauthorityCapture+"$")
	ipathRE             = mustCompileNamed("ipath", "^"+ipath+"$")
	iqueryRE            = mustCompileNamed("iquery", "^"+ipath+"$")
	ifragmentRE         = mustCompileNamed("ifragment", "^"+ifragment+"$")

	percentEncodedChar      = mustCompileNamed("percentEncodedChar", pctEncoded)
	pctEncodedCharOneOrMore = mustCompileNamed("pctEncodedOneOrMore", pctEncodedOneOrMore)
	iunreservedRE           = mustCompileNamed("iunreservedRE", "^"+iunreserved+"$")

	hexToRune = func() map[string]rune {
		m := map[string]rune{}
		for i := 0; i <= 255; i++ {
			m[fmt.Sprintf("%02X", i)] = rune(i)
		}
		return m
	}()
	hexToByte = func() map[string]byte {
		m := map[string]byte{}
		for i := 0; i <= 255; i++ {
			m[fmt.Sprintf("%02X", i)] = byte(i)
		}
		return m
	}()
	byteToUppercasePercentEncoding = func() map[byte]string {
		m := map[byte]string{}
		for i := 0; i <= 255; i++ {
			m[byte(i)] = fmt.Sprintf("%%%02X", i)
		}
		return m
	}()

	// re from RFC 3986 page 50.
	uriRE                            = mustCompileNamed("uriRE", `^(([^:/?#]+):)?(//([^/?#]*))?([^?#]*)(\?([^#]*))?(#(.*))?`)
	uriRESchemeGroup                 = 2
	uriREAuthorityWithSlashSlahGroup = 3
	uriREAuthorityGroup              = 4
	uriREPathGroup                   = 5
	uriREQueryWithMarkGroup          = 6
	uriREQueryGroup                  = 7
	uriREFragmentGroup               = 9
	uriREFragmentWithHashGroup       = 8
)

// NormalizePercentEncoding returns an IRI that replaces any unnecessarily
// percent-escaped characters with unescaped characters.
//
// RFC3987 discusses this normalization procedure in 5.3.2.3:
// https://www.ietf.org/rfc/rfc3987.html#section-5.3.2.3.
func (iri IRI) NormalizePercentEncoding() IRI {
	normalized, err := iri.normalizePercentEncoding()
	if err != nil {
		return iri
	}
	return normalized
}

func (iri IRI) normalizePercentEncoding() (IRI, error) {
	var errs []error
	// Find consecutive percent-encoded octets and encode them together.
	replaced := pctEncodedCharOneOrMore.ReplaceAllStringFunc(string(iri), func(pctEscaped string) string {
		octets := make([]byte, len(pctEscaped)/3)
		for i := 0; i < len(octets); i++ {
			start := i * 3
			digitsStr := strings.ToUpper(pctEscaped[start+1 : start+3])
			octet, ok := hexToByte[digitsStr]
			if !ok {
				panic(fmt.Errorf("internal error: hex %q not present in %+v", octet, hexToRune)) // should not occur because of regexp
			}
			octets[i] = octet
		}

		normalized := ""
		unconsumedOctets := octets
		octetsOffset := 0
		for len(unconsumedOctets) > 0 {
			codePoint, size := utf8.DecodeRune(unconsumedOctets)
			if codePoint == utf8.RuneError {
				errs = append(errs, fmt.Errorf("percent-encoded sequence %q  contains invalid UTF-8 code point at start of byte sequence %+v", pctEscaped[octetsOffset*3:], unconsumedOctets))
				return pctEscaped
			}

			if iunreservedRE.MatchString(string(codePoint)) {
				normalized += string(codePoint)
			} else {
				buf := make([]byte, 4)
				codePointOctetCount := utf8.EncodeRune(buf, codePoint)
				for i := 0; i < codePointOctetCount; i++ {
					normalized += byteToUppercasePercentEncoding[buf[i]]
				}
			}
			unconsumedOctets = unconsumedOctets[size:]
			octetsOffset += size
		}

		return normalized
	})
	if len(errs) != 0 {
		return IRI(replaced), errs[0]
	}
	return IRI(replaced), nil
}

func mustCompileNamed(name, expr string) *regexp.Regexp {
	c, err := regexp.Compile(expr)
	if err != nil {
		panic(fmt.Errorf("failed to compile regexp %s: %w", name, err))
	}
	return c
}
