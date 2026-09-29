package format

import (
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// planInline is g's edits in a list not laid out one item per line: each region between two
// kept items, or between a bracket and the kept item next to it, where an item goes or comes,
// is written again; a comment goes only with the item the formatter attaches it to.
func (b *builder) planInline(g *batch) ([]edit, error) {
	// FORMATTER.md §8.1, §13 steps 4 and 5, log-2026-09-29 M4 U1r, U1b
	if err := b.inlineMoves(g); err != nil {
		return nil, err
	}
	var out []edit
	var news []slot
	left := -1
	for _, s := range b.final(g) {
		if s.item < 0 {
			news = append(news, s)
			continue
		}
		if len(news) > 0 || s.item != left+1 {
			out = append(out, b.region(g, left, s.item, news))
		}
		news, left = nil, s.item
	}
	if len(news) > 0 || left != len(g.l.items)-1 {
		out = append(out, b.region(g, left, len(g.l.items), news))
	}
	return out, nil
}

// regionWriter writes a region again: the parts that stay with their bytes, the text between
// two of them as it was when nothing was removed there, the new items after the left end's.
type regionWriter struct {
	b              *builder
	g              *batch
	r              span
	pos            int
	text           string
	marks          []mark
	broken, closed bool
	lastLine       bool
	edge           string
}

// region is the edit of the bytes between kept item left (-1: the opening bracket) and kept
// item right (the item count: the closing bracket), news coming between them.
func (b *builder) region(g *batch, left, right int, news []slot) edit {
	w := &regionWriter{b: b, g: g, closed: right == len(g.l.items)}
	if b.f.Tokens[g.l.open].Kind == syntax.TokLBrace {
		w.edge = space
	}
	w.r = span{int(b.f.Tokens[g.l.open].End), int(b.f.Tokens[g.l.close].Start)}
	if left >= 0 {
		w.r.lo = int(b.f.Tokens[g.l.items[left].Last()].End)
	}
	if !w.closed {
		w.r.hi = int(b.f.Tokens[g.l.items[right].First()].Start)
	}
	w.pos = w.r.lo
	atoms := b.atoms(g, left, right, w.r)
	lead := left >= 0 && (len(news) > 0 || !w.closed)
	written := false
	for _, a := range atoms {
		lead = lead && (!a.keep || a.text != syntax.TokComma.String())
	}
	for _, a := range atoms {
		switch {
		case !a.keep:
			w.broken = true
		case !a.left && !written:
			w.insert(lead, news)
			written = true
			w.keep(a)
		default:
			w.keep(a)
		}
	}
	if !written {
		w.insert(lead, news)
	}
	w.end(left < 0)
	return edit{lo: w.r.lo, hi: w.r.hi, text: w.text, marks: w.marks}
}

// keep writes a part that stays, after the text that was before it or a synthesized one.
func (w *regionWriter) keep(a atom) {
	src := w.b.f.Src.Content
	switch {
	case !w.broken:
		w.text += string(src[w.pos:a.s.lo])
	case w.lastLine || a.own:
		w.text += newlineText + strings.Repeat(space, w.indentAt(a.s.lo))
	case a.text != syntax.TokComma.String():
		w.text += space
	}
	w.text += a.text
	w.pos, w.broken, w.lastLine = a.s.hi, false, a.line
}

// insert writes the new items, with a comma after the left item when none stays there
// (lead) and one before a right item; nothing when there is nothing to write.
func (w *regionWriter) insert(lead bool, news []slot) {
	if !lead && len(news) == 0 {
		return
	}
	switch {
	case w.lastLine:
		w.text += newlineText + strings.Repeat(space, w.indentAt(w.r.hi))
	case lead:
	case w.text == "" && w.pos == w.r.lo:
		w.text += w.edge
	default:
		w.text += space
	}
	sep := ""
	if lead {
		w.text += syntax.TokComma.String()
		sep = space
	}
	for i, s := range news {
		if i > 0 {
			sep = listSep
		}
		w.text += sep
		t, m := w.b.piece(w.g, s)
		m.lo, m.hi = m.lo+len(w.text), m.hi+len(w.text)
		w.marks = append(w.marks, m)
		w.text += t
	}
	if len(news) > 0 && !w.closed {
		w.text += syntax.TokComma.String()
	}
	w.broken, w.lastLine = true, false
}

// end closes the region: the text that was left before the right end, or a synthesized one;
// a list left with nothing is "{}", "()" or "[]".
func (w *regionWriter) end(fromOpen bool) {
	switch {
	case !w.broken:
		w.text += string(w.b.f.Src.Content[w.pos:w.r.hi])
	case w.lastLine:
		w.text += newlineText + strings.Repeat(space, w.indentAt(w.r.hi))
	case fromOpen && w.closed && w.text == "":
	case w.closed:
		w.text += w.edge
	default:
		w.text += space
	}
}

// indentAt is the indentation of the line holding offset at.
func (w *regionWriter) indentAt(at int) int {
	src := w.b.f.Src.Content
	start := lineStart(src, at)
	line := string(src[start:lineEnd(src, start)])
	return len(line) - len(strings.TrimLeft(line, space))
}
