package format

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/syntax"
)

// Place is where a node is printed: from column Column of a line indented Indent, with Rest
// columns after it on its last line, which the width check counts.
type Place struct {
	Indent, Column, Rest int
}

// Node prints n, a node of f without error, at place: its first token to its last, without the
// comments at its edges; its brace lists keep their single-line bit from f's text.
func Node(f *syntax.File, n syntax.Node, at Place) ([]byte, error) {
	// FORMATTER.md §13 steps 1 and 2
	if at.Indent < 0 || at.Column < 0 || at.Rest < 0 {
		return nil, ErrPlace
	}
	if err := alone(f, n); err != nil {
		return nil, err
	}
	b := newBuilder(f)
	b.file()
	return []byte(renderAt(b.bare(n), at.Indent, at.Column, strings.Repeat(restFill, at.Rest))), nil
}

// Flat prints n, a node of f without error, on one line whatever its width. ErrLines when n
// holds what a line cannot: a line comment, a comma kept on its own line (DECISIONS 216), a
// multiline string, a block of several statements.
func Flat(f *syntax.File, n syntax.Node) ([]byte, error) {
	// API.md E26, FORMATTER.md §6.1
	if err := alone(f, n); err != nil {
		return nil, err
	}
	p := &printer{atStart: true, empties: 1}
	p.run(cmd{0, flatMode, newBuilder(f).bare(n)})
	if bytes.Contains(p.out, []byte(newlineText)) || manyStatements(n) {
		return nil, ErrLines
	}
	return p.out, nil
}

// manyStatements reports a block of several statements in n.
func manyStatements(n syntax.Node) bool {
	found := false
	syntax.Inspect(n, func(c syntax.Node) bool {
		if blk, ok := c.(*syntax.Block); ok && len(blk.Stmts) > 1 {
			found = true
		}
		return !found
	})
	return found
}

// alone is ErrSyntax for a node of f that did not parse, ErrNode for one printed only with the
// node holding it (an interpolation, a doc comment) or not of f.
func alone(f *syntax.File, n syntax.Node) error {
	switch {
	case !soundNode(n):
		return ErrSyntax
	case !holds(f, n):
		return ErrNode
	case buildTable[n.Kind()] == nil:
		return ErrNode
	}
	return nil
}

// soundNode reports a node without a part that did not parse.
func soundNode(n syntax.Node) bool {
	if absent(n) {
		return false
	}
	ok := true
	syntax.Inspect(n, func(c syntax.Node) bool {
		ok = ok && (c == nil || !badKinds[c.Kind()] && c.Last() >= c.First())
		return ok
	})
	return ok
}

// splice replaces the bytes lo to hi of a text by text.
type splice struct {
	lo, hi int
	text   string
}

// reprint lays item n out from its current column, the rest of its last line counted by fits.
func (b *builder) reprint(n syntax.Node) splice {
	// FORMATTER.md §13 steps 1 and 2; every item of n is built, whatever the focus
	focus := b.focus
	b.focus = nil
	defer func() { b.focus = focus }()
	src := b.f.Src.Content
	lo, hi := b.span(n)
	start := lineStart(src, lo)
	ind := len(src[start:lo]) - len(bytes.TrimLeft(src[start:lo], space))
	rest := string(src[hi:lineEnd(src, hi)])
	return splice{lo, hi, renderAt(b.bare(n), ind, utf8.RuneCount(src[start:lo]), rest)}
}

// unit re-prints item n, or the item holding its list printed on one line when n no longer
// fits there.
func (b *builder) unit(n syntax.Node) splice {
	// FORMATTER.md §13 step 3
	for {
		s := b.reprint(n)
		l, ok := b.idx.listOf(n)
		if !ok || !b.oneLine(l.open, l.close) || !b.overflows(s) {
			return s
		}
		up := b.outer(n)
		if up == nil {
			return s
		}
		n = up
	}
}

// overflows reports a splice whose text spans lines or leaves its line wider than Width.
func (b *builder) overflows(s splice) bool {
	if strings.Contains(s.text, newlineText) {
		return true
	}
	src := b.f.Src.Content
	line := string(src[lineStart(src, s.lo):s.lo]) + s.text + string(src[s.hi:lineEnd(src, s.hi)])
	return utf8.RuneCountInString(line) > Width
}

// lineStart is the offset of the line holding offset at; lineEnd, of its line break or the end.
func lineStart(src []byte, at int) int {
	return bytes.LastIndex(src[:at], []byte(newlineText)) + 1
}

func lineEnd(src []byte, at int) int {
	if i := bytes.Index(src[at:], []byte(newlineText)); i >= 0 {
		return at + i
	}
	return len(src)
}
