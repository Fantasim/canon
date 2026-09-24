package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// compound is the operator of each compound assignment (TYPES.md §7.1).
var compound = map[syntax.TokenKind]syntax.TokenKind{
	syntax.TokAddAssign: syntax.TokPlus, syntax.TokSubAssign: syntax.TokMinus,
	syntax.TokMulAssign: syntax.TokStar, syntax.TokDivAssign: syntax.TokSlash,
}

// execAssign rebinds a var, an element at any depth copied on write (EVALUATION.md §4.1).
func execAssign(r *run, s syntax.Stmt) flow {
	x := s.(*syntax.AssignStmt)
	root, segs := assignPath(x.Target)
	obj := r.ev.info.Uses[root]
	cur, ok := r.fr.vars[obj]
	if root == nil || obj == nil || !ok {
		r.bug(s)
		return flowAbort
	}
	keys := make([]value.Value, len(segs))
	for i, seg := range segs {
		if keys[i] = r.eval(seg.Index); keys[i] == nil {
			return flowAbort
		}
	}
	t := obj.Type()
	for range segs {
		t = elementType(t)
	}
	v := r.assigned(x, cur, keys, t)
	if v == nil {
		return flowAbort
	}
	next := r.setAt(cur, keys, v, x)
	if next == nil {
		return flowAbort
	}
	r.fr.vars[obj] = next
	return flowNext
}

// assigned is e, or `old op e` with old read before e is evaluated (EVALUATION.md §2.2).
func (r *run) assigned(x *syntax.AssignStmt, cur value.Value, keys []value.Value, t types.Type) value.Value {
	op, isCompound := compound[x.Op]
	var old value.Value
	if isCompound {
		if old = r.getAt(cur, keys, x); old == nil {
			return nil
		}
	}
	rhs := r.eval(x.Value)
	if rhs == nil {
		return nil
	}
	s := r.varSite(r.ev.info.Uses[rootOf(x.Target)])
	if !isCompound {
		return r.store(rhs, t, s, nil)
	}
	return r.store(r.binop(op, old, rhs, x, t), t, s, nil)
}

// rootOf is the root name of an assignment target.
func rootOf(e syntax.Expr) *syntax.IdentExpr {
	root, _ := assignPath(e)
	return root
}

// varSite is the declaration of a var with a written type, `n: Int`.
func (r *run) varSite(obj check.Object) site {
	if d, ok := obj.Decl().(*syntax.VarStmt); ok {
		return declSite(obj.File(), d.Name, d.Type)
	}
	return site{}
}

// assignPath is the root name of an assignment target and its index segments, root first.
func assignPath(e syntax.Expr) (*syntax.IdentExpr, []*syntax.IndexExpr) {
	var segs []*syntax.IndexExpr
	for {
		switch x := e.(type) {
		case *syntax.IdentExpr:
			for i, j := 0, len(segs)-1; i < j; i, j = i+1, j-1 {
				segs[i], segs[j] = segs[j], segs[i]
			}
			return x, segs
		case *syntax.IndexExpr:
			segs = append(segs, x)
			e = x.X
		case *syntax.ParenExpr:
			e = x.X
		default:
			return nil, nil
		}
	}
}

// elementType is the type of an element of a list, a value of a map, an entry of a table.
func elementType(t types.Type) types.Type {
	switch x := t.Base().(type) {
	case *types.ListType:
		return x.Elem
	case *types.MapType:
		return x.Value
	case *types.TableType:
		return x.Elem
	}
	return t
}

// getAt reads the element at keys.
func (r *run) getAt(v value.Value, keys []value.Value, at syntax.Node) value.Value {
	for _, k := range keys {
		i, found := r.slot(v, k, at, false)
		if !found {
			return nil
		}
		v = slotValue(v, i)
	}
	return v
}

// setAt is v with the element at keys replaced (EVALUATION.md §4.1).
func (r *run) setAt(v value.Value, keys []value.Value, nv value.Value, at syntax.Node) value.Value {
	if len(keys) == 0 {
		return nv
	}
	i, found := r.slot(v, keys[0], at, len(keys) == 1)
	if !found {
		return nil
	}
	inner := nv
	if len(keys) > 1 {
		if inner = r.setAt(slotValue(v, i), keys[1:], nv, at); inner == nil {
			return nil
		}
	}
	return r.ev.replaced(v, i, keys[0], inner)
}

// slot finds key k in v: the position of a list element, a map entry (-1 for a new key when
// adding is allowed) or a keyed entry; E4002 when it is missing.
func (r *run) slot(v, k value.Value, at syntax.Node, adding bool) (int, bool) {
	switch x := v.(type) {
	case *value.Map:
		r.site = r.span(at)
		if i, ok := std.MapIndex(r.host(), x, k); !ok || i >= 0 {
			return i, ok
		}
		if adding {
			return -1, true
		}
	case *value.List:
		if n, ok := k.(*value.Int); ok && !std.Keyed(x) {
			return r.listSlot(x, n.V, at)
		}
		return r.keyedSlot(v, k, at)
	case *value.Table:
		return r.keyedSlot(v, k, at)
	}
	r.fail(diag.E4002.AtKey(r.span(at), k, r.typeOfNode(v)))
	return 0, false
}

// listSlot is position n of a plain list, negative from the end; E4002 when it is missing.
func (r *run) listSlot(l *value.List, n int64, at syntax.Node) (int, bool) {
	i := n
	if i < 0 {
		i += int64(len(l.Elems))
	}
	if i >= 0 && i < int64(len(l.Elems)) {
		return int(i), true
	}
	r.fail(diag.E4002.AtIndex(r.span(at), n, int64(len(l.Elems))))
	return 0, false
}

func (r *run) keyedSlot(v, k value.Value, at syntax.Node) (int, bool) {
	key, _ := std.KeyOf(k)
	for i, e := range std.Elems(v) {
		if rec, ok := e.(*value.Record); ok && rec.Ident != nil && rec.Ident.Key == key {
			return i, true
		}
	}
	r.fail(diag.E4002.AtKey(r.span(at), k, r.typeOfNode(v)))
	return 0, false
}

// typeOfNode names a collection by its type, for E4002 in an assignment.
func (r *run) typeOfNode(v value.Value) string {
	return v.Type().String()
}

func slotValue(v value.Value, i int) value.Value {
	if m, ok := v.(*value.Map); ok {
		return m.Vals[i]
	}
	return std.Elems(v)[i]
}

// replaced copies v with slot i set to nv; slot -1 of a map appends key k.
func (e *Evaluator) replaced(v value.Value, i int, k, nv value.Value) value.Value {
	switch x := v.(type) {
	case *value.Map:
		out := &value.Map{T: x.T, Keys: append([]value.Value(nil), x.Keys...), Vals: append([]value.Value(nil), x.Vals...), P: x.P}
		if i < 0 {
			out.Keys, out.Vals = append(out.Keys, k), append(out.Vals, nv)
		} else {
			out.Vals[i] = nv
		}
		return out
	case *value.List:
		elems := append([]value.Value(nil), x.Elems...)
		elems[i] = e.withIdent(nv, elems[i])
		return &value.List{T: x.T, Elems: elems, P: x.P}
	case *value.Table:
		entries := append([]*value.Record(nil), x.Entries...)
		if rec, ok := e.withIdent(nv, x.Entries[i]).(*value.Record); ok {
			entries[i] = rec
		}
		return &value.Table{T: x.T, Entries: entries, P: x.P}
	}
	return v
}

// withIdent is nv in the place of old: a record replacing an entry takes its identity.
func (e *Evaluator) withIdent(nv, old value.Value) value.Value {
	rec, ok := nv.(*value.Record)
	prev, wasEntry := old.(*value.Record)
	if !ok || !wasEntry || prev.Ident == nil {
		return nv
	}
	return e.withIdentity(rec, prev.Ident)
}
