package edit

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// elemKey reports n's key token when n is an expression's `[k]` on a list keyed by a `ref C`
// whose receiver reaches the target's very element statically, judged by identity: a ref of that
// element, beside the ref of C's entry the token stays (API.md R7; DECISIONS 316, 473, 477, 488).
func (sc *nameScan) elemKey(n syntax.Node, at scanAt) {
	ix, isIndex := n.(*syntax.IndexExpr)
	if !isIndex || sc.tg.ident == nil || !refKeyed(sc.s.info.Types[ix.X]) {
		return
	}
	k := syntax.Unparen(ix.Index)
	if _, named := sc.names(k); named {
		return
	}
	rec, ok := sc.readElement(ix)
	if !ok || !sc.tg.isEntry(rec.Ident) {
		return
	}
	sc.listKey(k, at)
}

// instKey reports n's key token when n is an expression's `[k]` or `.k` on a list keyed by a ref
// into each instance's own collection: a ref of the entry the key field of the element its receiver
// reaches statically names, by identity, that instance's own alone (API.md R7; DECISIONS 483, 488).
func (sc *nameScan) instKey(n syntax.Node, at scanAt) {
	recv, tok := sc.keyRead(n)
	if recv == nil || sc.tg.ident == nil || !perInstanceKeyed(sc.s.info.Types[recv]) {
		return
	}
	rec, ok := sc.readElement(n.(syntax.Expr))
	if ok && sc.tg.isValue(keyField(rec, keyedBy(sc.s.info.Types[recv]))) {
		sc.listKey(tok, at)
	}
}

// readElement is the keyed-list element expression e reads statically, false for a dynamic one.
func (sc *nameScan) readElement(e syntax.Expr) (*value.Record, bool) {
	p, ok := sc.s.StaticPath(e)
	if !ok {
		return nil, false
	}
	res, err := Resolve(sc.s, p)
	rec, isRec := res.Target.(*value.Record)
	return rec, err == nil && isRec
}

// listKey lists key token tok, unless a stated value's span covers it.
func (sc *nameScan) listKey(tok syntax.Node, at scanAt) {
	if sp := sc.file.Span(tok); !sc.covered(sp) {
		sc.out = append(sc.out, Ref{Kind: at.kind, Package: sc.pkg, Span: sp})
	}
}

// keyRead is the receiver and key token of n when n reads a keyed list by key in an expression:
// `x[k]`, or `x.k`, which the checker records as a key or selects as an entry, its keys dynamic in a
// record field's default (DECISIONS 488, 491); nil for any other node.
func (sc *nameScan) keyRead(n syntax.Node) (syntax.Expr, syntax.Node) {
	switch x := n.(type) {
	case *syntax.IndexExpr:
		return x.X, syntax.Unparen(x.Index)
	case *syntax.SelectorExpr:
		sel := sc.s.info.Selections[x]
		if x.X != nil && (sc.s.info.Keys[x] != nil || sel != nil && sel.Kind == check.SelEntry) {
			return x.X, x.Name
		}
	}
	return nil, nil
}

// refKeyOf reports an expression's `.k` on a list keyed by a `ref C` naming the target, C's entry
// keyed k, as `[k]` does; C a let's collection, not one each instance holds its own of (DECISIONS 488).
func (sc *nameScan) refKeyOf(x *syntax.SelectorExpr) bool {
	id := sc.tg.ident
	recv, _ := sc.keyRead(x)
	if id == nil || id.Key.IsInt || id.Key.S != x.Name.Name || recv == nil {
		return false
	}
	rt, ok := keyRef(sc.s.info.Types[recv])
	return ok && rt.Target != nil && rt.Target.Kind != types.CollField && sc.tg.names(rt.Target)
}

// perIndexKey reports n the key of at's index expression on a list keyed by a ref into each
// instance's own collection, which instKey alone judges.
func (sc *nameScan) perIndexKey(n syntax.Node, at scanAt) bool {
	return at.index != nil && n == syntax.Unparen(at.index.Index) && perInstanceKeyed(sc.s.info.Types[at.index.X])
}

// perInstanceKeyed reports t a list keyed by a ref into a record field's collection.
func perInstanceKeyed(t types.Type) bool {
	rt, ok := keyRef(t)
	return ok && rt.Target != nil && rt.Target.Kind == types.CollField
}

// refKeyed reports t, optionals aside, a list keyed by a field typed `ref C`.
func refKeyed(t types.Type) bool {
	_, ok := keyRef(t)
	return ok
}

// keyRef is the `ref C` type of the key field of t, optionals aside, a keyed list; false for another type.
func keyRef(t types.Type) (*types.RefType, bool) {
	kf := keyedBy(t)
	if kf == nil {
		return nil, false
	}
	rt, isRef := baseOf(kf.Type).(*types.RefType)
	return rt, isRef
}

// keyedBy is the key field of t, optionals aside, a keyed list; nil for another type.
func keyedBy(t types.Type) *types.Field {
	if t == nil {
		return nil
	}
	lt, isList := present(t).Base().(*types.ListType)
	if !isList {
		return nil
	}
	return lt.KeyedBy
}
