package verify

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// reach is how a ref's target collection was reached.
type reach uint8

// ref checks the key's entry exists, and is not retired outside a retired entry and a past slot (TYPES.md §10.3).
func (w *walker) ref(r *value.Ref, t types.Type, sc scope, at *Path) {
	rt, ok := t.(*types.RefType)
	if !ok {
		rt, ok = r.T.Base().(*types.RefType)
	}
	if !ok || rt.Target == nil {
		return
	}
	entries, how := w.collection(r, rt.Target)
	w.noteRef(rt.Target, r.Key, how, entries[r.Key])
	switch how {
	case poisoned:
		w.invalid(r)
		return
	case unbound:
		w.unbound(r, at)
		return
	case reached:
	}
	target, found := entries[r.Key]
	if !found {
		s := SiteOf(r)
		w.flag(s, w.src.related(diag.E3501.At(s.Span, r, w.collName(rt.Target)), rt), r, at)
		return
	}
	w.retiredTarget(r, rt, target, at, sc)
}

// collection forces the ref's target and indexes its entries by key.
func (w *walker) collection(r *value.Ref, c *types.Collection) (map[value.Key]*value.Record, reach) {
	var root value.Value
	if c.Kind == types.CollField {
		if r.Owner == nil {
			return nil, unbound
		}
		root = r.Owner
	} else {
		v, ok := w.ev.Force(w.ctx, eval.Root{Pkg: c.Pkg, Name: c.Name})
		if !ok {
			return nil, poisoned
		}
		root = v
	}
	return w.index(follow(root, c.FieldPath)), reached
}

// follow reads a path of record fields; nil when a field is absent.
func follow(v value.Value, names []string) value.Value {
	for _, name := range names {
		r, ok := v.(*value.Record)
		if !ok {
			return nil
		}
		v = nil
		for i, f := range Fields(r.T) {
			if f.Name == name && i < len(r.Fields) {
				v = r.Fields[i]
			}
		}
	}
	return v
}

// index maps each key of a table or keyed list to its first entry, once per collection value.
func (w *walker) index(coll value.Value) map[value.Key]*value.Record {
	if coll == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if idx, ok := w.indexes[coll]; ok {
		return idx
	}
	idx := map[value.Key]*value.Record{}
	for _, e := range entriesOf(coll) {
		if _, seen := idx[e.Ident.Key]; !seen {
			idx[e.Ident.Key] = e
		}
	}
	w.indexes[coll] = idx
	return idx
}

// entriesOf is the elements of a collection that carry an identity.
func entriesOf(coll value.Value) []*value.Record {
	var out []*value.Record
	switch c := coll.(type) {
	case *value.Table:
		out = c.Entries
	case *value.List:
		for _, e := range c.Elems {
			if r, ok := e.(*value.Record); ok {
				out = append(out, r)
			}
		}
	}
	var kept []*value.Record
	for _, e := range out {
		if e != nil && e.Ident != nil {
			kept = append(kept, e)
		}
	}
	return kept
}

// collName names a collection as E3501 does: qualified when it is another package's.
func (w *walker) collName(c *types.Collection) string {
	if c.Kind == types.CollField && c.Owner != nil {
		return w.local(c.Owner.Pkg, c.Owner.Name, c.FieldPath...)
	}
	return w.local(c.Pkg, c.Name, c.FieldPath...)
}
