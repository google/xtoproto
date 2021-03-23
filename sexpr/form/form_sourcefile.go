package form

// SourceSpan is a continuous interval of positions within a text file.
type SourceSpan struct {
}

// FileName is the name of the source file.
func (s *SourceSpan) FileName() string {

}

// String is a concise, human-readable representation of the span suitable
// for printing in error messages.
func (s *SourceSpan) String() string {

}
func (s *SourceSpan) start() rowCol {

}
func (s *SourceSpan) end() rowCol {

}
