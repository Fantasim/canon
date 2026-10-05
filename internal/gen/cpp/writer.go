package cppgen

import (
	"fmt"
	"strings"
)

// writer accumulates generated C++ text, one line at a time.
type writer struct {
	b    strings.Builder
	base int // levels every indented line gets on top of its own, inside a block written apart
}

// line writes s and a line break; indentation is part of s.
func (w *writer) line(s string) {
	w.b.WriteString(s)
	w.b.WriteString(newline)
}

func (w *writer) blank() { w.b.WriteString(newline) }

// printf writes formatted text as is: the format carries its own line breaks.
func (w *writer) printf(format string, args ...any) {
	fmt.Fprintf(&w.b, format, args...)
}

// linef writes an indented formatted line: depth levels of four spaces.
func (w *writer) linef(depth int, format string, args ...any) {
	w.b.WriteString(strings.Repeat(indentUnit, w.base+depth))
	fmt.Fprintf(&w.b, format, args...)
	w.b.WriteString(newline)
}

// lineAt writes s indented at depth, and a line break.
func (w *writer) lineAt(depth int, s string) {
	w.b.WriteString(strings.Repeat(indentUnit, w.base+depth))
	w.line(s)
}

func (w *writer) write(s string) { w.b.WriteString(s) }

func (w *writer) String() string { return w.b.String() }

func (w *writer) bytes() []byte { return []byte(w.b.String()) }
