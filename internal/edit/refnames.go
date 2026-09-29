package edit

import (
	"sort"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// nameRefs are the names of the target in the files of every package but those inside a
// stated value (API.md R7): each is code, unless in a view or a translation, a check or an amendment.
func (s *Snapshot) nameRefs(tg *target, covers []source.Span) []Ref {
	sc := &nameScan{s: s, tg: tg, covers: covers}
	for _, pkg := range s.pkgs {
		sc.pkg = pkg.Path
		for _, f := range pkg.Files {
			sc.file = f
			sc.walk(f, RefCode)
		}
	}
	return sc.out
}

// nameScan walks the syntax of one package's files at a time.
type nameScan struct {
	s      *Snapshot
	tg     *target
	covers []source.Span
	pkg    string
	file   *syntax.File
	out    []Ref
}

// walk reports n when it names the target, then its children, under the kind of the
// construct holding them.
func (sc *nameScan) walk(n syntax.Node, kind RefKind) {
	switch n.(type) {
	case *syntax.ViewDecl, *syntax.TranslationEntry:
		kind = RefView
	case *syntax.CheckDecl:
		kind = RefCheck
	case *syntax.AmendBlock:
		kind = RefLayer
	}
	if at, ok := sc.names(n); ok {
		if sp := sc.file.Span(at); !sc.covered(sp) {
			sc.out = append(sc.out, Ref{Kind: kind, Package: sc.pkg, Span: sp})
		}
	}
	for c := range syntax.Children(n) {
		sc.walk(c, kind)
	}
}

// names is the node naming the target in n, if n does: a name the checker resolved to one of
// its declarations, or a symbolic key or key literal it recorded for the target's collection.
func (sc *nameScan) names(n syntax.Node) (syntax.Node, bool) {
	info := sc.s.info
	switch x := n.(type) {
	case *syntax.IdentExpr:
		return x, sc.isObject(info.Uses[x]) || sc.isKey(x, x.Name)
	case *syntax.Ident:
		return x, sc.isObject(info.NameUses[x])
	case *syntax.SelectorExpr:
		return x.Name, sc.isKey(x, x.Name.Name)
	case *syntax.StringLit, *syntax.RawStringLit:
		text, ok := constString(x.(syntax.Expr))
		return x, ok && sc.isKey(x.(syntax.Expr), text)
	case *syntax.IntLit:
		id := sc.tg.ident
		return x, id != nil && id.Key.IsInt && x.Value.IsInt64() && x.Value.Int64() == id.Key.I && sc.inColl(x)
	}
	return nil, false
}

// covered reports a name inside the span a value reference is stated at: the same reference.
// Covers are sorted tokens, disjoint but for a selector and its name, so the last one starting
// at or before sp is the one that can hold it.
func (sc *nameScan) covered(sp source.Span) bool {
	i := sort.Search(len(sc.covers), func(k int) bool { return spanOrder(sc.covers[k], sp) > 0 })
	return i > 0 && contains(sc.covers[i-1], sp)
}

func (sc *nameScan) isObject(o check.Object) bool {
	return o != nil && sc.tg.objects[o]
}

// isKey reports a symbolic key or key literal of the target's collection with the target's key.
func (sc *nameScan) isKey(e syntax.Expr, text string) bool {
	id := sc.tg.ident
	return id != nil && !id.Key.IsInt && id.Key.S == text && sc.inColl(e)
}

func (sc *nameScan) inColl(e syntax.Expr) bool {
	coll := sc.s.info.Keys[e]
	return coll != nil && coll == sc.tg.ident.Coll
}
