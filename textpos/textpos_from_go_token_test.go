package textpos

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
)

func checkPos(t *testing.T, msg string, got, want Position) {
	if got.documentName != want.documentName {
		t.Errorf("%s: got filename = %q; want %q", msg, got.documentName, want.documentName)
	}
	if got.offset != want.offset {
		t.Errorf("%s: got offset = %d; want %d", msg, got.Offset(), want.Offset())
	}
	if got.Line() != want.Line() {
		t.Errorf("%s: got line = %d; want %d", msg, got.Line().Ordinal(), want.Line().Ordinal())
	}
	if got.Column() != want.Column() {
		t.Errorf("%s: got column = %d; want %d", msg, got.Column().Ordinal(), want.Column().Ordinal())
	}
}

func TestNoPos(t *testing.T) {
	if NoPos.IsValid() {
		t.Errorf("NoPos should not be valid")
	}
	var fset *FileSet
	checkPos(t, "nil NoPos", fset.Position(NoPos), Position{})
	fset = NewFileSet()
	checkPos(t, "fset NoPos", fset.Position(NoPos), Position{})
}

var tests = []struct {
	filename string
	source   []byte // may be nil
	size     int
	lines    []int
}{
	{"a", []byte{}, 0, []int{}},
	{"b", []byte("01234"), 5, []int{0}},
	{"c", []byte("\n\n\n\n\n\n\n\n\n"), 9, []int{0, 1, 2, 3, 4, 5, 6, 7, 8}},
	{"d", nil, 100, []int{0, 5, 10, 20, 30, 70, 71, 72, 80, 85, 90, 99}},
	{"e", nil, 777, []int{0, 80, 100, 120, 130, 180, 267, 455, 500, 567, 620}},
	{"f", []byte("package p\n\nimport \"fmt\""), 23, []int{0, 10, 11}},
	{"g", []byte("package p\n\nimport \"fmt\"\n"), 24, []int{0, 10, 11}},
	{"h", []byte("package p\n\nimport \"fmt\"\n "), 25, []int{0, 10, 11, 24}},
}

func linecol(lines []int, offs int) LineColumn {
	prevLineOffs := 0
	for line, lineOffs := range lines {
		if offs < lineOffs {
			return MakeLineColumn(LineFromOrdinal(line), ColumnFromOrdinal(offs-prevLineOffs+1))
		}
		prevLineOffs = lineOffs
	}
	return MakeLineColumn(LineFromOrdinal(len(lines)), ColumnFromOrdinal(offs-prevLineOffs+1))
}

func verifyPositions(t *testing.T, fset *FileSet, f *File, lines []int) {
	for offs := 0; offs < f.Size(); offs++ {
		p := f.Pos(offs)
		offs2 := f.Offset(p)
		if offs2 != offs {
			t.Errorf("%s, Offset: got offset %d; want %d", f.Name(), offs2, offs)
		}
		lineCol := linecol(lines, offs)
		msg := fmt.Sprintf("%s (offs = %d, p = %d)", f.Name(), offs, p)
		checkPos(t, msg, f.Position(f.Pos(offs)), Position{f.Name(), offs, lineCol})
		checkPos(t, msg, fset.Position(p), Position{f.Name(), offs, lineCol})
	}
}

func makeTestSource(size int, lines []int) []byte {
	src := make([]byte, size)
	for _, offs := range lines {
		if offs > 0 {
			src[offs-1] = '\n'
		}
	}
	return src
}

func TestPositions(t *testing.T) {
	const delta = 7 // a non-zero base offset increment
	fset := NewFileSet()
	for _, test := range tests {
		// verify consistency of test case
		if test.source != nil && len(test.source) != test.size {
			t.Errorf("%s: inconsistent test case: got file size %d; want %d", test.filename, len(test.source), test.size)
		}

		// add file and verify name and size
		f := fset.AddFile(test.filename, fset.Base()+delta, test.size)
		if f.Name() != test.filename {
			t.Errorf("got filename %q; want %q", f.Name(), test.filename)
		}
		if f.Size() != test.size {
			t.Errorf("%s: got file size %d; want %d", f.Name(), f.Size(), test.size)
		}
		if fset.File(f.Pos(0)) != f {
			t.Errorf("%s: f.Pos(0) was not found in f", f.Name())
		}

		// add lines individually and verify all positions
		for i, offset := range test.lines {
			f.AddLine(offset)
			if f.LineCount() != i+1 {
				t.Errorf("%s, AddLine: got line count %d; want %d", f.Name(), f.LineCount(), i+1)
			}
			// adding the same offset again should be ignored
			f.AddLine(offset)
			if f.LineCount() != i+1 {
				t.Errorf("%s, AddLine: got unchanged line count %d; want %d", f.Name(), f.LineCount(), i+1)
			}
			verifyPositions(t, fset, f, test.lines[0:i+1])
		}

		// add lines with SetLines and verify all positions
		if ok := f.SetLines(test.lines); !ok {
			t.Errorf("%s: SetLines failed", f.Name())
		}
		if f.LineCount() != len(test.lines) {
			t.Errorf("%s, SetLines: got line count %d; want %d", f.Name(), f.LineCount(), len(test.lines))
		}
		verifyPositions(t, fset, f, test.lines)

		// add lines with SetLinesForContent and verify all positions
		src := test.source
		if src == nil {
			// no test source available - create one from scratch
			src = makeTestSource(test.size, test.lines)
		}
		f.SetLinesForContent(src)
		if f.LineCount() != len(test.lines) {
			t.Errorf("%s, SetLinesForContent: got line count %d; want %d", f.Name(), f.LineCount(), len(test.lines))
		}
		verifyPositions(t, fset, f, test.lines)
	}
}

func TestLineInfo(t *testing.T) {
	fset := NewFileSet()
	f := fset.AddFile("foo", fset.Base(), 500)
	lines := []int{0, 42, 77, 100, 210, 220, 277, 300, 333, 401}
	// add lines individually and provide alternative line information
	for _, offs := range lines {
		f.AddLine(offs)
		f.addLineInfo(offs, "bar", 42)
	}
	// verify positions for all offsets
	for offs := 0; offs <= f.Size(); offs++ {
		p := f.Pos(offs)
		lineCol := MakeLineColumn(LineFromOrdinal(42), linecol(lines, offs).Column())
		msg := fmt.Sprintf("%s (offs = %d, p = %d)", f.Name(), offs, p)
		checkPos(t, msg, f.Position(f.Pos(offs)), Position{"bar", offs, lineCol})
		checkPos(t, msg, fset.Position(p), Position{"bar", offs, lineCol})
	}
}

func TestFiles(t *testing.T) {
	fset := NewFileSet()
	for i, test := range tests {
		base := fset.Base()
		if i%2 == 1 {
			// Setting a negative base is equivalent to
			// fset.Base(), so test some of each.
			base = -1
		}
		fset.AddFile(test.filename, base, test.size)
		j := 0
		fset.Iterate(func(f *File) bool {
			if f.Name() != tests[j].filename {
				t.Errorf("got filename = %s; want %s", f.Name(), tests[j].filename)
			}
			j++
			return true
		})
		if j != i+1 {
			t.Errorf("got %d files; want %d", j, i+1)
		}
	}
}

// FileSet.File should return nil if Pos is past the end of the FileSet.
func TestFileSetPastEnd(t *testing.T) {
	fset := NewFileSet()
	for _, test := range tests {
		fset.AddFile(test.filename, fset.Base(), test.size)
	}
	if f := fset.File(Pos(fset.Base())); f != nil {
		t.Errorf("got %v, want nil", f)
	}
}

func TestFileSetCacheUnlikely(t *testing.T) {
	fset := NewFileSet()
	offsets := make(map[string]int)
	for _, test := range tests {
		offsets[test.filename] = fset.Base()
		fset.AddFile(test.filename, fset.Base(), test.size)
	}
	for file, pos := range offsets {
		f := fset.File(Pos(pos))
		if f.Name() != file {
			t.Errorf("got %q at position %d, want %q", f.Name(), pos, file)
		}
	}
}

// issue 4345. Test that concurrent use of FileSet.Pos does not trigger a
// race in the FileSet position cache.
func TestFileSetRace(t *testing.T) {
	fset := NewFileSet()
	for i := 0; i < 100; i++ {
		fset.AddFile(fmt.Sprintf("file-%d", i), fset.Base(), 1031)
	}
	max := int32(fset.Base())
	var stop sync.WaitGroup
	r := rand.New(rand.NewSource(7))
	for i := 0; i < 2; i++ {
		r := rand.New(rand.NewSource(r.Int63()))
		stop.Add(1)
		go func() {
			for i := 0; i < 1000; i++ {
				fset.Position(Pos(r.Int31n(max)))
			}
			stop.Done()
		}()
	}
	stop.Wait()
}

// issue 16548. Test that concurrent use of File.AddLine and FileSet.PositionFor
// does not trigger a race in the FileSet position cache.
func TestFileSetRace2(t *testing.T) {
	const N = 1e3
	var (
		fset = NewFileSet()
		file = fset.AddFile("", -1, N)
		ch   = make(chan int, 2)
	)

	go func() {
		for i := 0; i < N; i++ {
			file.AddLine(i)
		}
		ch <- 1
	}()

	go func() {
		pos := file.Pos(0)
		for i := 0; i < N; i++ {
			fset.positionFor(pos, false)
		}
		ch <- 1
	}()

	<-ch
	<-ch
}

func lineColFromOrdinals(line, col int) LineColumn {
	return MakeLineColumn(LineFromOrdinal(line), ColumnFromOrdinal(col))
}

func TestPositionFor(t *testing.T) {
	src := []byte(`
foo
b
ar
//line :100
foobar
//line bar:3
done
`)

	const filename = "foo"
	fset := NewFileSet()
	f := fset.AddFile(filename, fset.Base(), len(src))
	f.SetLinesForContent(src)

	// verify position info
	for i, offs := range f.lines {
		got1 := f.positionFor(f.Pos(offs), false)
		got2 := f.positionFor(f.Pos(offs), true)
		got3 := f.Position(f.Pos(offs))
		want := Position{filename, offs, MakeLineColumn(LineFromOrdinal(i+1), ColumnFromOrdinal(1))}
		checkPos(t, "1. PositionFor unadjusted", got1, want)
		checkPos(t, "1. PositionFor adjusted", got2, want)
		checkPos(t, "1. Position", got3, want)
	}

	// manually add //line info on lines l1, l2
	const l1, l2 = 5, 7
	f.addLineInfo(f.lines[l1-1], "", 100)
	f.addLineInfo(f.lines[l2-1], "bar", 3)

	// unadjusted position info must remain unchanged
	for i, offs := range f.lines {
		got1 := f.positionFor(f.Pos(offs), false)
		want := Position{filename, offs, lineColFromOrdinals(i+1, 1)}
		checkPos(t, "2. PositionFor unadjusted", got1, want)
	}

	// adjusted position info should have changed
	for i, offs := range f.lines {
		got2 := f.positionFor(f.Pos(offs), true)
		got3 := f.Position(f.Pos(offs))
		want := Position{filename, offs, lineColFromOrdinals(i+1, 1)}
		// manually compute wanted filename and line
		line := want.Line()
		if i+1 >= l1 {
			want.documentName = ""
			want.lineColumn = lineColFromOrdinals(line.Ordinal()-l1+100, want.lineColumn.Column().Ordinal())
		}
		if i+1 >= l2 {
			want.documentName = "bar"
			want.lineColumn = lineColFromOrdinals(line.Ordinal()-l2+3, want.lineColumn.Column().Ordinal())
		}
		checkPos(t, "3. PositionFor adjusted", got2, want)
		checkPos(t, "3. Position", got3, want)
	}
}

func TestLineStart(t *testing.T) {
	const src = "one\ntwo\nthree\n"
	fset := NewFileSet()
	f := fset.AddFile("input", -1, len(src))
	f.SetLinesForContent([]byte(src))

	for line := 1; line <= 3; line++ {
		pos := f.LineStart(LineFromOrdinal(line))
		position := fset.Position(pos)
		if position.Line() != LineFromOrdinal(line) || position.Column() != ColumnFromOrdinal(1) {
			t.Errorf("LineStart(%d) returned wrong pos %d: %s", line, pos, position)
		}
	}
}

func TestMultibyteChar(t *testing.T) {
	for _, tt := range []struct {
		name    string
		content []byte
		offset  int
		want    LineColumn
	}{
		{
			// See https://github.com/golang/go/issues/45169 for justification
			// of using bytes, not characters, for column.
			"column after 4-byte character should be 5, not 2",
			[]byte("😀fun"), // 😀 is U+1F600, four bytes in utf-8 (f09f9880)),
			4,
			lineColFromOrdinals(1, 5),
		},
		{
			"column after 4-byte character should be 2, not 5",
			[]byte("abcde"),
			4,
			lineColFromOrdinals(1, 5),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {

			fs := NewFileSet()
			f := fs.AddFile("main.txt", -1, len(tt.content))
			f.SetLinesForContent(tt.content)
			got := f.Position(f.Pos(tt.offset)).lineColumn
			if got.String() != tt.want.String() {
				t.Errorf("file.Position(offset %d) got %s, want %s", tt.offset, got, tt.want)
			}
		})
	}
}

func TestPositionForLineColumn(t *testing.T) {
	for _, tt := range []struct {
		name           string
		content        []byte
		lineColumn     LineColumn
		wantValid      bool
		wantByteOffset int
	}{
		{
			"columns can be in the middle of a code point",
			[]byte("😀fun"), // 😀 is U+1F600, four bytes in utf-8 (f09f9880).
			lineColFromOrdinals(1, 2),
			true,
			1,
		},
		{
			"2,2 is offset 11",
			[]byte("abcdefghi\ncd"),
			lineColFromOrdinals(2, 2),
			true,
			11,
		},
		{
			"invalid position 2,10",
			[]byte("abcdefghi\ncd"),
			lineColFromOrdinals(2, 10),
			false,
			0,
		},
		{
			"invalid position 3,3",
			[]byte("abcdefghi\ncd"),
			lineColFromOrdinals(3, 3),
			false,
			0,
		},
		{
			"invalid position 1,0",
			[]byte("abcdefghi\ncd"),
			lineColFromOrdinals(1, 0),
			false,
			0,
		},
		{
			"invalid position 0,1",
			[]byte("abcdefghi\ncd"),
			lineColFromOrdinals(0, 1),
			false,
			0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {

			fs := NewFileSet()
			f := fs.AddFile("main.txt", -1, len(tt.content))
			f.SetLinesForContent(tt.content)
			gotPos := f.PosForLineColumn(tt.lineColumn)
			if got, want := gotPos.IsValid(), tt.wantValid; got != want {
				t.Fatalf("got position.IsValid() = %v, want %v", got, want)
			}
			if !gotPos.IsValid() {
				return
			}
			if got, want := f.Offset(gotPos), tt.wantByteOffset; got != want {
				t.Errorf("got offset %d, want %d", got, want)
			}
		})
	}
}

//😀
