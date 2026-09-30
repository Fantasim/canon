package format

import (
	"cmp"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// file is the layout of a whole file (FORMATTER.md §4, §9).
func (b *builder) file() *doc {
	f := b.f
	if f.FileKind == syntax.FileProject {
		b.idx.top([]syntax.Node{f.Project})
		return cat(b.item(f.Project), b.eof(false))
	}
	head := cat(b.keyword(b.before(f.Package.First()), true), spaceDoc, b.node(f.Package))
	switch {
	case f.Layer != nil:
		head = cat(head, hardlineDoc, b.keyword(b.before(f.Layer.First()), false), spaceDoc, b.node(f.Layer))
	case f.Lang != nil:
		head = cat(head, hardlineDoc, b.keyword(b.before(f.Lang.First()), false), spaceDoc, b.node(f.Lang))
	}
	imports := b.imports()
	b.idx = newIndex() // the header holds no item §13 edits (log-2026-09-29 M4 U1r)
	items := slices.Concat(entries(b, f.Decls), entries(b, f.Amends), entries(b, f.Entries))
	b.idx.top(nodesOf(items))
	eof := b.eof(len(items) == 0)
	return cat(head, imports, b.topItems(items, joinedAfter(eof)), eof)
}

// keyword is a keyword token that starts a line of its own, its own-line comments before it.
func (b *builder) keyword(t syntax.Tok, blanks bool) *doc {
	return cat(b.lead(t, blanks), text(b.raw(t)), b.trail(t))
}

// imports is the import block: sorted by path then alias, without blank lines, after one
// blank line.
func (b *builder) imports() *doc {
	if len(b.f.Imports) == 0 {
		return nil
	}
	sorted := slices.Clone(b.f.Imports)
	slices.SortStableFunc(sorted, func(x, y *syntax.Import) int {
		return cmp.Or(strings.Compare(pathText(x.Path), pathText(y.Path)), strings.Compare(aliasText(x), aliasText(y)))
	})
	ds := []*doc{hardlineDoc, blankDoc}
	for i, imp := range sorted {
		if i > 0 {
			ds = append(ds, hardlineDoc)
		}
		ds = append(ds, b.node(imp))
	}
	return cat(ds...)
}

func pathText(q *syntax.QualifiedName) string {
	parts := make([]string, len(q.Parts))
	for i, p := range q.Parts {
		parts[i] = p.Name
	}
	return strings.Join(parts, syntax.TokDot.String())
}

func aliasText(imp *syntax.Import) string {
	if imp.Alias == nil {
		return ""
	}
	return imp.Alias.Name
}

// importDecl is "import path [as alias] [{ names }]", the names sorted as bytes (§9.1).
func (b *builder) importDecl(n *syntax.Import) *doc {
	ds := []*doc{b.tok(n.First()), spaceDoc, b.node(n.Path)}
	if n.Alias != nil {
		ds = append(ds, spaceDoc, b.tok(b.before(n.Alias.First())), spaceDoc, b.node(n.Alias))
	}
	if n.Braces.Open != syntax.NoTok {
		names := slices.Clone(n.Names)
		slices.SortStableFunc(names, func(x, y *syntax.Ident) int { return strings.Compare(x.Name, y.Name) })
		items := make([]entry, len(names))
		for i, id := range names {
			items[i] = entry{n: id, d: b.node(id)}
		}
		ds = append(ds, spaceDoc, b.braceList(n.Braces.Open, n.Braces.Close, items))
	}
	return cat(ds...)
}

// topItems are the declarations of a file, one per line, after a blank line; a blank line
// between two of them is kept. joined: the file's closing comments start on the last one's line.
func (b *builder) topItems(items []entry, joined bool) *doc {
	var ds []*doc
	for i, e := range items {
		if e.d == nil { // left unbuilt by a focus
			continue
		}
		var gap *doc
		if e.gap || i == 0 {
			gap = blankDoc
		}
		part := cat(gap, e.d, e.trail)
		ds = append(ds, hardlineDoc, part)
		b.notePart(e.n, part, i == len(items)-1 && joined)
	}
	return cat(ds...)
}

// eof is the comments at the end of the file, after the last item; after the header alone
// the first follows a blank line.
func (b *builder) eof(afterHeader bool) *doc {
	notes := b.notes[b.f.Last()].lead
	ds := make([]*doc, 0, len(notes))
	for i, n := range notes {
		ds = append(ds, comment(n, n.blank || i == 0 && afterHeader))
	}
	return cat(ds...)
}

// qualifiedName is a dotted name.
func (b *builder) qualifiedName(n *syntax.QualifiedName) *doc {
	var ds []*doc
	for i, p := range n.Parts {
		if i > 0 {
			ds = append(ds, b.tok(b.before(p.First())))
		}
		ds = append(ds, b.node(p))
	}
	return cat(ds...)
}

// project is "project name { items }" (GRAMMAR.md §7).
func (b *builder) project(n *syntax.ProjectDecl) *doc {
	return cat(b.tok(n.First()), spaceDoc, b.node(n.Name), spaceDoc, b.braceList(n.Braces.Open, n.Braces.Close, entries(b, n.Items)))
}

// sugared reports a map-valued project key written "key: { … }", printed "key { … }" (§9.3).
func sugared(e *syntax.ProjectEntry) bool {
	_, isMap := e.Value.(*syntax.ProjectMap)
	return isMap && e.Colon != syntax.NoTok
}

// projectEntry is "key: value", or "key { … }" for a map value at the top of the project.
func (b *builder) projectEntry(n *syntax.ProjectEntry) *doc {
	top := false
	if p := b.f.Project; p != nil {
		top = slices.Contains(p.Items, n)
	}
	if _, isMap := n.Value.(*syntax.ProjectMap); isMap && top && (n.Colon == syntax.NoTok || b.drop[n.Colon]) {
		return cat(b.node(n.Key), spaceDoc, b.node(n.Value))
	}
	return cat(b.node(n.Key), b.assign(n.Colon, n.Value))
}
