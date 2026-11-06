package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// baseRead is what the base evaluation makes of a read of the target's key that the layered one does
// not reach the target by: holds, a replaced version holds it; unknown, no base is kept (DECISIONS 489).
type baseRead struct {
	holds, unknown bool
}

// stuck reports a read no rewrite is known to keep: the rename is refused `computed` for it (DECISIONS 503).
func (b baseRead) stuck() bool {
	return b.holds || b.unknown
}

// letBase is keyBase of the collection let path coll reaches, once per collection: nothing for no
// let path, or one naming the target in the layered evaluation.
func (tg *target) letBase(coll *types.Collection) baseRead {
	if tg.ident == nil || coll == nil || coll.Kind != types.CollLet || tg.names(coll) {
		return baseRead{}
	}
	if known, ok := tg.bases[coll]; ok {
		return known
	}
	if tg.bases == nil {
		tg.bases = map[*types.Collection]baseRead{}
	}
	var read baseRead
	if res, err := tg.s.open(letPath(coll)); err == nil {
		read = tg.keyBase(res)
	}
	tg.bases[coll] = read
	return read
}

// keyBase is what the base evaluation reads at the target's key in res's collection: holds when a
// replaced version holds the target (a rewrite retargets the layered read), unknown when a layer
// line's own entry is read through a value with no base kept (DECISIONS 483, 489, 503).
func (tg *target) keyBase(res resolution) baseRead {
	k := tg.ident.Key
	versions := tg.s.a.History(res.values()...)
	for i := 1; i < len(versions); i++ {
		if old := keyedRecord(versions[i], k); old != nil && (tg.isEntry(old.Ident) || tg.ofOrigin(old)) {
			return baseRead{holds: true}
		}
	}
	cur := keyedRecord(res.Target, k)
	if len(versions) > 1 || cur == nil || cur.Ident.Coll != tg.ident.Coll || !tg.s.layerBuilt(cur) || tg.replaced(res, cur) {
		return baseRead{}
	}
	return baseRead{unknown: true}
}

// replaced reports cur, the entry at the target's key in the collection res reaches, a layer line's
// value put in the target's place: the rename rewrites that line's key with it (DECISIONS 491, 494).
func (tg *target) replaced(res resolution, cur *value.Record) bool {
	versions := tg.s.a.History(append(res.values(), cur)...)
	for i := 1; i < len(versions); i++ {
		if old, ok := versions[i].(*value.Record); ok && (tg.isEntry(old.Ident) || tg.ofOrigin(old)) {
			return true
		}
	}
	return false
}

// ofOrigin reports rec, as its origin produced it, the target's entry.
func (tg *target) ofOrigin(rec *value.Record) bool {
	base, ok := tg.s.a.Produced(rec).(*value.Record)
	return ok && tg.isEntry(base.Ident)
}

// layerBuilt reports rec an active layer's line's own: no copy of an entry the base evaluation holds.
func (s *Snapshot) layerBuilt(rec *value.Record) bool {
	return s.a.Produced(rec) == rec && slices.ContainsFunc(s.a.Layers(), func(x string) bool { return s.statedBy(rec, x) })
}

// readPath is the path of the target's key in the collection at p, which a rewritten token would
// make another entry's read: a shared path, it refuses the rename `computed` (API.md E12; DECISIONS 503).
func (tg *target) readPath(p Path) string {
	p.Segs = append(slices.Clip(p.Segs), entrySeg(tg.ident.Key))
	if res, err := Resolve(tg.s, p); err == nil {
		return res.Canonical
	}
	return p.String()
}

// baseKeys keeps, as a shared path, each key token of n naming the target's key through a let path
// whose base read keyBase cannot keep: one that names the target only in the base evaluation is
// listed by names, one whose base is unknown by none (DECISIONS 483, 489, 503).
func (sc *nameScan) baseKeys(n syntax.Node, at scanAt) {
	coll, ok := sc.keyColl(n)
	switch {
	case !ok:
	case sc.tg.letBase(coll).stuck():
		sc.shared = append(sc.shared, sc.tg.readPath(letPath(coll)))
	case coll == sc.tg.ident.Coll:
		if read, p := sc.readBase(sc.keyRecv(n, at)); read.stuck() { // a field's, read through a static path
			sc.shared = append(sc.shared, sc.tg.readPath(p))
		}
	}
}

// keyRecv is the receiver key token n reads: a selector's, or the nearest index's when n is its key.
func (sc *nameScan) keyRecv(n syntax.Node, at scanAt) syntax.Expr {
	if x, isSel := n.(*syntax.SelectorExpr); isSel {
		return x.X
	}
	if at.index != nil && syntax.Unparen(at.index.Index) == n {
		return at.index.X
	}
	return nil
}

// keyColl is the let-path collection key token n reads the target's key in: an identifier no
// scope resolves, a selector's name, a map's word key, a string or integer literal.
func (sc *nameScan) keyColl(n syntax.Node) (*types.Collection, bool) {
	id, info := sc.tg.ident, sc.s.info
	if id == nil {
		return nil, false
	}
	switch x := n.(type) {
	case *syntax.IdentExpr:
		return sc.exprColl(x), !id.Key.IsInt && x.Name == id.Key.S && sc.s.scopeName(x) == nil
	case *syntax.SelectorExpr:
		return info.Keys[x], x.Name != nil && !id.Key.IsInt && x.Name.Name == id.Key.S
	case *syntax.Ident:
		return info.NameKeys[x], !id.Key.IsInt && x.Name == id.Key.S
	case *syntax.StringLit, *syntax.RawStringLit:
		text, ok := constString(x.(syntax.Expr))
		return sc.exprColl(x.(syntax.Expr)), ok && !id.Key.IsInt && text == id.Key.S
	case *syntax.IntLit:
		return sc.exprColl(x), id.Key.IsInt && x.Value.IsInt64() && x.Value.Int64() == id.Key.I
	}
	return nil, false
}

// exprColl is the collection key expression e is recorded a key of, else the one its `ref` type names.
func (sc *nameScan) exprColl(e syntax.Expr) *types.Collection {
	if coll := sc.s.info.Keys[e]; coll != nil {
		return coll
	}
	if rt, isRef := baseOf(sc.s.info.Types[e]).(*types.RefType); isRef {
		return rt.Target
	}
	return nil
}

// readBase is keyBase of the collection receiver recv reads statically, and that collection's path;
// nothing for a dynamic receiver or none.
func (sc *nameScan) readBase(recv syntax.Expr) (baseRead, Path) {
	if recv == nil || sc.tg.ident == nil {
		return baseRead{}, Path{}
	}
	p, ok := sc.s.StaticPath(recv)
	if !ok {
		return baseRead{}, Path{}
	}
	p = sc.s.unalias(p)
	res, err := sc.s.open(p)
	if err != nil || sc.tg.isEntry(keyedIdent(res.Target, sc.tg.ident.Key)) {
		return baseRead{}, p
	}
	return sc.tg.keyBase(res), p
}
