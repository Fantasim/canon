package format

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// lines is the layout of a list laid out one item per line, or of the top level: the end of its
// first line, each item's lines with comments and separator, the blank lines before each item,
// and the tail after the last item, whose leading blank lines are blank.
type lines struct {
	head               int
	regions, gaps      []span
	tail, blank        span
	comments, headNote bool
}

// layout is g's layout, each item's lines holding what the formatter's attachment gives it
// (log-2026-09-29 M4 U1r); false when two items, or an item and a comment another part owns,
// share a line.
func (b *builder) layout(g *batch) (lines, bool) {
	src := b.f.Src.Content
	var ly lines
	var from syntax.Tok
	if g.top {
		from, ly.tail.hi = b.header(), len(src)
	} else {
		from, ly.tail.hi = g.l.open, lineStart(src, int(b.f.Tokens[g.l.close].Start))
		ly.headNote = len(b.notes[g.l.open].trail) > 0
	}
	ly.head = min(lineEnd(src, b.owns(from, from).hi)+1, len(src))
	prev := ly.head
	for _, n := range g.l.items {
		s := b.ownsItem(n)
		r := span{lineStart(src, s.lo), min(lineEnd(src, s.hi)+1, len(src))}
		if r.lo < prev || strings.TrimSpace(string(src[prev:r.lo])) != "" || b.cuts(r.lo) || b.cuts(r.hi) {
			return ly, false
		}
		ly.gaps, ly.regions, prev = append(ly.gaps, span{prev, r.lo}), append(ly.regions, r), r.hi
	}
	if prev > ly.tail.hi || b.cuts(ly.head) || b.cuts(prev) {
		return ly, false
	}
	ly.tail.lo, ly.blank = prev, span{prev, prev}
	rest := src[prev:ly.tail.hi]
	for len(rest) > 0 && len(bytes.TrimSpace(rest[:lineEnd(rest, 0)])) == 0 {
		cut := min(lineEnd(rest, 0)+1, len(rest))
		ly.blank.hi, rest = ly.blank.hi+cut, rest[cut:]
	}
	ly.comments = len(bytes.TrimSpace(rest)) > 0
	return ly, true
}

// planLines is g's edits in a list laid out one item per line, or at the top level: removed
// items go with their lines, new ones take lines of their own after the item before them,
// then commas and blank lines follow the final items.
func (b *builder) planLines(g *batch) ([]edit, error) {
	// FORMATTER.md §4, §13 steps 4 and 5, DECISIONS 211, log-2026-09-29 M4 U1r
	ly, ok := b.layout(g)
	if !ok {
		return nil, fmt.Errorf("%w: an item shares a line with another part of its list", ErrChange)
	}
	fin := b.final(g)
	if len(fin) == 0 && !g.top && !ly.headNote && !ly.comments {
		return []edit{{lo: int(b.f.Tokens[g.l.open].End), hi: int(b.f.Tokens[g.l.close].Start)}}, nil
	}
	var out []edit
	for i, r := range ly.regions {
		if g.removed[i] {
			out = append(out, edit{lo: r.lo, hi: r.hi})
		}
	}
	out = append(out, b.lineNews(g, ly, fin)...)
	if !g.top {
		out = append(out, b.lineCommas(g, fin)...)
	}
	return append(out, b.lineGaps(g, ly)...), nil
}

// lineNews writes the new items at each position on lines of their own: at the list's item
// indentation with the comma they need, or after one blank line at the top level.
func (b *builder) lineNews(g *batch, ly lines, fin []slot) []edit {
	var out []edit
	brace := g.top || b.f.Tokens[g.l.open].Kind == syntax.TokLBrace
	ind := strings.Repeat(space, b.itemIndent(g))
	for k, s := range fin {
		if s.item >= 0 {
			continue
		}
		pos := ly.head
		if s.at > 0 {
			pos = ly.regions[s.at-1].hi
		}
		if len(out) == 0 || out[len(out)-1].lo != pos {
			out = append(out, edit{lo: pos, hi: pos})
		}
		e := &out[len(out)-1]
		lead, tail := ind, newlineText
		if g.top {
			lead = newlineText
		} else if needsComma(brace, fin, k) {
			tail = syntax.TokComma.String() + newlineText
		}
		m := mark{lo: len(e.text) + len(lead), unit: true, fresh: true, want: g.want()}
		m.hi = m.lo + len(s.text)
		e.text += lead + s.text + tail
		e.marks = append(e.marks, m)
	}
	return out
}

// itemIndent is the indentation of g's items: the first one's, else one level past the line
// of the opening bracket.
func (b *builder) itemIndent(g *batch) int {
	if len(g.l.items) > 0 {
		return b.indentOf(g.l.items[0].First())
	}
	return b.indentOf(g.l.open) + indentUnit
}

// lineCommas writes or removes the comma after each kept item as the item after it needs.
func (b *builder) lineCommas(g *batch, fin []slot) []edit {
	var out []edit
	brace := b.f.Tokens[g.l.open].Kind == syntax.TokLBrace
	for k, s := range fin {
		if s.item < 0 {
			continue
		}
		last := g.l.items[s.item].Last()
		c := b.after(last)
		has, need := b.f.Tokens[c].Kind == syntax.TokComma, needsComma(brace, fin, k)
		switch {
		case need && !has:
			end := int(b.f.Tokens[last].End)
			out = append(out, edit{lo: end, hi: end, text: syntax.TokComma.String()})
		case !need && has && b.oneLine(last, c):
			out = append(out, edit{lo: int(b.f.Tokens[c].Start), hi: int(b.f.Tokens[c].End)})
		}
	}
	return out
}

// lineGaps removes the blank lines the changes leave where they are not allowed: one of two in
// a row, one after an opening bracket, before a closing one or at the end of the file. A new
// top-level declaration brings its own blank line.
func (b *builder) lineGaps(g *batch, ly lines) []edit {
	// FORMATTER.md §4, log-2026-09-29 M4 G1
	var gs gaps
	for p := range g.news {
		if len(g.news[p]) > 0 {
			gs.flush(!g.top && gs.seen)
		}
		if p == len(g.removed) {
			break
		}
		gs.pend = append(gs.pend, ly.gaps[p])
		if !g.removed[p] {
			gs.flush(g.top || gs.seen)
		}
	}
	gs.pend = append(gs.pend, ly.blank)
	gs.flush((gs.seen || g.top) && ly.comments)
	return gs.out
}

// gaps are the blank lines met since the last item kept or added (pend), and the removals of
// the ones that go.
type gaps struct {
	pend []span
	out  []edit
	seen bool
}

// flush settles the pending blank lines before an item: the first one stays when keep is set,
// every other one goes.
func (gs *gaps) flush(keep bool) {
	for _, s := range gs.pend {
		if s.hi > s.lo && !keep {
			gs.out = append(gs.out, edit{lo: s.lo, hi: s.hi})
		}
		keep = keep && s.hi == s.lo
	}
	gs.pend, gs.seen = nil, true
}
