package form

import "github.com/google/xtoproto/textpos"

// SourcePosition is a continuous interval of positions within a text file.
type SourcePosition interface {
	IsValid() bool
	Range() *textpos.PositionRange
}
