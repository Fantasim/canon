package eval

import (
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// letPath reports a let path `v.f….g`: a collection a top-level let holds in a record field (TYPES.md §10.2).
func letPath(c *types.Collection) bool {
	return c != nil && c.Kind != types.CollField && len(c.FieldPath) > 0
}

// inColl reports id, an entry's or a bound field ref's, in c: the same descriptor, or the field
// a let path names, of the instance it reaches (DECISIONS 315); false ok: the run stopped.
func (r *run) inColl(id *value.Identity, c *types.Collection, at syntax.Node, sp source.Span) (yes, ok bool) {
	if id.Coll == c {
		return true, true
	}
	if !letPath(c) || id.Coll == nil || id.Coll.Kind != types.CollField || id.Owner == nil {
		return false, true
	}
	if r.ev.pathField(c) != id.Coll {
		return false, true
	}
	holder := r.pathHolder(c, at, sp)
	if holder == nil {
		return false, false
	}
	return r.ev.instance(r.mv.latest(id.Owner)) == r.ev.instance(holder), true
}

// pathField is the field collection a let path's last field holds, nil when none: memoized per path.
func (e *Evaluator) pathField(c *types.Collection) *types.Collection {
	if via, done := e.pathFields[c]; done {
		return via
	}
	var via *types.Collection
	if st := e.rootState(Root{Pkg: c.Pkg, Name: c.Name}); st != nil {
		t := st.obj.Type()
		last := len(c.FieldPath) - 1
		for _, name := range c.FieldPath[:last] {
			t = fieldType(t, name)
		}
		if rt := recordOf(t); rt != nil {
			if f := fieldNamed(t, c.FieldPath[last]); f != nil {
				via = e.fieldColl(rt, f)
			}
		}
	}
	e.pathFields[c] = via
	return via
}

// fieldType is the type of field name of record type t, the error type when none.
func fieldType(t types.Type, name string) types.Type {
	if f := fieldNamed(t, name); f != nil {
		return f.Type
	}
	return types.ErrorType
}

// fieldNamed is field name of a record, applied record or case type, nil when none.
func fieldNamed(t types.Type, name string) *types.Field {
	if i := fieldIndex(t, name); i >= 0 {
		return fieldsOf(t)[i]
	}
	return nil
}

// pathHolder is the record holding a let path's last field, read like a dereference (EVALUATION.md §4.2).
func (r *run) pathHolder(c *types.Collection, at syntax.Node, sp source.Span) *value.Record {
	base := r.letRoot(c, at, sp)
	if base == nil {
		return nil
	}
	v, decoded := walkFields(base, c.FieldPath[:len(c.FieldPath)-1])
	rec, isRec := v.(*value.Record)
	if !decoded || !isRec { // a let path goes through plain record fields of a let (E3504): never half-decoded
		r.bug(at)
		return nil
	}
	return rec
}

// letRoot is the top-level let a collection descriptor starts at, forced at at, E4301 at sp
// for a let being forced; nil once the run stopped (a fold reads no let, DECISIONS 210).
func (r *run) letRoot(c *types.Collection, at syntax.Node, sp source.Span) value.Value {
	if r.nonConstant() {
		return nil
	}
	st := r.ev.rootState(Root{Pkg: c.Pkg, Name: c.Name})
	if st == nil {
		r.bug(at)
		return nil
	}
	v, ok := r.forceRead(st, at)
	if !ok {
		if at == nil && sp != (source.Span{}) && st.status == forcing && !r.failed { // force reports a cycle only at a node
			r.ev.cycleAt(st, r, sp)
		}
		r.readPoisoned(c.Pkg, c.Name, at)
		return nil
	}
	return v
}

// walkFields follows names from base: nil for a non-record, false at a half-decoded instance (EVALUATION.md §3.2).
func walkFields(base value.Value, names []string) (value.Value, bool) {
	for _, name := range names {
		rec, ok := base.(*value.Record)
		i := -1
		if ok {
			i = fieldIndex(rec.T, name)
		}
		if i < 0 {
			return nil, true
		}
		if rec.Fields[i] == nil {
			return nil, false
		}
		base = rec.Fields[i]
	}
	return base, true
}

// fieldIdentity is the identity a value names in field form: an entry's, or a bound field ref's; nil for another.
func fieldIdentity(v value.Value) *value.Identity {
	switch x := v.(type) {
	case *value.Record:
		if x.Ident != nil && x.Ident.Coll != nil && x.Ident.Coll.Kind == types.CollField {
			return x.Ident
		}
	case *value.Ref:
		rt, ok := x.T.Base().(*types.RefType)
		if ok && rt.Target != nil && rt.Target.Kind == types.CollField && x.Owner != nil {
			return &value.Identity{Coll: rt.Target, Owner: x.Owner, Key: x.Key}
		}
	}
	return nil
}

// letPathRef is v as a ref through a let path, nil for another value.
func letPathRef(v value.Value) *value.Ref {
	x, ok := v.(*value.Ref)
	if !ok {
		return nil
	}
	if rt, isRef := x.T.Base().(*types.RefType); isRef && letPath(rt.Target) {
		return x
	}
	return nil
}

// Belongs reports en an entry of c, through a let path its field's instance (std.Host, DECISIONS 315).
func (h *stdHost) Belongs(en *value.Record, c *types.Collection) (yes, ok bool) {
	if en.Ident == nil {
		return false, true
	}
	return h.r.inColl(en.Ident, c, nil, h.r.site)
}
