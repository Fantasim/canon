package format

import (
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
)

// entry is an item of a brace list and apart its trailing comments, whether a blank line
// precedes it in the input, whether its first token is in joinsLine, and whether its last token
// is in cannotEndItem (DECISIONS 211).
type entry struct {
	n                         syntax.Node
	d, trail                  *doc
	gap, joins, endJoin, kept bool
}

func (e entry) node() syntax.Node { return e.n }

// entries are the nodes of a brace list as its items (§4, §8).
func entries[N syntax.Node](b *builder, ns []N) []entry {
	out := make([]entry, len(ns))
	for i, n := range ns {
		joins := joinsLine[b.f.Tokens[n.First()].Kind]
		endJoin := cannotEndItem[b.f.Tokens[n.Last()].Kind]
		body, trail := b.parts(n, true)
		out[i] = entry{n: n, d: body, trail: trail, gap: b.gap(n.First()), joins: joins, endJoin: endJoin, kept: b.keeps(n.Last())}
	}
	return out
}

// braceList is BL(items) of FORMATTER.md §7.2: single-line by §6.1, an empty one always so.
func (b *builder) braceList(open, close syntax.Tok, items []entry) *doc {
	return listGroup(b.single(open, close, items), b.braceParts(open, close, items))
}

// single is a brace list's single-line bit: written on one line without a comment, or, created
// by the edit API, holding no list in its items.
func (b *builder) single(open, close syntax.Tok, items []entry) bool {
	// FORMATTER.md §6.1, §6.3
	switch {
	case b.inner(open, close):
		return false
	case b.fresh(open):
		return !slices.ContainsFunc(items, func(e entry) bool { return holdsList(e.n) })
	default:
		return len(items) == 0 || b.oneLine(open, close)
	}
}

// holdsList reports a brace literal or a list literal, comprehensions included, in n.
func holdsList(n syntax.Node) bool {
	// FORMATTER.md §6.3, log-2026-09-29 M4 U1r
	found := false
	syntax.Inspect(n, func(c syntax.Node) bool {
		if c != nil && (c.Kind() == syntax.KindBraceLit || c.Kind() == syntax.KindListLit || c.Kind() == syntax.KindListComp) {
			found = true
		}
		return !found
	})
	return found
}

// braceParts is a brace list without its group, for an if chain whose blocks share one (§7.2).
func (b *builder) braceParts(open, close syntax.Tok, items []entry) *doc {
	b.idx.record(open, close, nodesOf(items))
	body := b.listBody(close, items)
	if body == nil && !b.inner(open, close) {
		return cat(b.tok(open), b.closer(close))
	}
	return cat(b.tok(open), body, lineDoc, b.closer(close))
}

// listBody is the indented part of a brace list, its closing comments included; nil when
// the list is empty and holds no comment.
func (b *builder) listBody(close syntax.Tok, items []entry) *doc {
	var ds []*doc
	comma := text(syntax.TokComma.String())
	for i, e := range items {
		ds = append(ds, lineDoc)
		if e.gap && i > 0 {
			ds = append(ds, blankDoc)
		}
		switch {
		case i == len(items)-1 || e.kept:
			ds = append(ds, e.d, e.trail)
		case e.endJoin || items[i+1].joins:
			ds = append(ds, e.d, comma, e.trail)
		default:
			ds = append(ds, e.d, e.trail, ifBreak(emptyDoc, comma))
		}
	}
	ends := b.closing(close, len(items) > 0)
	if len(ds) == 0 && ends == nil {
		return nil
	}
	return indent(append(ds, ends)...)
}

// closing is the own-line comments before a closing bracket, printed with the list's items;
// the first keeps the blank line before it when items precede it.
func (b *builder) closing(close syntax.Tok, items bool) *doc {
	if b.heldLead[close] {
		return nil
	}
	notes := b.notes[close].lead
	ds := make([]*doc, 0, len(notes))
	for i, n := range notes {
		n.joined = false
		ds = append(ds, comment(n, n.blank && (i > 0 || items)))
	}
	if len(ds) == 0 {
		return nil
	}
	return cat(ds...)
}

// part is an item of a parenthesized list: its text with its leading comments, apart its
// trailing comments, which follow the comma after it, and whether that comma is kept.
type part struct {
	n           syntax.Node
	body, trail *doc
	kept        bool
}

func (p part) node() syntax.Node { return p.n }

// partsOf are the nodes of a parenthesized list as its items.
func partsOf[N syntax.Node](b *builder, ns []N) []part {
	out := make([]part, len(ns))
	for i, n := range ns {
		out[i].n = n
		out[i].body, out[i].trail = b.parts(n, false)
		out[i].kept = b.keeps(n.Last())
	}
	return out
}

// separator is the comma written after an item, none when the item's own comma is kept.
func separator(kept bool) *doc {
	if kept {
		return nil
	}
	return text(syntax.TokComma.String())
}

// parenList is PL(open, items, close) of FORMATTER.md §7.2; trailing comments follow the comma.
func (b *builder) parenList(open, close syntax.Tok, items []part) *doc {
	b.idx.record(open, close, nodesOf(items))
	var ds []*doc
	for i, it := range items {
		sep, trail := separator(it.kept), it.trail
		if i == len(items)-1 {
			sep = ifBreak(cat(sep), emptyDoc)
		} else {
			trail = cat(trail, lineDoc)
		}
		ds = append(ds, it.body, sep, trail)
	}
	ends := b.closing(close, len(items) > 0)
	if len(items) == 0 && ends == nil {
		return cat(b.tok(open), b.closer(close))
	}
	return group(false, b.tok(open), indent(softlineDoc, cat(ds...), ends), softlineDoc, b.closer(close))
}

// flatList is a parenthesized or bracketed list that never breaks: type and annotation
// arguments.
func (b *builder) flatList(open, close syntax.Tok, items []part) *doc {
	b.idx.record(open, close, nodesOf(items))
	var ds []*doc
	for i, it := range items {
		if i < len(items)-1 {
			ds = append(ds, it.body, separator(it.kept), it.trail, lineDoc)
		} else {
			ds = append(ds, it.body, it.trail)
		}
	}
	ends := b.closing(close, len(items) > 0)
	return flat(cat(b.tok(open), indent(cat(ds...), ends), b.closer(close)))
}

// nodes converts a list of nodes of one type.
func nodes[N syntax.Node](ns []N) []syntax.Node {
	out := make([]syntax.Node, len(ns))
	for i, n := range ns {
		out[i] = n
	}
	return out
}

// commaList is ns joined by ", " with their comments, for lists without brackets of their own:
// patterns, binders.
func (b *builder) commaList(ns []syntax.Node) *doc {
	var ds []*doc
	for i, n := range ns {
		body, trail := b.parts(n, false)
		if i < len(ns)-1 {
			ds = append(ds, body, separator(b.keeps(n.Last())), trail, spaceDoc)
		} else {
			ds = append(ds, body, trail)
		}
	}
	return cat(ds...)
}
