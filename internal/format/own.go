package format

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// hostedBy are the comments the formatter's attachment gives to token t: the notes it prints
// before and after t, wherever they lie in the text (log-2026-09-29 M4 U1r).
func (b *builder) hostedBy(t syntax.Tok) []note {
	// FORMATTER.md §8.1, DECISIONS 167, 168, 216
	return slices.Concat(b.notes[t].lead, b.notes[t].trail)
}

// owns is the bytes of the tokens from to to and of every comment attached to them.
func (b *builder) owns(from, to syntax.Tok) span {
	s := span{int(b.f.Tokens[from].Start), int(b.f.Tokens[to].End)}
	for t := from; t <= to; t++ {
		for _, n := range b.hostedBy(t) {
			s.lo, s.hi = min(s.lo, n.src.lo), max(s.hi, n.src.hi)
		}
	}
	return s
}

// cuts reports an offset strictly inside a comment, where no part of a list may begin or end.
func (b *builder) cuts(at int) bool {
	if b.comments == nil {
		b.comments = []span{}
		for _, a := range b.notes {
			for _, n := range slices.Concat(a.lead, a.trail) {
				b.comments = append(b.comments, n.src)
			}
		}
		slices.SortFunc(b.comments, func(x, y span) int { return x.lo - y.lo })
	}
	i, _ := slices.BinarySearchFunc(b.comments, at, func(s span, at int) int { return s.lo - at })
	return i > 0 && b.comments[i-1].lo < at && at < b.comments[i-1].hi
}

// ownsItem is what item n owns: its tokens and its separator comma, whose comments go with it
// when the comma is kept (DECISIONS 216), and their comments.
func (b *builder) ownsItem(n syntax.Node) span { return b.owns(n.First(), b.separator(n)) }

// sides are the token ranges of the two ends of a region of a list, whose comments stay.
type sides struct{ lf, lt, rf, rt syntax.Tok }

// atom is a part of a region: a comment, a comma or a removed item; keep when it stays.
type atom struct {
	s          span
	text       string
	keep, left bool
	line, own  bool
}

// atoms are the parts of region r of g between kept item left (-1: the opening bracket) and
// kept item right (the item count: the closing bracket), in text order.
func (b *builder) atoms(g *batch, left, right int, r span) []atom {
	sd := b.sidesOf(g, left, right)
	var out []atom
	for t := sd.lf; t <= sd.rt; t++ {
		inLeft, inRight := t <= sd.lt, t >= sd.rf
		for _, n := range b.hostedBy(t) {
			if n.src.lo >= r.lo && n.src.hi <= r.hi {
				a := atom{s: n.src, keep: inLeft || inRight, left: inLeft, own: b.ownLine(n.src.lo)}
				a.text, a.line = string(b.f.Src.Content[n.src.lo:n.src.hi]), isLine(n) && !strings.Contains(n.text, newlineText)
				out = append(out, a)
			}
		}
		tk := b.f.Tokens[t]
		if tk.Kind == syntax.TokComma && int(tk.Start) >= r.lo && int(tk.End) <= r.hi {
			out = append(out, atom{s: span{int(tk.Start), int(tk.End)}, text: tk.Kind.String(), keep: inLeft && !b.drop[t], left: true})
		}
	}
	for i := left + 1; i < right; i++ {
		lo, hi := b.span(g.l.items[i])
		out = append(out, atom{s: span{lo, hi}})
	}
	slices.SortStableFunc(out, func(x, y atom) int { return x.s.lo - y.s.lo })
	return out
}

// sidesOf are the ends of the region between left and right: an item with its separator, or
// a bracket.
func (b *builder) sidesOf(g *batch, left, right int) sides {
	sd := sides{lf: g.l.open, lt: g.l.open, rf: g.l.close, rt: g.l.close}
	if left >= 0 {
		sd.lf, sd.lt = g.l.items[left].First(), b.separator(g.l.items[left])
	}
	if right < len(g.l.items) {
		sd.rf, sd.rt = g.l.items[right].First(), b.separator(g.l.items[right])
	}
	return sd
}
