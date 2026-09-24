package format

import (
	"github.com/fantasim/canonlang/internal/syntax"
)

// entry is an item of a brace list and apart its trailing comments, whether a blank line
// precedes it in the input, and whether it starts with a "." that would continue the line
// before it.
type entry struct {
	d, trail *doc
	gap, dot bool
}

// entries are the nodes of a brace list as its items (§4, §8).
func entries[N syntax.Node](b *builder, ns []N) []entry {
	out := make([]entry, len(ns))
	for i, n := range ns {
		dot := b.f.Tokens[n.First()].Kind == syntax.TokDot
		body, trail := b.parts(n, true)
		out[i] = entry{d: body, trail: trail, gap: b.gap(n.First()), dot: dot}
	}
	return out
}

// braceList is BL(items) of FORMATTER.md §7.2, forced broken unless written on one line (§6.1).
func (b *builder) braceList(open, close syntax.Tok, items []entry) *doc {
	return listGroup(b.oneLine(open, close) && !b.inner(open, close), b.braceParts(open, close, items))
}

// braceParts is a brace list without its group, for an if chain whose blocks share one (§7.2).
func (b *builder) braceParts(open, close syntax.Tok, items []entry) *doc {
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
		case i == len(items)-1:
			ds = append(ds, e.d, e.trail)
		case items[i+1].dot:
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

// part is an item of a parenthesized list: its text with its leading comments, and apart its
// trailing comments, which follow the comma after it.
type part struct {
	body, trail *doc
}

// partsOf are the nodes of a parenthesized list as its items.
func partsOf[N syntax.Node](b *builder, ns []N) []part {
	out := make([]part, len(ns))
	for i, n := range ns {
		out[i].body, out[i].trail = b.parts(n, false)
	}
	return out
}

// parenList is PL(open, items, close) of FORMATTER.md §7.2; trailing comments follow the comma.
func (b *builder) parenList(open, close syntax.Tok, items []part) *doc {
	var ds []*doc
	for i, it := range items {
		sep, trail := text(syntax.TokComma.String()), it.trail
		if i == len(items)-1 {
			sep = ifBreak(sep, emptyDoc)
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
	var ds []*doc
	for i, it := range items {
		if i < len(items)-1 {
			ds = append(ds, it.body, text(syntax.TokComma.String()), it.trail, lineDoc)
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
			ds = append(ds, body, text(syntax.TokComma.String()), trail, spaceDoc)
		} else {
			ds = append(ds, body, trail)
		}
	}
	return cat(ds...)
}
