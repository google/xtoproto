// Package sexpr parses s-expressions in a manner similar to the Common Lisp
// reader. It supports typical s-expression syntax and customization.
//
// Textual expressions are called "forms" by this package and represented with
// the Form interface. Form provides a Value() method for obtaining the
// underlying value. There are specific interfaces like `StringForm`,
// `NumberForm`, `CommentForm`, etc. corresponding to typical s-expression
// types. Type switches and assertions may be used to cast a form to a suitable
// type.
package sexpr

import (
	"fmt"
	"go/constant"
	"io"
	"strconv"
	"strings"
)

// readerRequiredSymbol specifies one of the symbols needed by the the s-expression parser
// to handle typical lisp-style s-expressions
type readerRequiredSymbol string

const (
	quoteSymbol           readerRequiredSymbol = "QUOTE"            // '
	quasiquoteSymbol      readerRequiredSymbol = "QUASIQUOTE"       // `
	unquoteSymbol         readerRequiredSymbol = "UNQUOTE"          // ,
	unquoteSplicingSymbol readerRequiredSymbol = "UNQUOTE-SPLICING" // ,@
)

// Form is a value that may have a corresponding textual representation in
// a source code file. Forms parsed by this package have an S-expression
// syntax unless the reader is customized.
//
// A Form has an underlying value, obtainable by calling Value(). For example,
// the s-expression `"abc"` has an underlying value that is the string "abc".
//
// This package does not define any implementations of Form. FormReader allows a
// custom FormProvider to be specified that will create a Form for a given
// underlying value and Sourcespan.
type Form interface {
	SourceSpan() SourceSpan
	Value() interface{}
}

// ListForm implements Form for a form comprised of an ordered list of subforms.
//
// The s-expression `("abc" xyz)` would be expected to be read as a ListForm
// with two subforms.
type ListForm interface {
	Form
	// Subforms returns the ordered list of forms that comprise the list.
	//
	// The list of forms should be substantive.
	Subforms() []Form

	// Len returns the length of the list. It is equivalent to len(Subforms())
	// but may be more efficient.
	Len() int

	// Nth returns Subforms()[n]. It may panic if the length of the list is
	// <= n.
	Nth(n int) Form
}

// StringForm is an interface for a form with an underlying string literal
// representation.
//
// This method should not be implemented by other types of forms, even if they
// can be represented as a string. For example, a number should implemented
// NumberForm and not StringForm.
type StringForm interface {
	Form

	// String Value returns the value of the form as a string literal.
	StringValue() string
}

// NumberForm is an interface for a form with an underlying number literal.
//
// A NumberForm is roughly equivalent to an untyped number const in Go.
type NumberForm interface {
	Form

	// NumberValue returns the number using go's constant package. The
	// value should be one of `int64, *big.Int, *big.Float, *big.Rat`.
	NumberValue() constant.Value
}

// ValuelessForm is a form that should be ignored in most contexts. Examples of
// valueless forms are comments and whitespace.
type ValuelessForm interface {
	Form

	// The Valueless function indicates that this form should be ignored
	// when read in most contexts. Examples of valueless forms are
	Valueless()
}

// CommentForm is an interface for a form with an underlying comment literal.
type CommentForm interface {
	ValuelessForm

	// Comment returns the comment literal without delimiters. Note that
	// multiple CommentForms may comprise a single logical comment block
	// separated by whitespace, as the whitespace forms will be parsed
	// separately.
	Comment() CommentText
}

// WhitespaceText contains the contents of a Whitespace literal.
type WhitespaceText string

// WhitespaceForm is an interface for a form with an underlying string literal
// representation.
type WhitespaceForm interface {
	ValuelessForm

	// Whitespace returns the whiespace literal.
	Whitespace() WhitespaceText
}

// CommentText contains the contents of a comment. This value excludes
// the characters used to delimit the comment, such as '//', '/*', and '*/'.
type CommentText string

type readTable map[rune]func(fr *FormReader) (ReaderMacroResult, error)

// ReaderMacroResult is returned by custom reader macro functions.
type ReaderMacroResult interface {
	// Skip returns true if the reader macro declined to read from the stream
	// and returned the cursor back to the original location. If true is
	// rturned, the Form() function will not be called.
	Skip() bool

	// The form read by the reader macro.
	Form() Form
}

// FormReader reads a stream of S-Expressions.
type FormReader struct {
	src       sourceFile
	readTable readTable
	factory   FormFactory
}

type Option interface {
	apply(*FormReader)
}

type simpleOption func(fr *FormReader)

func (opt simpleOption) apply(fr *FormReader) {
	opt(fr)
}

// CustomFormFactory returns an Option that may be passed to NewFileReader
// that uses a non-default FormFactory for constructing Forms.
//
// The passed in function will be called with a newly created *FormReader
// as an argument, allowing the factory to depend on the reader to implement
// its functionality.
func CustomFormFactory(factoryProvider func(*FormReader) FormFactory) Option {
	return simpleOption(func(fr *FormReader) {
		fr.factory = factoryProvider(fr)
	})
}

// NewFileReader returns an object for reading Forms from a source file, which
// is provided as a string.
//
// The filename value is used to print error messages and will not be accessed
// by the reader, so it does not need to be a real file at all.
func NewFileReader(fileName, contents string, opts ...Option) *FormReader {
	fr := newFormReader(fileName, contents)
	for _, opt := range opts {
		opt.apply(fr)
	}
	return fr
}

func newFormReader(name, code string) *FormReader {
	sf := newStrSourceFile(name, code)
	fr := &FormReader{sf, make(readTable), nil}
	fr.factory = DefaultFormFactory(fr)

	// To give a flavor of macro characters, we use a non-builtin method of
	// defining quote, quasiquote, and unquasiquote:
	registerQuoteMacroChars(fr)
	return fr
}

// ReadForm reads the next form in the input stream.
//
// If the end of the file is encountered, the second value will be io.EOF.
func (fr *FormReader) ReadForm() (Form, error) {
	for {
		form, err := fr.readFormEvenTrivial()
		if err != nil {
			return nil, err
		}
		if _, ok := form.(nonSubstantiveForm); ok {
			continue
		}
		return form, nil
	}
}

// readFormEvenTrivial returns a (Form, error), just like ReadForm(), but it
// also reads comments and whitespace.
func (fr *FormReader) readFormEvenTrivial() (Form, error) {
	r, err := fr.src.peekRune()
	if err != nil {
		return nil, err
	}
	handler := fr.readTable[r]

	if handler != nil {
		result, err := handler(fr)
		if err != nil {
			return nil, err
		}
		if !result.Skip() {
			return result.Form(), nil
		}
	}

	switch r {
	case ' ', '\t', '\n':
		return fr.readWhitespace()
	case '"':
		return fr.readString()
	case '(':
		return fr.readList()
	default:
		return fr.readNumberSymbolOrComment()
	}
}

func (fr *FormReader) errorfWithPosition(format string, arg ...interface{}) error {
	return fmt.Errorf("%s:%s: %w",
		fr.src.fileName(),
		fr.src.offsetToRowCol(fr.src.cursorOffset()).String(),
		fmt.Errorf(format, arg...))
}

func (fr *FormReader) errorfWithRangePosition(start, end cursorOffset, format string, arg ...interface{}) error {
	return fmt.Errorf("%s: %w",
		fr.makeSpan(start, end).String(),
		fmt.Errorf(format, arg...))
}

func (fr *FormReader) makeSpan(start, end cursorOffset) SourceSpan {
	return &simpleSourceSpan{fr.src.fileName(), fr.src.offsetToRowCol(start), fr.src.offsetToRowCol(end)}
}

func (fr *FormReader) readWhitespace() (Form, error) {
	start := fr.src.cursorOffset()
	literal := strings.Builder{}
loop:
	for {
		r, err := fr.src.readRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch r {
		case ' ', '\t', '\n':
			literal.WriteRune(r)
		default:
			if err := fr.src.unreadRune(); err != nil {
				return nil, err
			}
			break loop
		}
	}
	if literal.Len() == 0 {
		return nil, fr.errorfWithPosition("failed to consume any whitespace")
	}

	end := fr.src.cursorOffset()
	return fr.factory.NewWhitespaceForm(WhitespaceText(literal.String()), fr.makeSpan(start, end))
}

func (fr *FormReader) readString() (StringForm, error) {
	start := fr.src.cursorOffset()
	r, err := fr.src.readRune()
	if err != nil {
		return nil, err
	}
	if r != '"' {
		return nil, fr.errorfWithPosition("expected opening \", got %q", r)
	}

	value := strings.Builder{}
	value.WriteRune(r)
	prevCharWasEscape := false
loop:
	for {
		r, err := fr.src.readRune()
		if err == io.EOF {
			return nil, fr.errorfWithPosition("did not find end of string token '\"'")
		}
		if err != nil {
			return nil, err
		}
		switch r {
		case '\n':
			value.WriteString(`\n`) // Replace newline literals with the literal `\n`, which is supported by strconv.Unquote.
			//return nil, fr.errorfWithPosition("unexpected newline before terminating \" character", src)
		case '"':
			if !prevCharWasEscape {
				value.WriteRune('"')
				break loop
			}
			fallthrough
		default:
			value.WriteRune(r)
		}
		prevCharWasEscape = (r == '\\')
	}
	parsedValue, err := strconv.Unquote(value.String())
	if err != nil {
		return nil, fr.errorfWithPosition("error parsing string - %w", err)
	}

	end := fr.src.cursorOffset()
	return fr.factory.NewStringForm(parsedValue, fr.makeSpan(start, end))
}

func (fr *FormReader) readList() (ListForm, error) {
	start := fr.src.cursorOffset()
	r, err := fr.src.readRune()
	if err != nil {
		return nil, err
	}
	if r != '(' {
		return nil, fr.errorfWithPosition("expected opening paren, got %q", r)
	}

	var forms []Form
loop:
	for {
		r, err := fr.src.readRune()
		if err == io.EOF {
			return nil, fr.errorfWithPosition("did not find end of list token ')'")
		}
		if err != nil {
			return nil, err
		}
		switch r {
		case ')':
			break loop
		default:
			if err := fr.src.unreadRune(); err != nil {
				return nil, err
			}
			f, err := fr.readFormEvenTrivial()
			if err != nil {
				return nil, err
			}
			forms = append(forms, f)
		}
	}

	end := fr.src.cursorOffset()
	return fr.factory.NewListForm(forms, fr.makeSpan(start, end))
}

func (fr *FormReader) readNumberSymbolOrComment() (Form, error) {
	start := fr.src.cursorOffset()
	literal := strings.Builder{}
loop:
	for {
		r, err := fr.src.readRune()
		if err == io.EOF {
			break loop
		}
		if err != nil {
			return nil, err
		}
		switch r {
		case ' ', '\t', '\n', ')', '(', '"':
			if err := fr.src.unreadRune(); err != nil {
				return nil, err
			}
			break loop
		case '/':
			if literal.Len() == 0 {
				// could be a comment
				commentForm, isComment, err := fr.readPossibleComment(start)
				if err != nil {
					return nil, err
				}
				if isComment {
					return commentForm, nil
				}
			}

			// Could have come to the end of a symbol and start of a comment, or a /
			// is in the middle of the symbol.
			r2, err := fr.src.readRune()
			if err == io.EOF {
				break loop
			}
			if err := fr.src.unreadRune(); err != nil {
				return nil, err
			}
			startOfComment := r2 == '/' || r2 == '*'
			if startOfComment {
				// Start of comment... unread r as well and return.
				if err := fr.src.unreadRune(); err != nil {
					return nil, err
				}
				break loop
			}
		default:
			literal.WriteRune(r)
		}
	}
	if literal.Len() == 0 {
		return nil, fr.errorfWithPosition("failed to consume a token")
	}

	end := fr.src.cursorOffset()
	return fr.makeFormFromSymbolOrNumberToken(literal.String(), start, end)
}

func (fr *FormReader) makeFormFromSymbolOrNumberToken(token string, start, end cursorOffset) (Form, error) {
	num, err := parseNumber(token)
	if err != nil {
		return nil, fr.errorfWithRangePosition(start, end, "bad number: %w", err)
	}
	if num == nil {
		return fr.factory.NewSymbolForm(token, fr.makeSpan(start, end))
	}
	return fr.factory.NewNumberForm(num.constValue(), fr.makeSpan(start, end))
}

func (fr *FormReader) makeFormFromReaderRequiredSymbol(rr readerRequiredSymbol, start, end cursorOffset) (Form, error) {
	// This could be swapped out for some other symbol construction method.
	return fr.factory.NewSymbolForm(string(rr), fr.makeSpan(start, end))
}

// Callsed after reading a '/' rune.
func (fr *FormReader) readPossibleComment(start cursorOffset) (CommentForm, bool, error) {
	contents := strings.Builder{}
	contents.WriteRune('/')
	secondCommentRune, err := fr.src.readRune()
	if err != nil {
		return nil, false, err
	}
	contents.WriteRune(secondCommentRune)
	switch secondCommentRune {
	case '*':
		// Consume the contents of the multi-line comment.
		for {
			r, err := fr.src.readRune()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, false, err
			}
			contents.WriteRune(r)
			if r != '*' {
				continue
			}
			r2, err := fr.src.readRune()
			contents.WriteRune(r2)
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, false, err
			}
			if r2 == '/' {
				break
			}
		}
		end := fr.src.cursorOffset()
		c, err := fr.factory.NewCommentForm(CommentText(contents.String()), fr.makeSpan(start, end))
		if err != nil {
			return nil, false, err
		}
		return c, true, nil
	case '/':
		for {
			r, err := fr.src.readRune()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, false, err
			}
			if r == '\n' {
				break
			}
			contents.WriteRune(r)
		}
		end := fr.src.cursorOffset()
		c, err := fr.factory.NewCommentForm(CommentText(contents.String()), fr.makeSpan(start, end))
		if err != nil {
			return nil, false, err
		}
		return c, true, nil
	default:
		// Unread last character.
		return nil, false, fr.src.unreadRune()
	}
}

func registerQuoteMacroChars(fr *FormReader) {
	fr.readTable['\''] = func(fr *FormReader) (ReaderMacroResult, error) {
		start := fr.src.cursorOffset()
		if _, err := fr.src.readRune(); err != nil {
			return nil, fr.errorfWithPosition("I/O failure reading quote character after peek: %w", err)
		}
		afterQuote := fr.src.cursorOffset()
		quoteSym, err := fr.makeFormFromReaderRequiredSymbol(quoteSymbol, start, afterQuote)
		if err != nil {
			return nil, err
		}

		quotedValue, err := fr.ReadForm()
		if err == io.EOF {
			return nil, fr.errorfWithPosition("expecting form after quote character, got EOF")
		}
		if err != nil {
			return nil, err
		}
		end := fr.src.cursorOffset()
		f, err := fr.factory.NewListForm([]Form{quoteSym, quotedValue}, fr.makeSpan(start, end))
		if err != nil {
			return nil, err
		}
		return &simpleReaderMacroResult{
			skip: false,
			form: f,
		}, nil
	}
}
