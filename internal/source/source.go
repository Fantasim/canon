package source

import (
	"bytes"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"sync"
)

// FileID identifies a file of a file set; NoFile is no file.
type FileID uint32

// Pos is a byte offset in a file's normalized content.
type Pos int32

// Span is the byte range [Start, End) of one file.
type Span struct {
	File       FileID
	Start, End Pos
}

// Len is the number of bytes the span covers.
func (s Span) Len() int { return int(s.End - s.Start) }

// Contains reports whether p lies in [Start, End).
func (s Span) Contains(p Pos) bool { return s.Start <= p && p < s.End }

// Cover is the smallest span of s's file holding both s and t.
func (s Span) Cover(t Span) Span {
	return Span{File: s.File, Start: min(s.Start, t.Start), End: max(s.End, t.End)}
}

// File is one source file; Content has every "\r\n" normalized to "\n" (GRAMMAR.md §1).
type File struct {
	ID      FileID
	Path    string
	Abs     string
	Content []byte
	lines   []Pos
	size    Pos
}

// Position is the 1-based line and column of p, the column counted in UTF-8 bytes (API.md §1.3).
func (f *File) Position(p Pos) (line, col int) {
	p = min(max(p, 0), f.size)
	i, found := slices.BinarySearch(f.lines, p)
	if !found {
		i--
	}
	return i + 1, int(p-f.lines[i]) + 1
}

// Location is a span resolved for display: a display path and 1-based, end-exclusive bounds.
type Location struct {
	Path            string
	Line, Col       int
	EndLine, EndCol int
}

// FileSet holds the files of one build; it is append-only and safe for concurrent use.
type FileSet struct {
	mu    sync.RWMutex
	files []*File
	last  FileID
}

// Add normalizes content, records it under the next id and returns the new file.
func (s *FileSet) Add(path, abs string, content []byte) (*File, error) {
	if len(content) > math.MaxInt32 {
		return nil, fmt.Errorf("%s: %w", path, ErrFileTooLarge)
	}
	text := bytes.ReplaceAll(content, []byte(crlf), []byte(lf))
	f := &File{Path: filepath.ToSlash(path), Abs: filepath.ToSlash(abs), Content: text}
	f.lines, f.size = index(text)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.last++
	f.ID = s.last
	s.files = append(s.files, f)
	return f, nil
}

// File is the file with the given id, or nil when the set holds none.
func (s *FileSet) File(id FileID) *File {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id == NoFile || int(id) > len(s.files) {
		return nil
	}
	return s.files[id-1]
}

// Path is the display path of the file with the given id, "" for no known file.
func (s *FileSet) Path(id FileID) string {
	if f := s.File(id); f != nil {
		return f.Path
	}
	return ""
}

// Position is File.Position of the file with the given id, 0:0 for no known file.
func (s *FileSet) Position(id FileID, p Pos) (line, col int) {
	if f := s.File(id); f != nil {
		return f.Position(p)
	}
	return 0, 0
}

// Content is the normalized content of the file with the given id, nil for no known file.
func (s *FileSet) Content(id FileID) []byte {
	if f := s.File(id); f != nil {
		return f.Content
	}
	return nil
}

// Locate resolves sp for display; a span of no known file is the zero Location.
func (s *FileSet) Locate(sp Span) Location {
	f := s.File(sp.File)
	if f == nil {
		return Location{}
	}
	loc := Location{Path: f.Path}
	loc.Line, loc.Col = f.Position(sp.Start)
	loc.EndLine, loc.EndCol = f.Position(sp.End)
	return loc
}

// index returns the start of each line of text and its length, counted as positions.
func index(text []byte) ([]Pos, Pos) {
	lines := []Pos{0}
	var p Pos
	for _, b := range text {
		p++
		if b == newline {
			lines = append(lines, p)
		}
	}
	return lines, p
}
