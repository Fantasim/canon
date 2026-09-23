package source

// FileID identifies a file of a file set.
type FileID uint32

// Pos is a byte offset in a file's normalized content.
type Pos int32

// Span is the byte range [Start, End) of one file.
type Span struct {
	File       FileID
	Start, End Pos
}
