package std

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// keyedMethods are STDLIB.md §5 over Seq(T).
func keyedMethods() map[string]builtin {
	m := seqMethods()
	m[bGet], m[bFind], m[bHasKey], m[bAt] = keyedGet, keyedGet, keyedHasKey, keyedAt
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
	e, found, ok := keyedEntry(h, c)
	if !ok {
		return nil, false
	}
	return c.orNone(e, found), h.Charge(1)
}

// keyedHasKey is hasKey(k): an entry of key k exists, as get(k) != none, at get's cost (STDLIB.md §5, DECISIONS 317).
func keyedHasKey(h Host, c *Call) (value.Value, bool) {
	_, found, ok := keyedEntry(h, c)
	if !ok {
		return nil, false
	}
	return c.boolv(found), h.Charge(1)
}

// keyedEntry is the entry of argument 0's key, found false when there is none; false ok: the root aborted.
func keyedEntry(h Host, c *Call) (e *value.Record, found, ok bool) {
	k, has, ok := argKey(h, c.Recv, c.arg(0))
	if !has || !ok {
		return nil, false, ok
	}
	e, found = h.Entry(c.Recv, k)
	return e, found, true
}

// argKey is the key x names in recv: a value of the key type or a ref keeps its own; an entry
// converted to a `ref C` key has one only as an entry of C, a let path's instance included
// (DECISIONS 315); false ok: the root aborted.
func argKey(h Host, recv, x value.Value) (k value.Key, has, ok bool) {
	rec, isRec := x.(*value.Record)
	if !isRec {
		k, has = KeyOf(x)
		return k, has, true
	}
	_, kt := keyedTypes(recv)
	if kt == nil || rec.Ident == nil {
		return value.Key{}, false, true
	}
	r, isRef := kt.Base().(*types.RefType)
	if !isRef {
		return value.Key{}, false, true
	}
	yes, ok := h.Belongs(rec, r.Target)
	return rec.Ident.Key, yes, ok
}

// keyedTypes are the element and key types of a table or keyed list, nil for another value.
func keyedTypes(v value.Value) (elem, key types.Type) {
	switch x := v.(type) {
	case *value.Table:
		if t, ok := x.T.Base().(*types.TableType); ok {
			return t.Elem, types.StringType
		}
	case *value.List:
		if t, ok := x.T.Base().(*types.ListType); ok && t.KeyedBy != nil {
			return t.Elem, t.KeyedBy.Type
		}
	}
	return nil, nil
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
			r.Key, r.Owner = e.Ident.Key, fieldOwner(rt, e.Ident.Owner)
		}
		out[i] = r
	})
	return c.list(out), ok
}

// fieldOwner is owner for a ref into a field's collection; a let-path ref holds no instance (DECISIONS 315).
func fieldOwner(rt types.Type, owner *value.Record) *value.Record {
	if r, ok := rt.Base().(*types.RefType); ok && r.Target != nil && r.Target.Kind == types.CollField {
		return owner
	}
	return nil
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
