package edit

import (
	"slices"
	"sort"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// nameRefs are the names of the target in the files of every package but those inside a
// stated value (API.md R7): each is code, unless in a view or a translation, a check or an
// amendment; and the paths where a shared token names the target among other entries (refsOf).
func (s *Snapshot) nameRefs(tg *target, covers []source.Span, uses []sharedUse) ([]Ref, []string) {
	sc := &nameScan{s: s, tg: tg, covers: covers, reach: reaches(covers), uses: uses, built: map[transientKey]bool{}, calls: map[check.Object]bool{}}
	for _, pkg := range s.pkgs {
		sc.pkg = pkg.Path
		for _, f := range pkg.Files {
			sc.file = f
			sc.walk(f, scanAt{kind: RefCode})
		}
	}
	slices.Sort(sc.shared)
	return sc.out, slices.Compact(sc.shared)
}

// nameScan walks the syntax of one package's files at a time.
type nameScan struct {
	s      *Snapshot
	tg     *target
	covers []source.Span
	reach  []source.Pos // the furthest end of the covers of one file up to each
	uses   []sharedUse
	pkg    string
	file   *syntax.File
	out    []Ref
	shared []string
	built  map[transientKey]bool // whether a function or a lambda builds a record without a field
	calls  map[check.Object]bool // whether code uses a function
}

// scanAt is where a name lies: the kind of the construct holding it, the amendment line and
// block, the record and field declaring it, and whether a default, constant, function or lambda,
// which instances share, holds it.
type scanAt struct {
	kind   RefKind
	block  *syntax.AmendBlock
	line   *syntax.Amendment
	segKey syntax.Node       // the node of a line's segment the walk meets for its key (segToken)
	index  *syntax.IndexExpr // the nearest index expression, whose key instKey may judge
	record *syntax.RecordDecl
	field  *syntax.FieldDecl
	shared bool
}

// enter is at inside n.
func (at scanAt) enter(n syntax.Node) scanAt {
	switch x := n.(type) {
	case *syntax.ViewDecl, *syntax.TranslationEntry:
		at.kind = RefView
	case *syntax.CheckDecl:
		at.kind = RefCheck
	case *syntax.AmendBlock:
		at.kind, at.block = RefLayer, x
	case *syntax.Amendment:
		at.line = x
	case *syntax.AmendSegment:
		at.segKey, _ = segToken(x)
	case *syntax.IndexExpr:
		at.index = x
	case *syntax.RecordDecl:
		at.record = x
	case *syntax.FieldDecl:
		at.field, at.shared = x, true
	case *syntax.ConstDecl, *syntax.FnDecl, *syntax.LambdaExpr:
		at.shared = true
	}
	return at
}

// walk reports n when it names the target or tests its key, looks up its element by a ref key, or
// is an amend path's key segment naming it, then its children, under the construct holding them.
func (sc *nameScan) walk(n syntax.Node, at scanAt) {
	at = at.enter(n)
	if tok, ok := sc.claims(n, at); ok {
		if sp := sc.file.Span(tok); !sc.covered(sp) && sc.instanceHolds(tok, sp, at) {
			sc.out = append(sc.out, Ref{Kind: at.kind, Package: sc.pkg, Span: sp})
		}
	} else if tok, ok := sc.lineKey(n, at); ok {
		sc.out = append(sc.out, Ref{Kind: RefLayer, Package: sc.pkg, Span: sc.file.Span(tok)})
		sc.literalKey(n, at)
	}
	sc.keyTest(n, at)
	sc.elemKey(n, at)
	sc.instKey(n, at)
	for c := range syntax.Children(n) {
		sc.walk(c, at)
	}
}

// names is the node naming the target in n, if n does: a name the checker resolved to one of
// its declarations, or a symbolic key or key literal it recorded for the target's collection.
func (sc *nameScan) names(n syntax.Node) (syntax.Node, bool) {
	switch x := n.(type) {
	case *syntax.IdentExpr:
		return x, sc.objectNamed(x) || !sc.inScope(x) && sc.isKey(x, x.Name)
	case *syntax.Ident:
		return x, sc.objectNamed(x)
	case *syntax.SelectorExpr:
		return x.Name, sc.isKey(x, x.Name.Name) && sc.throughOwn(x) || sc.isEntryOf(x) || sc.refKeyOf(x)
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

// throughOwn reports the key lookup x reaching the target's own collection: a let's always; a record
// field's, at any depth, through `.`, `?.` or `!`, only when x reads statically from a let root the
// target's very entry, judged by identity as a spread copy holds its source's (API.md R7; DECISIONS 410, 473).
func (sc *nameScan) throughOwn(x *syntax.SelectorExpr) bool {
	if coll := sc.s.info.Keys[x]; coll == nil || coll.Kind != types.CollField {
		return true
	}
	p, ok := sc.s.StaticPath(x)
	if !ok {
		return false
	}
	res, err := Resolve(sc.s, p)
	entry, isRec := res.Target.(*value.Record)
	return err == nil && isRec && sc.tg.isEntry(entry.Ident)
}

// inColl reports key token e naming an entry of the target's collection: recorded as its key,
// or typed `ref C` for it, as a path's key into a list keyed by a ref is (DECISIONS 316).
func (sc *nameScan) inColl(e syntax.Expr) bool {
	if sc.tg.names(sc.s.info.Keys[e]) {
		return true
	}
	rt, isRef := baseOf(sc.s.info.Types[e]).(*types.RefType)
	return isRef && sc.tg.names(rt.Target)
}
