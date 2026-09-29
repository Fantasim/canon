package format

import (
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
)

// list is a bracketed list of items, a brace list or a ( ) or [ ] list (FORMATTER.md §13).
type list struct {
	open, close syntax.Tok
	items       []syntax.Node
	partial     bool // it holds a method's "self", which is no node
}

// index is what the builder records of a file's items: every list with its items, and the
// top-level items, which no list holds.
type index struct {
	lists []list
	byTok map[syntax.Tok]int
	owner map[syntax.Node]int
	items []syntax.Node
}

func newIndex() *index {
	return &index{byTok: map[syntax.Tok]int{}, owner: map[syntax.Node]int{}}
}

// record notes the list from open to close holding ns; a list built twice is noted once.
func (x *index) record(open, close syntax.Tok, ns []syntax.Node) {
	if _, ok := x.byTok[open]; ok {
		return
	}
	x.byTok[open] = len(x.lists)
	kept := slices.DeleteFunc(slices.Clone(ns), func(n syntax.Node) bool { return n == nil })
	x.lists = append(x.lists, list{open: open, close: close, items: kept, partial: len(kept) < len(ns)})
	for _, n := range kept {
		x.owner[n] = len(x.lists) - 1
		x.items = append(x.items, n)
	}
}

// top notes the top-level items of the file: declarations, amend blocks, translation entries.
func (x *index) top(ns []syntax.Node) {
	for _, n := range ns {
		x.owner[n] = -1
		x.items = append(x.items, n)
	}
}

// listOf is the list holding item n; false for a top-level item.
func (x *index) listOf(n syntax.Node) (list, bool) {
	i, ok := x.owner[n]
	if !ok || i < 0 {
		return list{}, false
	}
	return x.lists[i], true
}

// span is the byte range of n, its first token to its last.
func (b *builder) span(n syntax.Node) (lo, hi int) {
	return int(b.f.Tokens[n.First()].Start), int(b.f.Tokens[n.Last()].End)
}

// unitAt is the smallest item whose text holds the bytes lo to hi; nil when none does.
func (b *builder) unitAt(lo, hi int) syntax.Node {
	var best syntax.Node
	width := len(b.f.Src.Content) + 1
	for _, n := range b.idx.items {
		s, e := b.span(n)
		if s <= lo && hi <= e && e-s < width {
			best, width = n, e-s
		}
	}
	return best
}

// outer is the item holding the list that holds item n; nil for a top-level item.
func (b *builder) outer(n syntax.Node) syntax.Node {
	l, ok := b.idx.listOf(n)
	if !ok {
		return nil
	}
	return b.unitAt(int(b.f.Tokens[l.open].Start), int(b.f.Tokens[l.close].End))
}

// nodesOf are the nodes of a list's entries or parts; a method's "self" has none, nil.
func nodesOf[T interface{ node() syntax.Node }](ts []T) []syntax.Node {
	out := make([]syntax.Node, len(ts))
	for i, t := range ts {
		out[i] = t.node()
	}
	return out
}
