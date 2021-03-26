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
	SourcePosition() SourcePosition
	Value() interface{}
}

// List implements Form for a form comprised of an ordered list of subforms.
//
// The s-expression `("abc" xyz)` would be expected to be read as a List
// with two subforms.
type List interface {
	Form

	// Len returns the length of the list. It is equivalent to len(Subforms())
	// but may be more efficient.
	Len() int

	// Nth returns Subforms()[n]. It may panic if the length of the list is
	// <= n.
	Nth(n int) Form
}

// Subforms returns the ordered list of forms that comprise the list.
//
// The list of forms should be substantive.
func Subforms(f List) []Form {
	var out []Form
	for i := 0; i < f.Len(); i++ {
		out = append(out, f.Nth(i))
	}
	return out
}

// String is an interface for a form with an underlying string literal
// representation.
//
// This method should not be implemented by other types of forms, even if they
// can be represented as a string. For example, a number should implemented
// Number and not String.
type String interface {
	Form

	// StringValue returns the value of the form as a string literal.
	StringValue() string
}

// Number is an interface for a form with an underlying number literal.
//
// A Number is roughly equivalent to an untyped number const in Go.
type Number interface {
	Form

	// Number returns the number using go's constant package. The
	// value should be one of `int64, *big.Int, *big.Float, *big.Rat`.
	Number() constant.Value
}

// Symbol is an interface for a form with an underlying Symbol literal.
//
// A Symbol is roughly equivalent to an untyped Symbol const in Go.
type Symbol interface {
	Form

	// SymbolLiteral returns the symbol as a string. This may be different from the
	// symbol as it appeared in the source text if the reader changes the
	// literal form using some sort of normalization.
	SymbolLiteral() string
}

// Valueless is a form that should be ignored in most contexts. Examples of
// valueless forms are comments and whitespace.
type Valueless interface {
	Form

	// The Valueless function indicates that this form should be ignored
	// when read in most contexts. Examples of valueless forms are
	Valueless()
}

// Comment is an interface for a form with an underlying comment literal.
type Comment interface {
	Valueless

	// Comment returns the comment literal including delimiters.
	//
	// Note that multiple Comments may comprise a single logical comment block
	// separated by whitespace, as the whitespace forms will be parsed
	// separately.
	CommentLiteral() string
}

// Whitespace is an interface for a form with an underlying string literal
// representation.
type Whitespace interface {
	Valueless

	// Whitespace returns the whiespace literal.
	Whitespace() string
}
