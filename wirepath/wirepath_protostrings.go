package wirepath

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// const (
// 	intLit     = decimalLit | octalLit | hexLit
// 	decimalLit = ( "1" … "9" ) { decimalDigit }
// 	octalLit   = "0" { octalDigit }
// 	hexLit     = "0" ( "x" | "X" ) hexDigit { hexDigit }

// 	strLit = ( "'" { charValue } "'" ) |  ( '"' { charValue } '"' )
// 	charValue = hexEscape | octEscape | charEscape | /[^\0\n\\]/
// 	hexEscape = '\' ( "x" | "X" ) hexDigit hexDigit
// 	octEscape = '\' octalDigit octalDigit octalDigit
// 	charEscape = '\' ( "a" | "b" | "f" | "n" | "r" | "t" | "v" | '\' | "'" | '"' )
// 	quote = "'" | '"'
// )

const (
	hexDigit   = `[0-9A-Fa-f]`
	octalDigit = `[0-7]`

	protobufStringLiteralChar = `(?:` +
		`\\[xX](` + hexDigit + hexDigit + `)|` + // hex
		`\\0(` + octalDigit + octalDigit + octalDigit + `)|` + // octal
		`\\([abfnrtv\\'"])` + "|" + // charEscape
		`([^\0\n\\])` + // regular character
		`)`
	protobufStringLiteralDoubleQuoted = `(?:"` + protobufStringLiteralChar + `*")`
)

var protobufStringElemRegexp = regexp.MustCompile("^" + protobufStringLiteralChar)

// parseProtobufStringLiteral parses a string literal according to the protobuf spec:
// https://developers.google.com/protocol-buffers/docs/reference/proto3-spec#letters_and_digits
func parseProtobufStringLiteral(literal string) (string, error) {
	if !strings.HasPrefix(literal, `"`) {
		return "", fmt.Errorf("literal must begin with \": %s", literal)
	}
	if !strings.HasSuffix(literal, `"`) {
		return "", fmt.Errorf("literal must end with \": %s", literal)
	}
	literal = literal[1 : len(literal)-1]

	out := &strings.Builder{}

	const (
		hexGroup         = 1
		octalGroup       = 2
		escapedCharGroup = 3
		regularCharGroup = 4
	)

	unparsed := literal
	for len(unparsed) > 0 {
		groups := protobufStringElemRegexp.FindStringSubmatch(unparsed)
		if len(groups) == 0 {
			return "", fmt.Errorf("invalid string literal %s; %q does not match %s", literal, unparsed, protobufStringElemRegexp)
		}
		unparsed = unparsed[len(groups[0]):]

		if g := groups[regularCharGroup]; g != "" {
			out.WriteString(g)
			continue
		}
		if g := groups[hexGroup]; g != "" {
			number, err := strconv.ParseUint(g, 16, 8)
			if err != nil {
				return "", fmt.Errorf("internal error in string parser with hex digits: %w", err)
			}
			out.WriteByte(byte(number))
			continue
		}
		if g := groups[octalGroup]; g != "" {
			number, err := strconv.ParseUint(g, 8, 8)
			if err != nil {
				return "", fmt.Errorf("internal error in string parser with octal digits: %w", err)
			}
			out.WriteByte(byte(number))
			continue
		}
		if g := groups[escapedCharGroup]; g != "" {
			var char rune
			switch g[0] {
			case 'a':
				char = '\a'
			case 'b':
				char = '\b'
			case 'f':
				char = '\f'
			case 'n':
				char = '\n'
			case 'r':
				char = '\r'
			case 't':
				char = '\t'
			case 'v':
				char = '\v'
			case '\\':
				char = '\\'
			case '\'':
				char = '\''
			case '"':
				char = '"'
			default:
				return "", fmt.Errorf("programing error in charEscape")
			}
			out.WriteRune(char)
			continue
		}
	}

	return out.String(), nil
}

// 	bytePos := 0
// 	read := func() (rune, error) {
// 		r, count, err := reader.ReadRune()
// 		bytePos += count
// 		return r, err
// 	}
// 	peek := func() (rune, error) {
// 		r, err := read()
// 		if err != nil {
// 			return r, err
// 		}
// 		err = reader.UnreadRune()
// 		return r, err
// 	}
// 	consumeRuneOrErr := func(want rune) error {
// 		startPos := bytePos
// 		r, err := read()
// 		if err != nil {
// 			return err
// 		}
// 		if r != want {
// 			return fmt.Errorf("got %q, want %q at position %d of protobuf string literal", r, want, startPos)
// 		}
// 	}

// 	if err := consumeRuneOrErr('"'); err != nil {
// 		return "", err
// 	}

// 	builder := &strings.Builder{}

// 	for {
// 		cursorPos := bytePos
// 		r, err := read()
// 		if err != nil {
// 			return "", err
// 		}

// 	}

// }
