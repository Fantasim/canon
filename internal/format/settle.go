package format

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
)

// stepper is one turn of the settle: done when content, whose tree is g, is a fixed point, else
// the splice re-printing one item (API.md M5).
type stepper func(g *syntax.File, content []byte, last span) (splice, bool, error)

// settleLoop re-prints the item holding the first byte that differs from the formatter's layout,
// or the item holding the last one re-printed when that did not help, until content is a fixed
// point (API.md M5); g is content's tree when known, kind the role f is judged in.
func settleLoop(f *syntax.File, kind syntax.FileKind, content []byte, g *syntax.File, step stepper) ([]byte, error) {
	last := span{-1, -1}
	for tries := len(f.Tokens); tries >= 0; tries-- {
		if g == nil || !bytes.Equal(g.Src.Content, content) {
			var err error
			if g, err = reparse(f, kind, content); err != nil {
				return nil, err
			}
		}
		s, done, err := step(g, content, last)
		switch {
		case err != nil:
			return nil, err
		case done:
			return content, nil
		}
		content = slices.Concat(content[:s.lo], []byte(s.text), content[s.hi:])
		last, g = span{s.lo, s.lo + len(s.text)}, nil
	}
	return nil, ErrUnsettled
}

// settleStep is a turn of the settle judged on the whole file.
func settleStep(g *syntax.File, content []byte, last span) (splice, bool, error) {
	b := newBuilder(g)
	want := render(b.file())
	if bytes.Equal(want, content) {
		return splice{}, true, nil
	}
	return b.settleAt(content, want, last)
}

// settleAt re-prints the item holding the first byte of content that differs from want, the
// layout, or the item holding the last splice when that did not help.
func (b *builder) settleAt(content, want []byte, last span) (splice, bool, error) {
	at := commonPrefix(content, want)
	n := b.unitAt(at, at)
	for n != nil && within(b.span(n))(last) {
		n = b.outer(n)
	}
	if n == nil {
		return splice{}, false, fmt.Errorf("%w: offset %d", ErrUnsettled, at)
	}
	return b.reprint(n), false, nil
}

// aroundOf settles a Rewrite of f, a fixed point, around the bytes it changed: a section a
// broken list or the file prints alone begins and ends every look ahead of the printer, so the
// file is a fixed point exactly when the section holding every changed byte is.
type aroundOf struct {
	f  *syntax.File
	fb *builder // a builder of f, whose reading of comments and commas each look at f shares
	gb *builder // a builder of the text the settle starts from, shared the same way
}

// view is the layout of a text judged on one section: want, and the builder of the text that
// built every item of the section's item, whose bytes are item.
type view struct {
	b    *builder
	want []byte
	item span
}

// step is a turn of the settle judged on one section, or on the whole file when no section
// holds the changed bytes alone.
func (r *aroundOf) step(g *syntax.File, content []byte, last span) (splice, bool, error) {
	// FORMATTER.md §7.1, §13, API.md M5
	v, ok := r.judge(g, content)
	switch {
	case !ok:
		return settleStep(g, content, last)
	case bytes.Equal(v.want, content):
		return splice{}, true, nil
	}
	if at := commonPrefix(content, v.want); at < v.item.lo || at > v.item.hi {
		return settleStep(g, content, last)
	}
	return v.b.settleAt(content, v.want, last)
}

// judge is the layout of content, f's text changed inside one section: f's bytes around it, and
// that section printed again. False when no section holds every changed byte alone, or the
// section of f in its place does not print as f's own bytes.
func (r *aroundOf) judge(g *syntax.File, content []byte) (view, bool) {
	src := r.f.Src.Content
	pre := commonPrefix(src, content)
	if pre == len(src) && pre == len(content) {
		return view{want: content}, true
	}
	changed := span{pre, len(content) - commonSuffix(src[pre:], content[pre:])}
	gb, groot, n := itemAround(r.builderOf(g), changed)
	if n == nil {
		return view{}, false
	}
	delta := len(content) - len(src)
	fb, froot, fn := r.counterpart(g, n, delta)
	if fn == nil || !sameOutside(fb, gb, fn, n) {
		return view{}, false
	}
	gp, gok := gb.place(groot, n)
	fp, fok := fb.place(froot, fn)
	aligned := gp.ind == fp.ind && gp.bytes.lo == fp.bytes.lo && gp.bytes.hi-fp.bytes.hi == delta
	if !gok || !fok || !aligned || !bytes.Equal(renderSection(fp.d, fp.ind), src[fp.bytes.lo:fp.bytes.hi]) {
		return view{}, false
	}
	want := slices.Concat(content[:gp.bytes.lo], renderSection(gp.d, gp.ind), content[gp.bytes.hi:])
	lo, hi := gb.span(n)
	return view{b: gb, want: want, item: span{lo, hi}}, true
}

// builderOf is a builder of g, sharing the reading of the settle's first text when g is its tree.
func (r *aroundOf) builderOf(g *syntax.File) *builder {
	if r.gb != nil && r.gb.f == g {
		return r.gb.fork()
	}
	return newBuilder(g)
}

// itemAround is the smallest item of b's file holding the changed bytes whose section prints
// alone, with a builder of the file that built every item it holds, and its document.
func itemAround(b *builder, changed span) (*builder, *doc, syntax.Node) {
	b.focusOn(changed)
	n := b.laidOut(b.file(), changed.lo, changed.hi)
	if n == nil {
		return nil, nil, nil
	}
	b = b.fork()
	lo, hi := b.span(n)
	b.focusOn(span{lo, hi})
	return b, b.file(), n
}

// counterpart is the item of f standing where item n of g stands, delta bytes shorter, with a
// builder of f that built every item it holds, and f's document from that builder.
func (r *aroundOf) counterpart(g *syntax.File, n syntax.Node, delta int) (*builder, *doc, syntax.Node) {
	lo, hi := int(g.Tokens[n.First()].Start), int(g.Tokens[n.Last()].End)-delta
	b := r.fb.fork()
	b.focusOn(span{lo, hi})
	root := b.file()
	fn := b.unitAt(lo, hi)
	if fn == nil || fn.Kind() != n.Kind() || fn.First() != n.First() {
		return nil, nil, nil
	}
	if flo, fhi := b.span(fn); flo != lo || fhi != hi {
		return nil, nil, nil
	}
	return b, root, fn
}

// sameOutside reports two texts alike outside item fn of the first and gn of the second: the
// same tokens, shifted after the items, the same commas kept and comments read.
func sameOutside(fb, gb *builder, fn, gn syntax.Node) bool {
	// FORMATTER.md §8.1, DECISIONS 211, 216
	ft, gt := fb.f.Tokens, gb.f.Tokens
	shift := int(gn.Last()) - int(fn.Last())
	delta := len(gb.f.Src.Content) - len(fb.f.Src.Content)
	edges := ft[fn.First()].Kind == gt[gn.First()].Kind && ft[fn.Last()].Kind == gt[gn.Last()].Kind
	if len(gt)-len(ft) != shift || !edges {
		return false
	}
	for t := range ft {
		u, by := t, 0
		switch tok := syntax.Tok(t); {
		case tok > fn.Last():
			u, by = t+shift, delta
		case tok >= fn.First():
			continue
		}
		if !fb.sameToken(gb, t, u, by) {
			return false
		}
	}
	return true
}

// sameToken reports token t of b and token u of c alike, u by bytes further.
func (b *builder) sameToken(c *builder, t, u, by int) bool {
	x, y := b.f.Tokens[t], c.f.Tokens[u]
	return x.Kind == y.Kind && int(x.Start)+by == int(y.Start) && int(x.End)+by == int(y.End) &&
		b.drop[t] == c.drop[u] && sameNotes(b.notes[t], c.notes[u])
}

// sameNotes reports the comments of two tokens printed alike.
func sameNotes(x, y attached) bool {
	alike := func(a, b note) bool {
		return a.text == b.text && a.blank == b.blank && a.sameLine == b.sameLine && a.joined == b.joined && a.doc == b.doc
	}
	return x.blank == y.blank && slices.EqualFunc(x.lead, y.lead, alike) && slices.EqualFunc(x.trail, y.trail, alike)
}

// within reports, for a node's span, whether a span holds it.
func within(lo, hi int) func(span) bool {
	return func(s span) bool { return s.lo <= lo && hi <= s.hi }
}

// commonPrefix is how many bytes a and b start with alike.
func commonPrefix(a, b []byte) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// commonSuffix is how many bytes a and b end with alike.
func commonSuffix(a, b []byte) int {
	n := 0
	for n < len(a) && n < len(b) && a[len(a)-1-n] == b[len(b)-1-n] {
		n++
	}
	return n
}
