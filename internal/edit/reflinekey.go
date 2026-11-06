package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// claims is names on n, but at an amend segment's key of the collection it indexes, which lineKey
// alone judges by identity, only a declaration's name; a selector names C's entry as anywhere, and
// a key typed `ref C` C's entry by its type, never the element it keys (DECISIONS 316, 484, 487).
func (sc *nameScan) claims(n syntax.Node, at scanAt) (syntax.Node, bool) {
	e, isExpr := n.(syntax.Expr)
	switch _, isSel := e.(*syntax.SelectorExpr); {
	case sc.perIndexKey(n, at):
		return n, sc.objectNamed(n) // instKey judges it by its instance (DECISIONS 483)
	case n != at.segKey || isExpr && isSel:
		return sc.names(n)
	}
	if rt, isRef := baseOf(sc.s.info.Types[e]).(*types.RefType); isExpr && isRef {
		return n, sc.objectNamed(n) || sc.refNamed(e, rt)
	}
	return n, sc.objectNamed(n)
}

// refNamed reports key token e, typed rt, stating the target's key in rt's collection: C's entry.
func (sc *nameScan) refNamed(e syntax.Expr, rt *types.RefType) bool {
	if !sc.tg.names(rt.Target) {
		return false
	}
	if _, isConst := sc.constKey(e); isConst {
		return false // a constant's name is the constant's (DECISIONS 489)
	}
	if x, isIdent := e.(*syntax.IdentExpr); isIdent {
		return !sc.inScope(x) && !sc.tg.ident.Key.IsInt && sc.tg.ident.Key.S == x.Name
	}
	match, _ := sc.targetKey(e)
	return match
}

// objectNamed reports n a name the checker resolved to one of the target's declarations.
func (sc *nameScan) objectNamed(n syntax.Node) bool {
	switch x := n.(type) {
	case *syntax.IdentExpr:
		return sc.isObject(sc.s.info.Uses[x])
	case *syntax.Ident:
		return sc.isObject(sc.s.info.NameUses[x])
	}
	return false
}

// lineKey is the key token of n, an amend path's segment, when the line's path up to it reaches the
// target's very entry by identity (API.md R7; DECISIONS 316, 470, 483, 484); a key claims reports
// is left to it, so each token is listed once.
func (sc *nameScan) lineKey(n syntax.Node, at scanAt) (syntax.Node, bool) {
	seg, isSeg := n.(*syntax.AmendSegment)
	if !isSeg || at.line == nil || at.block == nil || sc.tg.ident == nil {
		return nil, false
	}
	holder, tok := segToken(seg)
	if tok == nil || sc.s.scopedSegs([]*syntax.AmendSegment{seg}) || !segKeyIs(seg, sc.tg.res.Steps[len(sc.tg.res.Steps)-1].Seg) {
		return nil, false
	}
	if _, claimed := sc.claims(holder, at); claimed {
		return nil, false
	}
	i := slices.Index(at.line.Path, seg)
	return tok, i >= 0 && sc.lineEntry(at, at.line.Path[:i+1])
}

// literalKey lists the key field of the literal a line replaces the entry its last segment, n, reaches
// with, when it states the target's key: one key, one token (API.md E11; DECISIONS 484, 491).
func (sc *nameScan) literalKey(n syntax.Node, at scanAt) {
	seg, isSeg := n.(*syntax.AmendSegment)
	if !isSeg || at.line.Value == nil || at.line.Path[len(at.line.Path)-1] != seg {
		return
	}
	kf := keyedBy(sc.tg.res.Steps[len(sc.tg.res.Steps)-1].Container)
	if kf == nil {
		return
	}
	fi := fieldItem(syntax.Unparen(at.line.Value), kf.Name)
	if fi == nil || fi.Value == nil {
		return
	}
	v := syntax.Unparen(fi.Value)
	if tok := sc.s.keyFieldToken(v, sc.tg.ident.Key); tok != nil {
		if _, claimed := sc.names(v); !claimed {
			sc.listKey(tok, at)
		}
	}
}

// segToken is the node of seg the walk meets for its key, and the token writing it: a `.k`'s name,
// a `[k]`'s key, a `[Q.m]`'s member name (E11); none for `[#n]`, which names no key.
func segToken(seg *syntax.AmendSegment) (holder, tok syntax.Node) {
	switch {
	case seg.Name != nil:
		return seg.Name, seg.Name
	case seg.Key != nil:
		k := syntax.Unparen(seg.Key)
		if sel, ok := k.(*syntax.SelectorExpr); ok {
			return sel, sel.Name
		}
		return k, k
	}
	return nil, nil
}

// segKeyIs reports seg written as the key of canonical segment s.
func segKeyIs(seg *syntax.AmendSegment, s Seg) bool {
	if seg.Name != nil {
		return nameIs(s, seg.Name.Name)
	}
	return keyExprIs(syntax.Unparen(seg.Key), s)
}

// lineEntry reports segs, the first segments of a line of block at.block, reaching the target's
// entry by identity, a copy holding its source's (410, 473) and carrying its key (486, 489, wasEntry),
// or an element keyed by a ref to it, whose `.k` names it as `[k]` does (DECISIONS 316).
func (sc *nameScan) lineEntry(at scanAt, segs []*syntax.AmendSegment) bool {
	obj := sc.s.info.NameUses[at.block.Target]
	if obj == nil || sc.s.scopedSegs(segs) {
		return false
	}
	p, err := Parse(sc.s.linePathOf(sc.file, Path{Package: obj.Pkg(), Root: obj.Name()}, segs))
	if err != nil {
		return false
	}
	res, err := sc.s.open(p)
	rec, isRec := res.Target.(*value.Record)
	if err != nil || !isRec || len(res.Steps) == 0 {
		return false
	}
	lt, keyed := baseOf(res.Steps[len(res.Steps)-1].Container).(*types.ListType)
	return sc.tg.isEntry(rec.Ident) || sc.wasEntry(res) || keyed && lt.KeyedBy != nil && sc.tg.isValue(keyField(rec, lt.KeyedBy))
}

// wasEntry reports the value res's path held before any amendment, as its origin produced it, being
// the target's entry by identity: the entry a copy held before a layer's line made it the copy's
// own, keeping its key; a value a line put in its place carries no key there (DECISIONS 486, 489).
func (sc *nameScan) wasEntry(res resolution) bool {
	versions := sc.s.a.History(res.values()...)
	if len(versions) == 0 {
		return false
	}
	rec, isRec := sc.s.a.Produced(versions[len(versions)-1]).(*value.Record)
	return isRec && sc.tg.isEntry(rec.Ident)
}

// rewrite replaces a reference's token: an amend path's key segment as layerKeys does (segKey),
// whichever root its line amends; any other token in its own form (retoken). Each token is
// rewritten once (E11).
func (r *renaming) rewrite(ref Ref) error {
	if f := r.x.a.snap.files[ref.Span.File]; ref.Kind == RefLayer && f != nil && f.FileKind == syntax.FileLayer {
		if it, i := lineSegAt(f, ref.Span); it != nil {
			r.pkgs[r.x.a.snap.display(ref.Span.File)] = ref.Package
			return r.segKey(f, it, i)
		}
	}
	return r.retoken(ref.Span, ref.Package)
}

// lineSegAt is the amend line of layer file f whose segment i writes its key at sp; nil for none.
func lineSegAt(f *syntax.File, sp source.Span) (*syntax.Amendment, int) {
	for _, b := range f.Amends {
		for _, it := range b.Items {
			if i := slices.IndexFunc(it.Path, func(seg *syntax.AmendSegment) bool { return tokenAt(f, seg, sp) }); i >= 0 {
				return it, i
			}
		}
	}
	return nil, 0
}

// tokenAt reports seg's key token at sp in f.
func tokenAt(f *syntax.File, seg *syntax.AmendSegment, sp source.Span) bool {
	_, tok := segToken(seg)
	return tok != nil && f.Span(tok) == sp
}
