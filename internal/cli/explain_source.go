package cli

import (
	"bytes"
	"path"
	"strings"
	"sync"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// sourceFS is the OS file system that keeps each file the compiler read, so explain quotes the
// sources of the analysis it explains, never a later copy of them.
type sourceFS struct {
	build.WriteFS
	mu   sync.Mutex
	read map[string][]byte
}

func newSourceFS() *sourceFS {
	return &sourceFS{WriteFS: build.OS(), read: map[string][]byte{}}
}

// ReadFile reads name and keeps its content.
func (f *sourceFS) ReadFile(name string) ([]byte, error) {
	data, err := f.WriteFS.ReadFile(name)
	if err == nil {
		f.mu.Lock()
		f.read[name] = data
		f.mu.Unlock()
	}
	return data, err
}

// EvalSymlinks keeps load.dir following links as through the OS file system (WIRE.md §6.5).
func (f *sourceFS) EvalSymlinks(name string) (string, error) {
	return project.EvalSymlinks(f.WriteFS, name)
}

// content is what the compiler read of name, nil when it read nothing there.
func (f *sourceFS) content(name string) []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.read[name]
}

// sources parses, once each, the project sources an explanation quotes.
type sources struct {
	fs    *sourceFS
	root  string
	set   source.FileSet
	files map[string]*syntax.File
}

// file is the parsed source at a display path, nil for none. Expressions are only in .canon
// sources, which are project-relative, never under a root (`@root/…`).
func (s *sources) file(display string) *syntax.File {
	if f, ok := s.files[display]; ok {
		return f
	}
	var f *syntax.File
	abs := path.Join(s.root, display)
	if data := s.fs.content(abs); data != nil {
		if src, err := s.set.Add(display, abs, data); err == nil {
			f = syntax.Parse(src, syntax.FileSource, diag.NewBag(&s.set, ""))
		}
	}
	s.files[display] = f
	return f
}

// expr is the source text at o's span on one line: a line break and the indentation after it
// become one space between two tokens; tokens stay byte-exact (oneLine writes their line breaks).
func (s *sources) expr(o canon.Origin) string {
	f := s.file(o.File)
	if f == nil {
		return ""
	}
	text := f.Src.Content
	start, end := offset(text, o.Line, o.Col), offset(text, o.EndLine, o.EndCol)
	var sb strings.Builder
	prev := -1
	for _, t := range f.Tokens {
		if int(t.Start) < start || int(t.End) > end || t.End == t.Start {
			continue
		}
		if gap := text[max(prev, start):t.Start]; prev >= 0 && bytes.ContainsRune(gap, lineBreakRune) {
			sb.WriteString(wordSep)
		} else if prev >= 0 {
			sb.Write(gap)
		}
		sb.Write(text[t.Start:t.End])
		prev = int(t.End)
	}
	return sb.String()
}

// offset is the byte offset of a 1-based line and byte column in text, len(text) past its end.
func offset(text []byte, line, col int) int {
	at := 0
	for n := 1; n < line; n++ {
		i := bytes.IndexByte(text[at:], lineBreakRune)
		if i < 0 {
			return len(text)
		}
		at += i + 1
	}
	return min(at+col-1, len(text))
}
