package project

import (
	"github.com/fantasim/canonlang/internal/syntax"
)

// graft gives nf, a new parse of old's path, old's header parts and declarations that are alike
// and wholly before the first changed byte, so their spans stay a fresh parse's; never an entry,
// which the evaluation memo keys by node. nf is changed before anyone else holds it.
func graft(old, nf *syntax.File) {
	if !graftable(old) || !graftable(nf) {
		return
	}
	g := grafter{old: old, nf: nf, limit: firstDiff(old.Src.Content, nf.Src.Content)}
	if g.docShared(old.Doc, nf.Doc) {
		nf.Doc = old.Doc
	}
	if old.Package != nil && nf.Package != nil && g.alike(old.Package, nf.Package) {
		nf.Package = old.Package
	}
	for i := range min(len(old.Imports), len(nf.Imports)) {
		if g.alike(old.Imports[i], nf.Imports[i]) {
			nf.Imports[i] = old.Imports[i]
		}
	}
	for i := range min(len(old.Decls), len(nf.Decls)) {
		if shareable(old.Decls[i]) && g.alike(old.Decls[i], nf.Decls[i]) {
			nf.Decls[i] = old.Decls[i]
		}
	}
}

// graftable reports a source file whose parse may share nodes: one holding only a header and
// declarations.
func graftable(f *syntax.File) bool {
	return f != nil && f.FileKind == syntax.FileSource && f.Layer == nil && f.Lang == nil && f.Project == nil &&
		len(f.Amends)+len(f.Entries) == 0
}

// shareable reports a declaration graft may share: any but an `entry` or one that did not parse.
func shareable(d syntax.Decl) bool {
	switch d.(type) {
	case *syntax.EntryDecl, *syntax.BadDecl:
		return false
	}
	return true
}

// firstDiff is the offset of the first byte a and b differ in, the shorter's length when one
// starts the other.
func firstDiff(a, b []byte) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// grafter compares nodes of old and nf below limit, the first byte their contents differ in.
type grafter struct {
	old, nf *syntax.File
	limit   int
}

// docShared reports two doc comments alike that end before limit.
func (g grafter) docShared(a, b *syntax.DocComment) bool {
	return a != nil && b != nil && a.Start == b.Start && a.End == b.End && int(a.End) <= g.limit && a.Text == b.Text
}

// alike reports two nodes of one shape over the same tokens, all of them ending before limit:
// each node of a, in walk order, has the kind and token bounds of b's, and each token the same
// kind and offsets in both files.
func (g grafter) alike(a, b syntax.Node) bool {
	na, nb := walked(a), walked(b)
	if len(na) != len(nb) {
		return false
	}
	for i, x := range na {
		y := nb[i]
		if x.Kind() != y.Kind() || x.First() != y.First() || x.Last() != y.Last() || !g.docsAlike(x, y) {
			return false
		}
	}
	return g.tokensAlike(a.First(), a.Last())
}

// docsAlike reports two nodes alike as doc comments when they are ones.
func (g grafter) docsAlike(x, y syntax.Node) bool {
	dx, ok := x.(*syntax.DocComment)
	if !ok {
		return true
	}
	return g.docShared(dx, y.(*syntax.DocComment))
}

// tokensAlike reports tokens from to last (inclusive) alike in both files, the last ending
// before limit.
func (g grafter) tokensAlike(from, last syntax.Tok) bool {
	if last < from || int(last) >= len(g.old.Tokens) || int(last) >= len(g.nf.Tokens) {
		return false
	}
	if int(g.old.Tokens[last].End) > g.limit {
		return false
	}
	for t := from; t <= last; t++ {
		a, b := g.old.Tokens[t], g.nf.Tokens[t]
		if a.Kind != b.Kind || a.Start != b.Start || a.End != b.End {
			return false
		}
	}
	return true
}

// walked is every node under n, n first, in walk order.
func walked(n syntax.Node) []syntax.Node {
	var out []syntax.Node
	syntax.Inspect(n, func(x syntax.Node) bool {
		if x != nil {
			out = append(out, x)
		}
		return true
	})
	return out
}
