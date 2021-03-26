package form

import "github.com/google/xtoproto/textpos"

// SourcePosition is a continuous interval of positions within a text file.
type SourcePosition interface {
	Range() *textpos.Range

	// String returns a human readable representation of the source position.
	String() string
}
