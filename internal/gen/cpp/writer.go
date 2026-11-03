package cppgen

import (
	"bytes"
	"fmt"
	"strings"
)

// writer accumulates generated C++ text, one line at a time.
type writer struct {
	b    bytes.Buffer
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

// size is the bytes written so far.
func (w *writer) size() int { return w.b.Len() }

func (w *writer) String() string { return w.b.String() }

func (w *writer) bytes() []byte { return bytes.Clone(w.b.Bytes()) }

// capture runs write against an empty writer and returns what it wrote; g.c holds what it held before.
func (g *gen) capture(write func()) string {
	outer := g.c
	g.c = writer{base: outer.base}
	write()
	text := g.c.String()
	g.c = outer
	return text
}
