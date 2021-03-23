package sexpr

import (
	"fmt"
	"io"
)

type sourceFile interface {
	fileName() string
	readRune() (rune, error)
	peekRune() (rune, error)
	unreadRune() error
	cursorOffset() cursorOffset
	offsetToRowCol(cursorOffset) rowCol
	rowColToOffset(rc rowCol) cursorOffset
}

type cursorOffset int

// lineNumber is a line number in a text file. The first line in a file has line
// number 0.
type lineNumber int

// Offset returns the line number as an int where 0 corresponds to the first line.
func (n lineNumber) Offset() int {
	return int(n)
}

// columnNumber is a column number in a text file. The first column in a file
// has columnNumber 0.
type columnNumber int

// Offset returns the column number as an int where 0 corresponds to the first column.
func (n columnNumber) Offset() int {
	return int(n)
}

type rowCol struct {
	row lineNumber
	col columnNumber
}

func invalidRowCol() rowCol { return rowCol{-1, -1} }

const (
	invalidCursorOffset = -1
)

func (rc rowCol) String() string {
	return fmt.Sprintf("%d:%d", rc.row.Offset()+1, rc.col.Offset()+1)
}

func (co cursorOffset) int() int { return int(co) }

type strSourceFile struct {
	name           string
	cursor         cursorOffset
	runes          []rune
	newlineOffsets []cursorOffset
}

func newStrSourceFile(name, code string) *strSourceFile {
	runes := []rune(code)
	var newlineOffsets []cursorOffset
	for i, r := range runes {
		if r == '\n' {
			newlineOffsets = append(newlineOffsets, cursorOffset(i))
		}
	}
	return &strSourceFile{name, 0, runes, newlineOffsets}
}

func (sf *strSourceFile) fileName() string {
	return sf.name
}
func (sf *strSourceFile) readRune() (rune, error) {
	if int(sf.cursor) == len(sf.runes) {
		return 0, io.EOF
	}
	r := sf.runes[sf.cursor]
	sf.cursor++
	return r, nil
}

func (sf *strSourceFile) peekRune() (rune, error) {
	if int(sf.cursor) == len(sf.runes) {
		return 0, io.EOF
	}
	r := sf.runes[sf.cursor]
	return r, nil
}

func (sf *strSourceFile) unreadRune() error {
	if sf.cursor == 0 {
		return io.EOF
	}
	sf.cursor--
	return nil
}

func (sf *strSourceFile) cursorOffset() cursorOffset {
	return sf.cursor
}

func (sf *strSourceFile) offsetToRowCol(co cursorOffset) rowCol {
	if co < 0 || int(co) > len(sf.runes) {
		return invalidRowCol()
	}
	for i, endOfLineOffset := range sf.newlineOffsets {
		line := lineNumber(i)
		if co > endOfLineOffset {
			continue
		}
		return rowCol{line, columnNumber(co - sf.lineStart(line))}
	}
	lastLine := lineNumber(len(sf.newlineOffsets))
	return rowCol{
		lastLine,
		columnNumber(co - sf.lineStart(lastLine)),
	}
}

func (sf *strSourceFile) rowColToOffset(rc rowCol) cursorOffset {
	start := sf.lineStart(rc.row)
	if start == invalidCursorOffset {
		return invalidCursorOffset
	}
	ll := sf.lineLength(rc.row)
	if rc.col.Offset() > ll {
		return invalidCursorOffset
	}
	return cursorOffset(int(start) + rc.col.Offset())
}

func (sf *strSourceFile) lineLength(l lineNumber) int {
	start := sf.lineStart(l)
	if start == invalidCursorOffset {
		return -1
	}
	nextStart := sf.lineStart(l + 1)
	if nextStart == invalidCursorOffset {
		return len(sf.runes) - start.int()
	}
	return nextStart.int() - start.int() - 1
}

func (sf *strSourceFile) lineLengths() []int {
	var ret []int
	for i := lineNumber(0); i.Offset() <= len(sf.newlineOffsets); i++ {
		ret = append(ret, sf.lineLength(i))
	}
	return ret
}

func (sf *strSourceFile) lineStart(l lineNumber) cursorOffset {
	if l.Offset() == 0 {
		return 0
	}
	if l.Offset()-1 >= len(sf.newlineOffsets) || l.Offset() < 0 {
		return invalidCursorOffset
	}
	return sf.newlineOffsets[l.Offset()-1] + 1
}

func (sf *strSourceFile) lineStarts() []cursorOffset {
	var ret []cursorOffset
	for i := lineNumber(0); i.Offset() <= len(sf.newlineOffsets); i++ {
		ret = append(ret, sf.lineStart(i))
	}
	return ret
}

// SourceSpan is a continuous interval of positions within a text file.
type SourceSpan interface {
	// FileName is the name of the source file.
	FileName() string
	// String is a concise, human-readable representation of the span suitable
	// for printing in error messages.
	String() string
	start() rowCol
	end() rowCol
}

type simpleSourceSpan struct {
	name string
	s, e rowCol
}

func (span *simpleSourceSpan) FileName() string {
	return span.name
}

func (span *simpleSourceSpan) String() string {
	rangePart := fmt.Sprintf("%s-", span.start())
	if span.start().row == span.end().row {
		rangePart += fmt.Sprintf("%d", span.end().col.Offset()+1)
	} else {
		rangePart += span.end().String()
	}
	return fmt.Sprintf("%s:%s", span.FileName(), rangePart)
}

func (span *simpleSourceSpan) start() rowCol {
	return span.s
}

func (span *simpleSourceSpan) end() rowCol {
	return span.e
}
