package format

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
)

// ChangeKind is what a Change does (FORMATTER.md §13).
type ChangeKind uint8

// Change is one change on Rewrite's tree: Replace writes Text over Node, Insert makes Text item
// At of the list List opens (with no List, a declaration appended to the file, At the count of
// its declarations), Remove deletes item Node, Retire marks Node retired.
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
	if err := usable(f); err != nil {
		return nil, err
	}
	b := newBuilder(f)
	canonical := bytes.Equal(render(b.file()), f.Src.Content)
	edits, err := b.edits(changes)
	if err != nil {
		return nil, err
	}
	content, marks, err := apply(f.Src.Content, edits)
	if err != nil {
		return nil, err
	}
	if content, err = reprintUnits(f, content, marks); err != nil || !canonical {
		return content, err
	}
	return settle(f, content)
}

// edits are the text changes of changes: Replace and Retire alone, Insert and Remove grouped by
// list, each list planned once over its final items (log-2026-09-29 M4 U1r).
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
func reprintUnits(f *syntax.File, content []byte, marks []mark) ([]byte, error) {
	g, err := reparse(f, content)
	if err != nil {
		return nil, err
	}
	b := newBuilder(g)
	b.fresh = func(t syntax.Tok) bool {
		at := int(g.Tokens[t].Start)
		return slices.ContainsFunc(marks, func(m mark) bool { return m.fresh && m.lo <= at && at < m.hi })
	}
	b.file()
	var out []splice
	for _, m := range marks {
		if m.want.check && !b.stands(m) {
			return nil, fmt.Errorf("%w: at offset %d", ErrText, m.lo)
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
	return append(buf, content[last:]...), nil
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
