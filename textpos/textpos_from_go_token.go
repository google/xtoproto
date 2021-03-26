package textpos

import (
	"fmt"
	"sort"
	"sync"
)

// This file contains code based on the go/token package without any Go specific
// constructs.

// -----------------------------------------------------------------------------
// FileSet

// A FileSet represents a set of source files.
// Methods of file sets are synchronized; multiple goroutines
// may invoke them concurrently.
//
// The byte offsets for each file in a file set are mapped into
// distinct (integer) intervals, one interval [base, base+size]
// per file. Base represents the first byte in the file, and size
// is the corresponding file size. A Pos value is a value in such
// an interval. By determining the interval a Pos value belongs
// to, the file, its file base, and thus the byte offset (position)
// the Pos value is representing can be computed.
//
// When adding a new file, a file base must be provided. That can
// be any integer value that is past the end of any interval of any
// file already in the file set. For convenience, FileSet.Base provides
// such a value, which is simply the end of the Pos interval of the most
// recently added file, plus one. Unless there is a need to extend an
// interval later, using the FileSet.Base should be used as argument
// for FileSet.AddFile.
//
type FileSet struct {
	mutex sync.RWMutex // protects the file set
	base  int          // base offset for the next file
	files []*File      // list of files in the order added to the set
	last  *File        // cache of last file looked up
}

// NewFileSet creates a new file set.
func NewFileSet() *FileSet {
	return &FileSet{
		base: 1, // 0 == NoPos
	}
}

// Base returns the minimum base offset that must be provided to
// AddFile when adding the next file.
//
func (s *FileSet) Base() int {
	s.mutex.RLock()
	b := s.base
	s.mutex.RUnlock()
	return b

}

// AddFile adds a new file with a given filename, base offset, and file size
// to the file set s and returns the file. Multiple files may have the same
// name. The base offset must not be smaller than the FileSet's Base(), and
// size must not be negative. As a special case, if a negative base is provided,
// the current value of the FileSet's Base() is used instead.
//
// Adding the file will set the file set's Base() value to base + size + 1
// as the minimum base value for the next file. The following relationship
// exists between a Pos value p for a given file offset offs:
//
//	int(p) = base + offs
//
// with offs in the range [0, size] and thus p in the range [base, base+size].
// For convenience, File.Pos may be used to create file-specific position
// values from a file offset.
//
func (s *FileSet) AddFile(filename string, base, size int) *File {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if base < 0 {
		base = s.base
	}
	if base < s.base {
		panic(fmt.Sprintf("invalid base %d (should be >= %d)", base, s.base))
	}
	if size < 0 {
		panic(fmt.Sprintf("invalid size %d (should be >= 0)", size))
	}
	// base >= s.base && size >= 0
	f := &File{set: s, name: filename, base: base, size: size, lines: []int{0}}
	base += size + 1 // +1 because EOF also has a position
	if base < 0 {
		panic("token.Pos offset overflow (> 2G of source code in file set)")
	}
	// add the file to the file set
	s.base = base
	s.files = append(s.files, f)
	s.last = f
	return f
}

// Iterate calls f for the files in the file set in the order they were added
// until f returns false.
//
func (s *FileSet) Iterate(f func(*File) bool) {
	for i := 0; ; i++ {
		var file *File
		s.mutex.RLock()
		if i < len(s.files) {
			file = s.files[i]
		}
		s.mutex.RUnlock()
		if file == nil || !f(file) {
			break
		}
	}
}

func searchFiles(a []*File, x int) int {
	return sort.Search(len(a), func(i int) bool { return a[i].base > x }) - 1
}

func (s *FileSet) file(p Pos) *File {
	s.mutex.RLock()
	// common case: p is in last file
	if f := s.last; f != nil && f.base <= int(p) && int(p) <= f.base+f.size {
		s.mutex.RUnlock()
		return f
	}
	// p is not in last file - search all files
	if i := searchFiles(s.files, int(p)); i >= 0 {
		f := s.files[i]
		// f.base <= int(p) by definition of searchFiles
		if int(p) <= f.base+f.size {
			s.mutex.RUnlock()
			s.mutex.Lock()
			s.last = f // race is ok - s.last is only a cache
			s.mutex.Unlock()
			return f
		}
	}
	s.mutex.RUnlock()
	return nil
}

// File returns the file that contains the position p.
// If no such file is found (for instance for p == NoPos),
// the result is nil.
//
func (s *FileSet) File(p Pos) (f *File) {
	if p != NoPos {
		f = s.file(p)
	}
	return
}

// positionFor converts a Pos p in the fileset into a Position value.
// If adjusted is set, the position may be adjusted by position-altering
// //line comments; otherwise those comments are ignored.
// p must be a Pos value in s or NoPos.
//
func (s *FileSet) positionFor(p Pos, adjusted bool) (pos Position) {
	if p != NoPos {
		if f := s.file(p); f != nil {
			return f.position(p, adjusted)
		}
	}
	return
}

// Position converts a Pos p in the fileset into a Position value.
// Calling s.Position(p) is equivalent to calling s.PositionFor(p, true).
//
func (s *FileSet) Position(p Pos) (pos Position) {
	return s.positionFor(p, true)
}

// -----------------------------------------------------------------------------
// File

// A File is a handle for a file belonging to a FileSet.
// A File has a name, size, and line offset table.
//
type File struct {
	set  *FileSet
	name string // file name as provided to AddFile
	base int    // Pos value range for this file is [base...base+size]
	size int    // file size as provided to AddFile

	// lines and infos are protected by mutex
	mutex sync.Mutex
	lines []int // lines contains the offset of the first character for each line (the first entry is always 0)
	infos []lineInfo
}

// Name returns the file name of file f as registered with AddFile.
func (f *File) Name() string {
	return f.name
}

// Base returns the base offset of file f as registered with AddFile.
func (f *File) Base() int {
	return f.base
}

// Size returns the size of file f as registered with AddFile.
func (f *File) Size() int {
	return f.size
}

// LineCount returns the number of lines in file f.
func (f *File) LineCount() int {
	f.mutex.Lock()
	n := len(f.lines)
	f.mutex.Unlock()
	return n
}

// AddLine adds the line offset for a new line.
// The line offset must be larger than the offset for the previous line
// and smaller than the file size; otherwise the line offset is ignored.
//
func (f *File) AddLine(offset int) {
	f.mutex.Lock()
	if i := len(f.lines); (i == 0 || f.lines[i-1] < offset) && offset < f.size {
		f.lines = append(f.lines, offset)
	}
	f.mutex.Unlock()
}

// MergeLine merges a line with the following line. It is akin to replacing
// the newline character at the end of the line with a space (to not change the
// remaining offsets). To obtain the line number, consult e.g. Position.Line.
// MergeLine will panic if given an invalid line number.
//
func (f *File) MergeLine(line int) {
	if line < 1 {
		panic(fmt.Sprintf("invalid line number %d (should be >= 1)", line))
	}
	f.mutex.Lock()
	defer f.mutex.Unlock()
	if line >= len(f.lines) {
		panic(fmt.Sprintf("invalid line number %d (should be < %d)", line, len(f.lines)))
	}
	// To merge the line numbered <line> with the line numbered <line+1>,
	// we need to remove the entry in lines corresponding to the line
	// numbered <line+1>. The entry in lines corresponding to the line
	// numbered <line+1> is located at index <line>, since indices in lines
	// are 0-based and line numbers are 1-based.
	copy(f.lines[line:], f.lines[line+1:])
	f.lines = f.lines[:len(f.lines)-1]
}

// SetLines sets the line offsets for a file and reports whether it succeeded.
// The line offsets are the offsets of the first character of each line;
// for instance for the content "ab\nc\n" the line offsets are {0, 3}.
// An empty file has an empty line offset table.
// Each line offset must be larger than the offset for the previous line
// and smaller than the file size; otherwise SetLines fails and returns
// false.
// Callers must not mutate the provided slice after SetLines returns.
//
func (f *File) SetLines(lines []int) bool {
	// verify validity of lines table
	size := f.size
	for i, offset := range lines {
		if i > 0 && offset <= lines[i-1] || size <= offset {
			return false
		}
	}

	// set lines table
	f.mutex.Lock()
	f.lines = lines
	f.mutex.Unlock()
	return true
}

// SetLinesForContent sets the line offsets for the given file content.
// It ignores position-altering //line comments.
func (f *File) SetLinesForContent(content []byte) {
	var lines []int
	line := 0
	for offset, b := range content {
		if line >= 0 {
			lines = append(lines, line)
		}
		line = -1
		if b == '\n' {
			line = offset + 1
		}
	}

	// set lines table
	f.mutex.Lock()
	f.lines = lines
	f.mutex.Unlock()
}

// LineStart returns the Pos value of the start of the specified line.
// It ignores any alternative positions set using AddLineColumnInfo.
// LineStart panics if the 1-based line number is invalid.
func (f *File) LineStart(line Line) Pos {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.lineStartWithMutexHeld(line)
}

func (f *File) lineStartWithMutexHeld(line Line) Pos {
	if !line.IsValid() {
		panic(fmt.Sprintf("invalid line %s", line))
	}
	if line.Ordinal() > len(f.lines) {
		panic(fmt.Sprintf("invalid line %s", line, len(f.lines)))
	}
	return Pos(f.base + f.lines[line.Offset()])
}

func (f *File) lineStartOrNoPosWithMutexHeld(line Line) Pos {
	if !line.IsValid() {
		return NoPos
	}
	if line.Ordinal() > len(f.lines) {
		return NoPos
	}
	return Pos(f.base + f.lines[line.Offset()])
}

// A lineInfo object describes alternative file, line, and column
// number information (such as provided via a //line directive)
// for a given file offset.
type lineInfo struct {
	// fields are exported to make them accessible to gob
	Offset       int
	Filename     string
	Line, Column int
}

// addLineInfo is like AddLineColumnInfo with a column = 1 argument.
// It is here for backward-compatibility for code prior to Go 1.11.
//
// deprecated: Part of the original Go implementation; not needed.
func (f *File) addLineInfo(offset int, filename string, line int) {
	f.addLineColumnInfo(offset, filename, line, 1)
}

// addLineColumnInfo adds alternative file, line, and column number
// information for a given file offset. The offset must be larger
// than the offset for the previously added alternative line info
// and smaller than the file size; otherwise the information is
// ignored.
//
// addLineColumnInfo is typically used to register alternative position
// information for line directives such as //line filename:line:column.
//
func (f *File) addLineColumnInfo(offset int, filename string, line, column int) {
	f.mutex.Lock()
	if i := len(f.infos); i == 0 || f.infos[i-1].Offset < offset && offset < f.size {
		f.infos = append(f.infos, lineInfo{offset, filename, line, column})
	}
	f.mutex.Unlock()
}

// Pos returns the Pos value for the given file offset;
// the offset must be <= f.Size().
// f.Pos(f.Offset(p)) == p.
//
func (f *File) Pos(offset int) Pos {
	if offset > f.size {
		panic(fmt.Sprintf("invalid file offset %d (should be <= %d)", offset, f.size))
	}
	return Pos(f.base + offset)
}

// Offset returns the offset for the given file position p;
// p must be a valid Pos value in that file.
// f.Offset(f.Pos(offset)) == offset.
//
func (f *File) Offset(p Pos) int {
	if err := f.checkPos(p); err != nil {
		panic(err)
	}
	return int(p) - f.base
}

func (f *File) checkPos(p Pos) error {
	if int(p) < f.base || int(p) > f.base+f.size {
		return fmt.Errorf("invalid Pos value %d (should be in [%d, %d[)", p, f.base, f.base+f.size)
	}
	return nil
}

// Line returns the line number for the given file position p;
// p must be a Pos value in that file or NoPos.
//
func (f *File) Line(p Pos) Line {
	return f.Position(p).Line()
}

func searchLineInfos(a []lineInfo, x int) int {
	return sort.Search(len(a), func(i int) bool { return a[i].Offset > x }) - 1
}

// unpack returns the filename and line and column number for a file offset.
// If adjusted is set, unpack will return the filename and line information
// possibly adjusted by //line comments; otherwise those comments are ignored.
//
func (f *File) unpack(offset int, adjusted bool) (filename string, lineColumn LineColumn) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.unpackWhileMutexHeld(offset, adjusted)
}

func (f *File) unpackWhileMutexHeld(offset int, adjusted bool) (filename string, lineColumn LineColumn) {
	var line, column int
	filename = f.name
	if i := searchInts(f.lines, offset); i >= 0 {
		line, column = i+1, offset-f.lines[i]+1
	}
	if adjusted && len(f.infos) > 0 {
		// few files have extra line infos
		if i := searchLineInfos(f.infos, offset); i >= 0 {
			alt := &f.infos[i]
			filename = alt.Filename
			if i := searchInts(f.lines, alt.Offset); i >= 0 {
				// i+1 is the line at which the alternative position was recorded
				d := line - (i + 1) // line distance from alternative position base
				line = alt.Line + d
				if alt.Column == 0 {
					// alternative column is unknown => relative column is unknown
					// (the current specification for line directives requires
					// this to apply until the next PosBase/line directive,
					// not just until the new newline)
					column = 0
				} else if d == 0 {
					// the alternative position base is on the current line
					// => column is relative to alternative column
					column = alt.Column + (offset - alt.Offset)
				}
			}
		}
	}
	return filename, MakeLineColumn(LineFromOrdinal(line), ColumnFromOrdinal(column))
}

func (f *File) position(p Pos, adjusted bool) (pos Position) {
	offset := int(p) - f.base
	pos.offset = offset
	pos.fileName, pos.lineColumn = f.unpack(offset, adjusted)
	return
}

// positionFor returns the Position value for the given file position p.
// If adjusted is set, the position may be adjusted by position-altering
// line comments; otherwise those comments are ignored.
// p must be a Pos value in f or NoPos.
//
func (f *File) positionFor(p Pos, adjusted bool) (pos Position) {
	if p != NoPos {
		if int(p) < f.base || int(p) > f.base+f.size {
			panic(fmt.Sprintf("invalid Pos value %d (should be in [%d, %d[)", p, f.base, f.base+f.size))
		}
		pos = f.position(p, adjusted)
	}
	return
}

// Position returns the Position value for the given file position p.
// Calling f.Position(p) is equivalent to calling f.PositionFor(p, true).
//
func (f *File) Position(p Pos) (pos Position) {
	return f.positionFor(p, true)
}

// PosForLineColumn returns the position of the given (line, column) pair in the
// file or NoPos if the (line, column) pair is out of bounds.
func (f *File) PosForLineColumn(lc LineColumn) Pos {
	if !lc.Line().IsValid() || !lc.Column().IsValid() {
		return NoPos
	}

	f.mutex.Lock()
	defer f.mutex.Unlock()

	lineStartPos := f.lineStartOrNoPosWithMutexHeld(lc.line)
	if lineStartPos == NoPos {
		return NoPos
	}
	finalPos := lineStartPos + Pos(lc.Column().Offset())
	if f.checkPos(finalPos) != nil {
		return NoPos
	}
	return finalPos
}

// Pos is a compact encoding of a source position within a file set.
// It can be converted into a Position for a more convenient, but much
// larger, representation.
//
// The Pos value for a given file is a number in the range [base, base+size],
// where base and size are specified when a file is added to the file set.
// The difference between a Pos value and the corresponding file base
// corresponds to the byte offset of that position (represented by the Pos value)
// from the beginning of the file. Thus, the file base offset is the Pos value
// representing the first byte in the file.
//
// To create the Pos value for a specific source offset (measured in bytes),
// first add the respective file to the current file set using FileSet.AddFile
// and then call File.Pos(offset) for that file. Given a Pos value p
// for a specific file set fset, the corresponding Position value is
// obtained by calling fset.Position(p).
//
// Pos values can be compared directly with the usual comparison operators:
// If two Pos values p and q are in the same file, comparing p and q is
// equivalent to comparing the respective source file offsets. If p and q
// are in different files, p < q is true if the file implied by p was added
// to the respective file set before the file implied by q.
//
type Pos int

// NoPos is the zero value for Pos; there is no file and line information
// associated with it, and NoPos.IsValid() is false. NoPos is always
// smaller than any other Pos value. The corresponding Position value
// for NoPos is the zero value for Position.
//
const NoPos Pos = 0

// IsValid reports whether the position is valid.
func (p Pos) IsValid() bool {
	return p != NoPos
}

// Range is a range within a single file.
//
// The range specifies all of the characters in the interval [r.Start(),
// r.End()).
type Range struct {
	f                            *File
	startInclusive, endExclusive Pos
}

// Start returns the start position of the range.
func (r *Range) Start() Position {
	return r.f.Position(r.StartPos())
}

// End returns the end position of the range.
func (r *Range) End() Position {
	return r.f.Position(r.EndPos())
}

// StartPos returns the start position of the range.
func (r *Range) StartPos() Pos {
	return r.startInclusive
}

// EndPos returns the end position of the range.
func (r *Range) EndPos() Pos {
	return r.endExclusive
}

// File returns the file within which the range is specified.
func (r *Range) File() *File {
	return r.f
}

// Position describes an arbitrary source position
// including the file, line, and column location.
// A Position is valid if the line number is > 0.
type Position struct {
	fileName   string     // filename, if any
	offset     int        // offset, starting at 0
	lineColumn LineColumn // line and column numbers, may be invalid
}

// FileName returns the fileName of the position.
func (p Position) FileName() string { return p.fileName }

// Offset returns the offset of the position.
func (p Position) Offset() int { return p.offset }

// Line returns the line of the position.
func (p Position) Line() Line { return p.lineColumn.Line() }

// Column returns the column of the position.
func (p Position) Column() Column { return p.lineColumn.Column() }

// IsValid reports whether the position is valid.
func (p Position) IsValid() bool { return p.Line().IsValid() }

// String returns a string in one of several forms:
//
//	file:line:column    valid position with file name
//	file:line           valid position with file name but no column (column == 0)
//	line:column         valid position without file name
//	line                valid position without file name and no column (column == 0)
//	file                invalid position with file name
//	-                   invalid position without file name
//
func (p Position) String() string {
	s := p.FileName()
	if p.IsValid() {
		if s != "" {
			s += ":"
		}
		s += fmt.Sprintf("%d", p.Line().Ordinal())
		if p.Column().IsValid() {
			s += fmt.Sprintf(":%d", p.Column().Ordinal())
		}
	}
	if s == "" {
		s = "-"
	}
	return s
}

// -----------------------------------------------------------------------------
// Helper functions

func searchInts(a []int, x int) int {
	// This function body is a manually inlined version of:
	//
	//   return sort.Search(len(a), func(i int) bool { return a[i] > x }) - 1
	//
	// With better compiler optimizations, this may not be needed in the
	// future, but at the moment this change improves the go/printer
	// benchmark performance by ~30%. This has a direct impact on the
	// speed of gofmt and thus seems worthwhile (2011-04-29).
	// TODO(gri): Remove this when compilers have caught up.
	i, j := 0, len(a)
	for i < j {
		h := i + (j-i)/2 // avoid overflow when computing h
		// i ≤ h < j
		if a[h] <= x {
			i = h + 1
		} else {
			j = h
		}
	}
	return i - 1
}
