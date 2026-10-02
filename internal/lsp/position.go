package lsp

import (
	"bytes"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// position is an LSP position: a 0-based line and a column in UTF-16 code units.
type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type textRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

// utf16Len is text's UTF-16 code units, an invalid byte one (U+FFFD); IMPLEMENTATION-PLAN §8.4.
func utf16Len(text []byte) int {
	n := 0
	for len(text) > 0 {
		r, size := utf8.DecodeRune(text)
		text = text[size:]
		n += max(utf16.RuneLen(r), 1)
	}
	return n
}

// lines indexes a file's content by line, to convert its findings' columns.
type lines struct {
	text   []byte
	starts []int
}

func newLines(text []byte) *lines {
	l := &lines{text: text, starts: []int{0}}
	for i, b := range text {
		if b == newline {
			l.starts = append(l.starts, i+1)
		}
	}
	return l
}

// at converts a 1-based line and UTF-8 byte column to an LSP position; a column past the
// line's end keeps its excess bytes as units.
func (l *lines) at(line, col int) position {
	if line < 1 || line > len(l.starts) {
		return position{}
	}
	text := l.text[l.starts[line-1]:]
	if end := bytes.IndexByte(text, newline); end >= 0 {
		text = text[:end]
	}
	bytesIn := max(col-1, 0)
	if bytesIn > len(text) {
		return position{Line: line - 1, Character: utf16Len(text) + bytesIn - len(text)}
	}
	return position{Line: line - 1, Character: utf16Len(text[:bytesIn])}
}

// span converts a finding's location; the end is exclusive in both forms.
func (l *lines) span(loc source.Location) textRange {
	return textRange{Start: l.at(loc.Line, loc.Col), End: l.at(loc.EndLine, loc.EndCol)}
}

// pathOf is the absolute '/'-separated name of a file URI, false for another scheme.
func pathOf(uri string) (string, bool) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != fileScheme || u.Path == "" {
		return "", false
	}
	name := u.Path
	if u.Host != "" {
		name = uncPrefix + u.Host + name
	} else if vol := strings.TrimPrefix(name, uriSep); filepath.VolumeName(filepath.FromSlash(vol)) != "" {
		name = vol
	}
	return project.Clean(name), true
}

// uriOf is the file URI of an absolute '/'-separated name.
func uriOf(name string) string {
	u := url.URL{Scheme: fileScheme, Path: name}
	if rest, ok := strings.CutPrefix(name, uncPrefix); ok {
		host, tail, _ := strings.Cut(rest, uriSep)
		u.Host, u.Path = host, uriSep+tail
	} else if !strings.HasPrefix(name, uriSep) {
		u.Path = uriSep + name
	}
	return u.String()
}
