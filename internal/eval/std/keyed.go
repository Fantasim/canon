package std

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// keyedMethods are STDLIB.md §5 over Seq(T).
func keyedMethods() map[string]builtin {
	m := seqMethods()
	m[bGet], m[bFind], m[bAt] = keyedGet, keyedGet, keyedAt
	m[bKeys], m[bValues], m[bActive] = keyedKeys, keyedValues, keyedActive
	return m
}

// listMethods are the methods of a plain list: Seq(T) and get(i) by position (STDLIB.md §4.1).
func listMethods() map[string]builtin {
	m := seqMethods()
	m[bGet] = listGet
	return m
}

// listGet is element i, negative from the end; none out of range.
func listGet(h Host, c *Call) (value.Value, bool) {
	xs := Elems(c.Recv)
	i := intOf(c.arg(0))
	if i < 0 {
		i += int64(len(xs))
	}
	if i < 0 || i >= int64(len(xs)) {
		return c.none(), h.Charge(1)
	}
	return xs[i], h.Charge(1)
}

// keyedGet is get(k) and find(k): the entry of key k (a ref of the collection too), or none.
func keyedGet(h Host, c *Call) (value.Value, bool) {
	k, _ := KeyOf(c.arg(0))
	e, ok := h.Entry(c.Recv, k)
	return c.orNone(e, ok), h.Charge(1)
}

// keyedAt is the entry at position i, negative from the end; E4002 out of range.
func keyedAt(h Host, c *Call) (value.Value, bool) {
	xs := Elems(c.Recv)
	i := intOf(c.arg(0))
	if !h.Charge(1) {
		return nil, false
	}
	j := i
	if j < 0 {
		j += int64(len(xs))
	}
	if j < 0 || j >= int64(len(xs)) {
		h.Fail(diag.E4002.AtIndex(h.Site(), i, int64(len(xs))))
		return nil, false
	}
	return xs[j], true
}

// keyedKeys are refs to the entries, in order.
func keyedKeys(h Host, c *Call) (value.Value, bool) {
	xs := Elems(c.Recv)
	rt := refTypeOf(c.Result, xs)
	out := make([]value.Value, len(xs))
	ok := each(h, xs, func(i int, x value.Value) {
		r := &value.Ref{T: rt, P: c.Prov}
		if e, isRec := x.(*value.Record); isRec && e.Ident != nil {
			r.Key, r.Owner = e.Ident.Key, e.Ident.Owner
		}
		out[i] = r
	})
	return c.list(out), ok
}

// refTypeOf is the element type of keys(): the checker's ref when it bound one, else a ref
// into the entries' collection.
func refTypeOf(t types.Type, xs []value.Value) types.Type {
	rt := elemType(t)
	if _, ok := rt.Base().(*types.RefType); ok || len(xs) == 0 {
		return rt
	}
	if e, ok := xs[0].(*value.Record); ok && e.Ident != nil {
		return &types.RefType{Target: e.Ident.Coll}
	}
	return rt
}

func keyedValues(h Host, c *Call) (value.Value, bool) {
	xs := Elems(c.Recv)
	ok := each(h, xs, func(int, value.Value) {})
	return c.list(xs), ok
}

// keyedActive are the entries that are not retired.
func keyedActive(h Host, c *Call) (value.Value, bool) {
	var out []value.Value
	ok := each(h, Elems(c.Recv), func(_ int, x value.Value) {
		if e, isRec := x.(*value.Record); isRec && (e.Ident == nil || !e.Ident.Retired) {
			out = append(out, x)
		}
	})
	return c.list(out), ok
}

// rangeMethods are STDLIB.md §10: len (E4002 on an open range), isEmpty, contains.
func rangeMethods() map[string]builtin {
	return map[string]builtin{bLen: rangeLen, bIsEmpty: rangeIsEmpty, bContains: rangeContains}
}

// rangeLen is max(end − start, 0), E4002 on an open range (DECISIONS 197).
func rangeLen(h Host, c *Call) (value.Value, bool) {
	r := c.Recv.(*value.Range)
	if !r.HasEnd {
		h.Fail(diag.E4002.AtOpen(h.Site()))
		return nil, false
	}
	if !h.Charge(1) {
		return nil, false
	}
	if r.End <= r.Start {
		return c.intv(0), true
	}
	return intArith(h, OpSub, r.End, r.Start, c.Prov)
}

func rangeIsEmpty(h Host, c *Call) (value.Value, bool) {
	r := c.Recv.(*value.Range)
	return c.boolv(r.HasEnd && r.End <= r.Start), h.Charge(1)
}

func rangeContains(h Host, c *Call) (value.Value, bool) {
	return c.boolv(InRange(c.Recv.(*value.Range), intOf(c.arg(0)))), h.Charge(1)
}

// InRange reports start <= x < end, x >= start on an open range.
func InRange(r *value.Range, x int64) bool {
	return x >= r.Start && (!r.HasEnd || x < r.End)
}
