package edit

import (
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
	p, ok := sc.s.StaticPath(ix)
	if !ok {
		return
	}
	res, err := Resolve(sc.s, p)
	rec, isRec := res.Target.(*value.Record)
	if err != nil || !isRec || !sc.tg.isEntry(rec.Ident) {
		return
	}
	if sp := sc.file.Span(k); !sc.covered(sp) {
		sc.out = append(sc.out, Ref{Kind: at.kind, Package: sc.pkg, Span: sp})
	}
}

// refKeyOf reports an expression's `.k` on a list keyed by a `ref C` naming the target, C's entry
// keyed k, as `[k]` does; C a let's collection, not one each instance holds its own of (DECISIONS 488).
func (sc *nameScan) refKeyOf(x *syntax.SelectorExpr) bool {
	id := sc.tg.ident
	if id == nil || id.Key.IsInt || id.Key.S != x.Name.Name || x.X == nil || sc.s.info.Keys[x] == nil {
		return false
	}
	rt, ok := keyRef(sc.s.info.Types[x.X])
	return ok && rt.Target != nil && rt.Target.Kind != types.CollField && sc.tg.names(rt.Target)
}

// refKeyed reports t, optionals aside, a list keyed by a field typed `ref C`.
func refKeyed(t types.Type) bool {
	_, ok := keyRef(t)
	return ok
}

// keyRef is the `ref C` type of the key field of t, optionals aside, a keyed list; false for another type.
func keyRef(t types.Type) (*types.RefType, bool) {
	if t == nil {
		return nil, false
	}
	lt, isList := present(t).Base().(*types.ListType)
	if !isList || lt.KeyedBy == nil {
		return nil, false
	}
	rt, isRef := baseOf(lt.KeyedBy.Type).(*types.RefType)
	return rt, isRef
}
