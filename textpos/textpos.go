// Package textpos provides types and functions for working with line-based
// positions of text in a textual document.
//
// WARNING: This package's API is in flux. It is based on the "go/token" package.
package textpos

import "fmt"

// Line is the line number of some text in a file.
type Line struct {
	value int
}

// LineFromOffset returns a Line object from an offset value.
func LineFromOffset(o int) Line { return LineFromOrdinal(o + 1) }

// LineFromOrdinal returns a Line object from a positive value.
func LineFromOrdinal(o int) Line { return Line{o} }

// Offset returns the line number where 0 indicates the first line.
func (n Line) Offset() int { return n.Ordinal() - 1 }

// Ordinal returns the line number where 1 indicates the first line.
func (n Line) Ordinal() int { return n.value }

// String returns the ordinal value encoded as a base 10 string.
func (n Line) String() string { return fmt.Sprintf("%d", n.Ordinal()) }

// IsValid reports if the line value is valid (ordinal >= 1).
func (n Line) IsValid() bool { return n.Ordinal() > 0 }

// Column is a number indicating a horrizontal offset within a line of text.
//
// Column may be used to designate byte or character offsets. Byte offsets are
// advantageous because they are simple well-defined but match cursor positions
// in most text editors. Characters are not a universally well-defined and
// require more complex lookup but generally correspond to cursor positions in
// text editors.
type Column struct {
	value int
}

// ColumnFromOffset returns a Column object from an offset value (where 0 indicates the first line).
func ColumnFromOffset(o int) Column { return ColumnFromOrdinal(o + 1) }

// ColumnFromOrdinal returns a Column object from an ordinal value (where 1 indicates the first line).
func ColumnFromOrdinal(o int) Column { return Column{o} }

// Offset returns the Column number where 0 indicates the first Column.
func (n Column) Offset() int { return n.Ordinal() - 1 }

// Ordinal returns the Column number where 1 indicates the first Column.
func (n Column) Ordinal() int { return n.value }

// String returns the ordinal value encoded as a base 10 string.
func (n Column) String() string { return fmt.Sprintf("%d", n.Ordinal()) }

// IsValid reports if the column value is valid (ordinal >= 1).
func (n Column) IsValid() bool { return n.Ordinal() > 0 }

// LineColumn is a two dimensional textual position (line, column).
type LineColumn struct {
	line Line
	col  Column
}

// MakeLineColumn returns a new LineColumn tuple.
func MakeLineColumn(line Line, col Column) LineColumn {
	return LineColumn{line, col}
}

// Line returns the line for the tuple.
func (p LineColumn) Line() Line { return p.line }

// Column returns the column for the tuple.
func (p LineColumn) Column() Column { return p.col }

// String returns a string representation of a LineColumn pair.
//
// If column and line are valid, returns "lineOrdinal:columnOrdinal."
func (p LineColumn) String() string {
	l, c := "-", "-"
	if p.Line().IsValid() {
		l = fmt.Sprintf("%d", p.Line().Ordinal())
	}
	if p.Line().IsValid() {
		c = fmt.Sprintf("%d", p.Column().Ordinal())
	}
	return fmt.Sprintf("%s:%s", l, c)
}
