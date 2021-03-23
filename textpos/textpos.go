// Package textpos provides types and functions for working with intervals of
// text in a textual document.
package textpos

import "fmt"

// Line is the relative line number of some text.
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

// Column is the relative column number of some text.
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
