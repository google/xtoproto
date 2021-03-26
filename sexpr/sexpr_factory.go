package sexpr

import (
	"go/constant"

	"github.com/google/xtoproto/sexpr/form"
)

// FormFactory is used by FormReader to construct Form values as it
// processes the input stream. This allows customization of the concrete types
// used to implement the Form interface.
//
// A default FormFactory is provided by DefaultFormFactory().
type FormFactory interface {
	// NewList returns a new form.List for the provided subforms.
	//
	// The passed forms slice contains whitespace and comment forms that should
	// not be part of the value used to implement the form.List interface.
	NewList(forms []form.Form, span SourceSpan) (form.List, error)

	// NewNumberForm returns a new NumberForm for the provided literal
	// representation.
	//
	// value is guaranteed to be one of the numeric values defined in the
	// constant package.
	NewNumberForm(value constant.Value, span SourceSpan) (form.Number, error)

	// NewSymbolForm returns a new Form for the provided symbol literal representation.
	NewSymbolForm(literal string, span SourceSpan) (Form, error)

	// NewStringForm returns a new StringForm for the provided literal representation.
	NewStringForm(value string, span SourceSpan) (StringForm, error)

	// NewCommentForm returns a new CommentForm for the provided literal representation.
	NewCommentForm(value string, span SourceSpan) (CommentForm, error)

	// NewWhitespaceForm returns a new WhitespaceForm for the provided literal representation.
	NewWhitespaceForm(value string, span SourceSpan) (WhitespaceForm, error)
}

// DefaultFormFactory returns a FormConstructor that creates Form objects
// using unexported types within the sexpr package.
func DefaultFormFactory(r *FormReader) FormFactory {
	return &defaultFactory{r}
}

type defaultFactory struct {
	r *FormReader
}

func (f *defaultFactory) NewStringForm(value string, span SourceSpan) (StringForm, error) {
	return &stringForm{value, span}, nil
}

func (f *defaultFactory) NewList(forms []Form, span SourceSpan) (form.List, error) {
	var valueForms []Form
	for _, f := range forms {
		if _, ok := f.(ValuelessForm); ok {
			continue
		}
		valueForms = append(valueForms, f)
	}
	return &list{valueForms, forms, span}, nil
}

// NewCommentForm returns a new CommentForm for the provided literal representation.
func (f *defaultFactory) NewCommentForm(value string, span SourceSpan) (CommentForm, error) {
	return &commentForm{value, span}, nil
}

// NewWhitespaceForm returns a new WhitespaceForm for the provided literal representation.
func (f *defaultFactory) NewWhitespaceForm(value string, span SourceSpan) (WhitespaceForm, error) {
	return &whitespaceForm{value, span}, nil
}

func (f *defaultFactory) NewNumberForm(value constant.Value, span SourceSpan) (form.Number, error) {
	return &numberForm{value, span}, nil
}

func (f *defaultFactory) NewSymbolForm(value string, span SourceSpan) (Form, error) {
	return &symbolForm{value, span}, nil
}

type nonSubstantiveForm interface {
	isNonSubstantive()
}

type stringForm struct {
	val  string
	span SourceSpan
}

func (f *stringForm) SourcePosition() form.SourcePosition {
	return f.span
}

func (f *stringForm) Value() interface{} {
	return f.StringValue()
}

func (f *stringForm) StringValue() string {
	return f.val
}

// list is a list expression. It is the result of reading a form string like
// `(abc 123)`.
//
// The Value() of the list is the list of substantive subforms that comprise the
// list, which may also be obtained without casting by calling Subforms().
type list struct {
	val                          []form.Form
	valWithCommentsAndWhitespace []form.Form
	span                         SourceSpan
}

// SourcePosition returns the source code location location of the form.
func (f *list) SourcePosition() form.SourcePosition {
	return f.span
}

// Value returns the underlying value of the S-expression.
func (f *list) Value() interface{} {
	return f.Subforms()
}

// Subforms returns the ordered list of forms that comprise the list.
//
// The list of forms should be substantive.
func (f *list) Subforms() []Form {
	return f.val
}

// Len returns the length of the list. It is equivalent to len(Subforms())
// but may be more efficient.
func (f *list) Len() int {
	return len(f.val)
}

// Nth returns Subforms()[n]. It may panic if the length of the list is
// <= n.
func (f *list) Nth(n int) Form {
	return f.val[n]
}

type commentForm struct {
	literal string
	span    SourceSpan
}

func (f *commentForm) SourcePosition() form.SourcePosition {
	return f.span
}

func (f *commentForm) Value() interface{} {
	return nil
}

func (f *commentForm) CommentLiteral() string {
	return f.literal
}

func (f *commentForm) Valueless() {}

func (f *commentForm) isNonSubstantive() {}

type whitespaceForm struct {
	literal string
	span    SourceSpan
}

func (f *whitespaceForm) SourcePosition() form.SourcePosition {
	return f.span
}

func (f *whitespaceForm) Value() interface{} {
	return nil
}

func (f *whitespaceForm) Valueless() {}

func (f *whitespaceForm) Whitespace() string {
	return f.literal
}

func (f *whitespaceForm) isNonSubstantive() {}

type symbolForm struct {
	literal string
	span    SourceSpan
}

func (f *symbolForm) SourcePosition() SourceSpan {
	return f.span
}

func (f *symbolForm) Value() interface{} {
	return nil
}

type numberForm struct {
	val  constant.Value
	span SourceSpan
}

func (f *numberForm) SourcePosition() SourceSpan {
	return f.span
}

func (f *numberForm) Value() interface{} {
	switch f.val.Kind() {
	case constant.Float:
		if v, exact := constant.Float64Val(f.val); exact {
			return v
		}
		fallthrough
	default:
		return constant.Val(f.val)
	}
}

// Number returns the number using go's constant package. The
// value should be one of `int64, *big.Int, *big.Float, *big.Rat`.
func (f *numberForm) Number() constant.Value {
	return f.val
}
