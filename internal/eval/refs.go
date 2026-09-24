package eval

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// deref is the entry a ref names: E3501 or E3505 when none, both hard (EVALUATION.md §7.1).
func (r *run) deref(v value.Value, at syntax.Node) value.Value {
	ref, ok := v.(*value.Ref)
	if !ok {
		return v
	}
	rt, ok := ref.T.Base().(*types.RefType)
	if !ok {
		r.bug(at)
		return nil
	}
	coll := r.collValue(ref, rt.Target, at)
	if coll == nil {
		return nil
	}
	e, found := r.ev.entry(coll, ref.Key)
	if !found {
		r.fail(diag.E3501.At(r.span(at), ref, r.collName(rt.Target)))
		return nil
	}
	return r.read(e)
}

// collValue is the collection a ref's target names (EVALUATION.md §3.4).
func (r *run) collValue(ref *value.Ref, c *types.Collection, at syntax.Node) value.Value {
	var base value.Value
	if c.Kind == types.CollField {
		if ref.Owner == nil {
			r.fail(diag.E3505.At(r.span(at), elemName(c), ownerName(c)))
			return nil
		}
		base = ref.Owner
	} else {
		st := r.ev.roots[Root{Pkg: c.Pkg, Name: c.Name}]
		if st == nil {
			r.bug(at)
			return nil
		}
		v, ok := r.ev.force(r.ctx, st, r, at)
		if !ok {
			r.stop()
			return nil
		}
		base = v
	}
	for _, name := range c.FieldPath {
		rec, ok := base.(*value.Record)
		i := -1
		if ok {
			i = fieldIndex(rec.T, name)
		}
		if i < 0 || rec.Fields[i] == nil {
			r.bug(at)
			return nil
		}
		base = rec.Fields[i]
	}
	return base
}

// collName names a collection in E3501: qualified when it is another package's.
func (r *run) collName(c *types.Collection) string {
	name := r.qualified(c.Pkg, c.Name)
	if c.Kind == types.CollField && c.Owner != nil {
		name = r.qualified(c.Owner.Pkg, c.Owner.Name)
	}
	return strings.Join(append([]string{name}, c.FieldPath...), dot)
}

func elemName(c *types.Collection) string {
	if rt, ok := c.Elem.Base().(*types.RecordType); ok {
		return rt.Name
	}
	return c.Elem.String()
}

func ownerName(c *types.Collection) string {
	if c.Owner != nil {
		return c.Owner.Name
	}
	return c.Name
}

// entry is the entry of key k of a table or keyed list, through an index built once per
// collection value when it is large enough to need one.
func (e *Evaluator) entry(coll value.Value, k value.Key) (*value.Record, bool) {
	elems := std.Elems(coll)
	if len(elems) < indexFrom {
		for _, x := range elems {
			if rec, ok := x.(*value.Record); ok && rec.Ident != nil && rec.Ident.Key == k {
				return rec, true
			}
		}
		return nil, false
	}
	idx := e.keyed[coll]
	if idx == nil {
		idx = keyIndex(elems)
		e.keyed[coll] = idx
	}
	rec, ok := idx[k]
	return rec, ok
}

// keyIndex maps each key to its first entry.
func keyIndex(elems []value.Value) map[value.Key]*value.Record {
	idx := map[value.Key]*value.Record{}
	for _, x := range elems {
		rec, ok := x.(*value.Record)
		if !ok || rec.Ident == nil {
			continue
		}
		if _, dup := idx[rec.Ident.Key]; !dup {
			idx[rec.Ident.Key] = rec
		}
	}
	return idx
}

// Entry is the evaluator's lookup for the standard library (std.Host).
func (h *stdHost) Entry(coll value.Value, k value.Key) (*value.Record, bool) {
	return h.r.ev.entry(coll, k)
}

// bindRefs binds the level-1 refs of a completed instance (EVALUATION.md §3.4).
func (e *Evaluator) bindRefs(rec *value.Record) {
	owned := e.owned(rec.T)
	if len(owned) == 0 {
		return
	}
	for i, f := range rec.Fields {
		rec.Fields[i] = e.bound(f, owned, rec)
	}
}

// bound is v with its unbound refs into owned bound to owner, copied where changed (EVALUATION.md §4.1).
func (e *Evaluator) bound(v value.Value, owned map[*types.Collection]bool, owner *value.Record) value.Value {
	switch x := v.(type) {
	case *value.Ref:
		rt, ok := x.T.Base().(*types.RefType)
		if !ok || x.Owner != nil || !owned[rt.Target] {
			return v
		}
		return e.carry(v, &value.Ref{T: x.T, Key: x.Key, Owner: owner, P: x.P})
	case *value.Record:
		if fields, changed := e.boundAll(x.Fields, owned, owner); changed {
			cp := *x
			cp.Fields = fields
			return e.carry(v, &cp)
		}
	case *value.List:
		if elems, changed := e.boundAll(x.Elems, owned, owner); changed {
			return e.carry(v, &value.List{T: x.T, Elems: elems, P: x.P})
		}
	case *value.Map:
		return e.boundMap(x, owned, owner)
	case *value.Pair:
		if ab, changed := e.boundAll([]value.Value{x.A, x.B}, owned, owner); changed {
			return e.carry(v, &value.Pair{T: x.T, A: ab[0], B: ab[1], P: x.P})
		}
	case *value.Table:
		return e.boundTable(x, owned, owner)
	}
	return v
}

// boundAll binds each value; the slice is a copy when one changed.
func (e *Evaluator) boundAll(vs []value.Value, owned map[*types.Collection]bool, owner *value.Record) ([]value.Value, bool) {
	var out []value.Value
	for i, v := range vs {
		b := e.bound(v, owned, owner)
		if b != v && out == nil {
			out = append([]value.Value(nil), vs...)
		}
		if out != nil {
			out[i] = b
		}
	}
	return out, out != nil
}

func (e *Evaluator) boundMap(m *value.Map, owned map[*types.Collection]bool, owner *value.Record) value.Value {
	keys, kc := e.boundAll(m.Keys, owned, owner)
	vals, vc := e.boundAll(m.Vals, owned, owner)
	if !kc && !vc {
		return m
	}
	if !kc {
		keys = m.Keys
	}
	if !vc {
		vals = m.Vals
	}
	return e.carry(m, &value.Map{T: m.T, Keys: keys, Vals: vals, P: m.P})
}

func (e *Evaluator) boundTable(t *value.Table, owned map[*types.Collection]bool, owner *value.Record) value.Value {
	entries := make([]*value.Record, len(t.Entries))
	changed := false
	for i, en := range t.Entries {
		b, ok := e.bound(en, owned, owner).(*value.Record)
		if !ok {
			b = en
		}
		entries[i], changed = b, changed || b != en
	}
	if !changed {
		return t
	}
	return e.carry(t, &value.Table{T: t.T, Entries: entries, P: t.P})
}
