package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// claims is names on n, but at an amend segment's key of the collection it indexes, which lineKey
// alone judges by identity, only a declaration's name; a key typed `ref C` or written as a
// selector names C's entry as anywhere (DECISIONS 316, 484).
func (sc *nameScan) claims(n syntax.Node, at scanAt) (syntax.Node, bool) {
	if n == at.segKey && !sc.refKey(n) {
		return n, sc.objectNamed(n)
	}
	return sc.names(n)
}

// refKey reports key token n a selector or typed `ref C`.
func (sc *nameScan) refKey(n syntax.Node) bool {
	e, isExpr := n.(syntax.Expr)
	if !isExpr {
		return false
	}
	if _, isSel := e.(*syntax.SelectorExpr); isSel {
		return true
	}
	_, isRef := baseOf(sc.s.info.Types[e]).(*types.RefType)
	return isRef
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
	if tok == nil || !segKeyIs(seg, sc.tg.res.Steps[len(sc.tg.res.Steps)-1].Seg) {
		return nil, false
	}
	if _, claimed := sc.claims(holder, at); claimed {
		return nil, false
	}
	i := slices.Index(at.line.Path, seg)
	return tok, i >= 0 && sc.lineEntry(at, at.line.Path[:i+1])
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
	if obj == nil {
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
	path := []value.Value{res.val}
	for _, st := range res.Steps {
		path = append(path, st.Value)
	}
	versions := sc.s.a.History(path...)
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
