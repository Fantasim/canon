package format

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
)

// ChangeKind is what a Change does (FORMATTER.md §13).
type ChangeKind uint8

// Change is one change on Rewrite's tree: Replace writes Text over Node, Insert makes Text item
// At of the list List opens (no List: a declaration appended, At their count), Remove deletes
// item Node, Retire marks Node retired, Move carries item Node of List with its comments to At.
type Change struct {
	Kind ChangeKind
	Node syntax.Node
	List syntax.Tok
	At   int
	Text string
}

// expect is what the new text of a mark must parse as: one node, like like, or an item.
type expect struct {
	check     bool
	like      syntax.Node
	item, top bool
}

// mark is a part of an edit's text: new text (fresh) and an item to re-print (unit).
type mark struct {
	lo, hi      int
	unit, fresh bool
	want        expect
}

// edit is one change of the text before any re-printing: the bytes lo to hi become text.
type edit struct {
	lo, hi int
	text   string
	marks  []mark
}

// span is a byte range of a text.
type span struct{ lo, hi int }

// Rewrite applies changes to f, a tree without syntax error, and re-prints the items they touch,
// every other byte kept; from a fixed point of the formatter the result is one too. Its errors
// wrap ErrSyntax, ErrLayout, ErrChange, ErrText or ErrUnsettled.
func Rewrite(f *syntax.File, changes []Change) ([]byte, error) {
	// FORMATTER.md §13, API.md M5
	l := layouts.of(f)
	if l.err != nil {
		return nil, l.err
	}
	b := newBuilder(f)
	hull, around := replaced(f, changes)
	if around = around && l.canonical; around {
		b.focusOn(hull)
	}
	b.file()
	return rewriteWith(b, changes, l.canonical, around)
}

// rewriteWith is Rewrite on b's file, whose items b has built, all of them or, around, the ones
// the changes touch; canonical, the file is a fixed point before them.
func rewriteWith(b *builder, changes []Change, canonical, around bool) ([]byte, error) {
	edits, err := b.edits(changes)
	if err != nil {
		return nil, err
	}
	content, marks, err := apply(b.f.Src.Content, edits)
	if err != nil {
		return nil, err
	}
	var gb *builder
	if content, gb, err = reprintUnits(b.f, content, marks, around); err != nil || !canonical {
		return content, err
	}
	if around {
		return settleLoop(b.f, content, gb.f, (&aroundOf{f: b.f, fb: b, gb: gb}).step)
	}
	return settleLoop(b.f, content, nil, settleStep)
}

// replaced is the bytes every change holds, true when each is a Replace of a node of f, a file
// with declarations: Rewrite then builds only the items around them.
func replaced(f *syntax.File, changes []Change) (span, bool) {
	hull := span{len(f.Src.Content), 0}
	for _, c := range changes {
		if c.Kind != Replace || !holds(f, c.Node) {
			return hull, false
		}
		hull.lo = min(hull.lo, int(f.Tokens[c.Node.First()].Start))
		hull.hi = max(hull.hi, int(f.Tokens[c.Node.Last()].End))
	}
	return hull, len(changes) > 0 && f.FileKind != syntax.FileProject
}

// edits are the text changes of changes: Replace and Retire alone, Insert, Remove and Move
// grouped by list, each list planned once over its final items (log-2026-09-29 M4 U1r, U1b).
func (b *builder) edits(changes []Change) ([]edit, error) {
	var out []edit
	gs := &batches{byList: map[syntax.Tok]*batch{}}
	for _, c := range changes {
		var err error
		switch {
		case c.Kind >= changeKindCount || c.Kind != Insert && !holds(b.f, c.Node):
			err = fmt.Errorf("%w: kind %d on no node of the tree", ErrChange, c.Kind)
		case c.Kind == Replace:
			out, err = b.replaceEdit(out, c)
		case c.Kind == Retire:
			out, err = b.retireEdit(out, c)
		default:
			err = b.collect(gs, c)
		}
		if err != nil {
			return nil, err
		}
	}
	for _, open := range gs.order {
		es, err := b.plan(gs.byList[open])
		if err != nil {
			return nil, err
		}
		out = append(out, es...)
	}
	return out, nil
}

// apply makes the edits, which must not overlap, and places their marks in the result.
func apply(src []byte, edits []edit) ([]byte, []mark, error) {
	slices.SortStableFunc(edits, func(x, y edit) int { return cmp.Or(x.lo-y.lo, x.hi-y.hi) })
	var out []byte
	var marks []mark
	last := 0
	for _, e := range edits {
		if e.lo < last {
			return nil, nil, fmt.Errorf("%w: two changes overlap at offset %d", ErrChange, e.lo)
		}
		out = append(out, src[last:e.lo]...)
		at := len(out)
		out = append(out, e.text...)
		last = e.hi
		for _, m := range e.marks {
			m.lo, m.hi = m.lo+at, m.hi+at
			marks = append(marks, m)
		}
	}
	return append(out, src[last:]...), marks, nil
}

// reprintUnits checks that each new text parses as the node it stands for, then re-prints the
// item holding each unit (steps 1 to 3); an item inside another re-printed one is left to it.
// Around, only the items holding a mark are built. The builder returned is content's, before.
func reprintUnits(f *syntax.File, content []byte, marks []mark, around bool) ([]byte, *builder, error) {
	g, err := reparse(f, content)
	if err != nil {
		return nil, nil, err
	}
	b := newBuilder(g)
	b.fresh = func(t syntax.Tok) bool {
		at := int(g.Tokens[t].Start)
		return slices.ContainsFunc(marks, func(m mark) bool { return m.fresh && m.lo <= at && at < m.hi })
	}
	if around && len(marks) > 0 {
		b.focusOn(hullOf(marks))
	}
	b.file()
	var out []splice
	for _, m := range marks {
		if m.want.check && !b.stands(m) {
			return nil, nil, fmt.Errorf("%w: at offset %d", ErrText, m.lo)
		}
		if n := b.unitAt(m.lo, m.hi); m.unit && n != nil {
			out = append(out, b.unit(n))
		}
	}
	slices.SortFunc(out, func(x, y splice) int { return cmp.Or(x.lo-y.lo, y.hi-x.hi) })
	var buf []byte
	last := 0
	for _, s := range out {
		if s.lo < last {
			continue
		}
		buf = append(append(buf, content[last:s.lo]...), s.text...)
		last = s.hi
	}
	return append(buf, content[last:]...), b, nil
}

// hullOf is the bytes from the first mark to the end of the last.
func hullOf(marks []mark) span {
	h := span{marks[0].lo, marks[0].hi}
	for _, m := range marks[1:] {
		h.lo, h.hi = min(h.lo, m.lo), max(h.hi, m.hi)
	}
	return h
}

// replaceEdit writes Text over Node, which an item must hold: only items are edited.
func (b *builder) replaceEdit(out []edit, c Change) ([]edit, error) {
	// FORMATTER.md §13, log-2026-09-29 M4 U1r
	t := trimmed(c.Text)
	lo, hi := b.span(c.Node)
	if t == "" || c.Node.Kind() == syntax.KindFile || b.unitAt(lo, hi) == nil {
		return nil, fmt.Errorf("%w: nothing to replace, or no item holds it", ErrChange)
	}
	m := mark{hi: len(t), unit: true, fresh: true, want: expect{check: true, like: c.Node}}
	return append(out, edit{lo: lo, hi: hi, text: t, marks: []mark{m}}), nil
}

// retireEdit writes "retired " before the key of an entry, table entry, member or case.
func (b *builder) retireEdit(out []edit, c Change) ([]edit, error) {
	// FORMATTER.md §13 step 6, API.md N7
	var key syntax.Tok
	var mods *syntax.Modifiers
	switch n := c.Node.(type) {
	case *syntax.EntryDecl:
		key, mods = n.First(), n.Mods
		if len(n.Annotations) > 0 {
			key = b.after(n.Annotations[len(n.Annotations)-1].Last())
		}
	case *syntax.EntryItem:
		key, mods = n.Key.First(), n.Mods
	case *syntax.EnumMember:
		key, mods = n.Name.First(), n.Mods
	case *syntax.VariantCase:
		key, mods = n.Name.First(), n.Mods
	default:
		return nil, fmt.Errorf("%w: only an entry, a member or a case retires", ErrChange)
	}
	if mods != nil {
		return nil, fmt.Errorf("%w: already retired", ErrChange)
	}
	at := int(b.f.Tokens[key].Start)
	word := syntax.KwRetired.String() + space
	return append(out, edit{lo: at, hi: at, text: word, marks: []mark{{hi: len(word), unit: true}}}), nil
}
