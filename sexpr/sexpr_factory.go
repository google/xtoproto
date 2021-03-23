package sexpr

import (
	"go/constant"
)

// FormFactory is used by FormReader to construct Form values as it
// processes the input stream. This allows customization of the concrete types
// used to implement the Form interface.
//
// A default FormFactory is provided by DefaultFormFactory().
type FormFactory interface {
	// NewListForm returns a new ListForm for the provided subforms.
	//
	// The passed forms slice contains whitespace and comment forms that should
	// not be part of the value used to implement the ListForm interface.
	NewListForm(forms []Form, span SourceSpan) (ListForm, error)

	// NewNumberForm returns a new NumberForm for the provided literal
	// representation.
	//
	// value is guaranteed to be one of the numeric values defined in the
	// constant package.
	NewNumberForm(value constant.Value, span SourceSpan) (NumberForm, error)

	// NewSymbolForm returns a new Form for the provided symbol literal representation.
	NewSymbolForm(literal string, span SourceSpan) (Form, error)

	// NewStringForm returns a new StringForm for the provided literal representation.
	NewStringForm(value string, span SourceSpan) (StringForm, error)

	// NewCommentForm returns a new CommentForm for the provided literal representation.
	NewCommentForm(value CommentText, span SourceSpan) (CommentForm, error)

	// NewWhitespaceForm returns a new WhitespaceForm for the provided literal representation.
	NewWhitespaceForm(value WhitespaceText, span SourceSpan) (WhitespaceForm, error)
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

func (f *defaultFactory) NewListForm(forms []Form, span SourceSpan) (ListForm, error) {
	var valueForms []Form
	for _, f := range forms {
		if _, ok := f.(ValuelessForm); ok {
			continue
		}
		valueForms = append(valueForms, f)
	}
	return &listForm{valueForms, forms, span}, nil
}

// NewCommentForm returns a new CommentForm for the provided literal representation.
func (f *defaultFactory) NewCommentForm(value CommentText, span SourceSpan) (CommentForm, error) {
	return &commentForm{value, span}, nil
}

// NewWhitespaceForm returns a new WhitespaceForm for the provided literal representation.
func (f *defaultFactory) NewWhitespaceForm(value WhitespaceText, span SourceSpan) (WhitespaceForm, error) {
	return &whitespaceForm{value, span}, nil
}

func (f *defaultFactory) NewNumberForm(value constant.Value, span SourceSpan) (NumberForm, error) {
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

func (f *stringForm) SourceSpan() SourceSpan {
	return f.span
}

func (f *stringForm) Value() interface{} {
	return f.StringValue()
}

func (f *stringForm) StringValue() string {
	return f.val
}

// listForm is a list expression. It is the result of reading a form string like
// `(abc 123)`.
//
// The Value() of the list is the list of substantive subforms that comprise the
// list, which may also be obtained without casting by calling Subforms().
type listForm struct {
	val                          []Form
	valWithCommentsAndWhitespace []Form
	span                         SourceSpan
}

// SourceSpan returns the source code location location of the form.
func (f *listForm) SourceSpan() SourceSpan {
	return f.span
}

// Value returns the underlying value of the S-expression.
func (f *listForm) Value() interface{} {
	return f.Subforms()
}

// Subforms returns the ordered list of forms that comprise the list.
//
// The list of forms should be substantive.
func (f *listForm) Subforms() []Form {
	return f.val
}

// Len returns the length of the list. It is equivalent to len(Subforms())
// but may be more efficient.
func (f *listForm) Len() int {
	return len(f.val)
}

// Nth returns Subforms()[n]. It may panic if the length of the list is
// <= n.
func (f *listForm) Nth(n int) Form {
	return f.val[n]
}

type commentForm struct {
	literal CommentText
	span    SourceSpan
}

func (f *commentForm) SourceSpan() SourceSpan {
	return f.span
}

func (f *commentForm) Value() interface{} {
	return nil
}

func (f *commentForm) Comment() CommentText {
	return f.literal
}

func (f *commentForm) Valueless() {}

func (f *commentForm) isNonSubstantive() {}

type whitespaceForm struct {
	literal WhitespaceText
	span    SourceSpan
}

func (f *whitespaceForm) SourceSpan() SourceSpan {
	return f.span
}

func (f *whitespaceForm) Value() interface{} {
	return nil
}

func (f *whitespaceForm) Valueless() {}

func (f *whitespaceForm) Whitespace() WhitespaceText {
	return f.literal
}

func (f *whitespaceForm) isNonSubstantive() {}

type symbolForm struct {
	literal string
	span    SourceSpan
}

func (f *symbolForm) SourceSpan() SourceSpan {
	return f.span
}

func (f *symbolForm) Value() interface{} {
	return nil
}

type numberForm struct {
	val  constant.Value
	span SourceSpan
}

func (f *numberForm) SourceSpan() SourceSpan {
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

// NumberValue returns the number using go's constant package. The
// value should be one of `int64, *big.Int, *big.Float, *big.Rat`.
func (f *numberForm) NumberValue() constant.Value {
	return f.val
}
