package edit

import (
	"sort"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// nameRefs are the names of the target in the files of every package but those inside a
// stated value (API.md R7): each is code, unless in a view or a translation, a check or an amendment.
func (s *Snapshot) nameRefs(tg *target, covers []source.Span) []Ref {
	sc := &nameScan{s: s, tg: tg, covers: covers, reach: reaches(covers)}
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
	reach  []source.Pos // the furthest end of the covers of one file up to each
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
		return x.Name, sc.isKey(x, x.Name.Name) && sc.throughOwn(x) || sc.isEntryOf(x)
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
// Covers are sorted by start; one may hold another (a selector and its name), so every earlier
// cover whose file reaches past sp is tried, nearest first (log-2026-09-29 M4 U4a).
func (sc *nameScan) covered(sp source.Span) bool {
	i := sort.Search(len(sc.covers), func(k int) bool { return spanOrder(sc.covers[k], sp) > 0 })
	for k := i - 1; k >= 0 && sc.covers[k].File == sp.File && sc.reach[k] >= sp.End; k-- {
		if contains(sc.covers[k], sp) {
			return true
		}
	}
	return false
}

// reaches is, for each sorted cover, the furthest end of the covers of its file up to it.
func reaches(covers []source.Span) []source.Pos {
	out := make([]source.Pos, len(covers))
	for k, c := range covers {
		out[k] = c.End
		if k > 0 && covers[k-1].File == c.File {
			out[k] = max(out[k], out[k-1])
		}
	}
	return out
}

func (sc *nameScan) isObject(o check.Object) bool {
	return o != nil && sc.tg.objects[o]
}

// isKey reports a symbolic key or key literal of the target's collection with the target's key.
func (sc *nameScan) isKey(e syntax.Expr, text string) bool {
	id := sc.tg.ident
	return id != nil && !id.Key.IsInt && id.Key.S == text && sc.inColl(e)
}

// isEntryOf reports `c.key` naming the target through a qualified collection `pkg.c` of
// another package, whose entry the checker selects without an object (API.md R7).
func (sc *nameScan) isEntryOf(x *syntax.SelectorExpr) bool {
	sel, id := sc.s.info.Selections[x], sc.tg.ident
	if sel == nil || sel.Kind != check.SelEntry || sel.Obj != nil || id == nil || id.Key.IsInt || id.Key.S != x.Name.Name {
		return false
	}
	var obj check.Object
	switch r := syntax.Unparen(x.X).(type) {
	case *syntax.IdentExpr:
		obj = sc.s.info.Uses[r]
	case *syntax.SelectorExpr:
		obj = sc.s.info.NameUses[r.Name]
	}
	coll := id.Coll
	return obj != nil && coll != nil && coll.Kind == types.CollLet && len(coll.FieldPath) == 0 &&
		obj.Kind() == check.ObjLet && obj.Pkg() == coll.Pkg && obj.Name() == coll.Name
}

// throughOwn reports the key lookup x reaching the target's own collection: a let's always; a
// record field's, shared by every instance, only when x's receiver is a let root with literal
// indices and keys all the way to the target's owner (log-2026-09-29 M4 Cleanup-A-r).
func (sc *nameScan) throughOwn(x *syntax.SelectorExpr) bool {
	if coll := sc.s.info.Keys[x]; coll == nil || coll.Kind != types.CollField {
		return true
	}
	field, ok := syntax.Unparen(x.X).(*syntax.SelectorExpr)
	if !ok || field.X == nil {
		return false
	}
	p, ok := sc.s.StaticPath(field.X)
	if !ok {
		return false
	}
	res, err := Resolve(sc.s, p)
	owner, isRec := res.Target.(*value.Record)
	return err == nil && isRec && owner == sc.tg.ident.Owner
}

func (sc *nameScan) inColl(e syntax.Expr) bool {
	coll := sc.s.info.Keys[e]
	return coll != nil && coll == sc.tg.ident.Coll
}
