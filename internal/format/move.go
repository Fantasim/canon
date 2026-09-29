package format

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// move takes item from to position to of g's list, counted among its items before the changes
// as an Insert's; a move to its own place changes nothing (log-2026-09-29 M4 U1b).
func (g *batch) move(from, to int) error {
	if to < 0 || to > len(g.l.items) {
		return fmt.Errorf("%w: no position %d to move to", ErrChange, to)
	}
	if to != from && to != from+1 {
		g.removed[from] = true
		g.news[to] = append(g.news[to], arrival{from: from})
	}
	return nil
}

// movedLines are the lines of moved item n, r in the layout, with the comma after it written or
// removed as need says (DECISIONS 211, 216).
func (b *builder) movedLines(r span, n syntax.Node, need bool) string {
	src := b.f.Src.Content
	e, ok := b.commaEdit(n, need)
	if !ok {
		return string(src[r.lo:r.hi])
	}
	return string(src[r.lo:e.lo]) + e.text + string(src[e.hi:r.hi])
}

// commaEdit writes or removes the comma after item n as need says; false when it is right.
func (b *builder) commaEdit(n syntax.Node, need bool) (edit, bool) {
	// FORMATTER.md §6.1, DECISIONS 211
	last := n.Last()
	c := b.after(last)
	has := b.f.Tokens[c].Kind == syntax.TokComma
	switch {
	case need && !has:
		end := int(b.f.Tokens[last].End)
		return edit{lo: end, hi: end, text: syntax.TokComma.String()}, true
	case !need && has && b.oneLine(last, c):
		return edit{lo: int(b.f.Tokens[c].Start), hi: int(b.f.Tokens[c].End)}, true
	}
	return edit{}, false
}

// edgeNotes are the comments the formatter attaches to item n and its separator that lie
// outside its tokens: before them and after them, in text order.
func (b *builder) edgeNotes(n syntax.Node) (before, after []note) {
	// FORMATTER.md §8.1, DECISIONS 167, 168, 216
	lo, hi := b.span(n)
	for t := n.First(); t <= b.separator(n); t++ {
		for _, c := range b.hostedBy(t) {
			switch {
			case c.src.hi <= lo:
				before = append(before, c)
			case c.src.lo >= hi:
				after = append(after, c)
			}
		}
	}
	byOffset := func(x, y note) int { return x.src.lo - y.src.lo }
	slices.SortFunc(before, byOffset)
	slices.SortFunc(after, byOffset)
	return before, after
}

// inlineMoves is ErrChange when an item moving in g's list, which is not laid out one item per
// line, holds a comment that ends its line or spans lines: its text cannot join a line.
func (b *builder) inlineMoves(g *batch) error {
	for _, news := range g.news {
		for _, e := range news {
			if e.from < 0 {
				continue
			}
			before, after := b.edgeNotes(g.l.items[e.from])
			if slices.ContainsFunc(slices.Concat(before, after), endsLine) {
				return fmt.Errorf("%w: a moved item's comment ends its line", ErrChange)
			}
		}
	}
	return nil
}

// endsLine reports a comment that a line cannot hold anything after: a line comment, or a
// block comment spanning lines.
func endsLine(n note) bool { return isLine(n) || strings.Contains(n.text, newlineText) }

// piece is the text slot s writes in a list on one line, with the mark on its item: a new text,
// or a moved item between its comments, one space apart, printed again from its place (step 3).
func (b *builder) piece(g *batch, s slot) (string, mark) {
	// FORMATTER.md §8.1, §13 steps 3 and 4, log-2026-09-29 M4 U1b
	if s.from < 0 {
		return s.text, mark{hi: len(s.text), unit: true, fresh: true, want: g.want()}
	}
	n := g.l.items[s.from]
	src := b.f.Src.Content
	lo, hi := b.span(n)
	before, after := b.edgeNotes(n)
	var text string
	for _, c := range before {
		text += string(src[c.src.lo:c.src.hi]) + space
	}
	m := mark{lo: len(text), hi: len(text) + hi - lo, unit: true}
	text += string(src[lo:hi])
	for _, c := range after {
		text += space + string(src[c.src.lo:c.src.hi])
	}
	return text, m
}
