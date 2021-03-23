// Package form is an API for working with S-Expression objects.
package form

import "go/constant"

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
