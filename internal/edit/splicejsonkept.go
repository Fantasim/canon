package edit

import (
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// dropMember leaves field i of nw, stated by member m in holder, to its default (API.md E6, Reset).
// An object holding keys no field claims, which `partial` kept (M8), keeps them, and the default's
// members it would not decode to are written beside them, as W9 writes a default (ruling).
func (d *jsonDiff) dropMember(old, nw *value.Record, i int, m, holder *jsonsrc.Node) error {
	f := fieldsOf(nw.T)[i]
	if !unclaimedIn(m, old.Fields[i]) {
		d.remove(m, memberStart(holder, f.WirePath))
		return nil
	}
	dv, ok := d.a.defaultOf(nw, i)
	def, isRec := dv.(*value.Record)
	was, wasRec := old.Fields[i].(*value.Record)
	switch {
	case !ok || !isRec || !wasRec:
		return &NotEditableError{Reason: ReasonComputed} // no object beside the kept keys decodes to it
	case !sameShape(was.T, def.T) || wiredApart(was, def):
		return d.rewriteKeeping(m, was, wiredDefault(d.a, def), f) // another case: its keys go
	}
	if err := d.value(was, wiredDefault(d.a, def), m, f); err != nil {
		return &NotEditableError{Reason: ReasonComputed}
	}
	return nil
}

// wiredDefault is the record def, a default, with only the fields its record's defaults do not give
// marked written: the members an object must hold to decode to def (WIRE.md 5.4).
func wiredDefault(a *applier, def *value.Record) *value.Record {
	out := withField(def, -1, nil)
	for j := range out.Fields {
		vd, _ := a.defaultVerdict(depFrame{rec: def, a: a}, j)
		out.Set[j] = out.Fields[j] != nil && vd != vEqual
	}
	return out
}

// unclaimedIn reports n an object stating v, a record, that holds a key no field of v's type
// claims, in it or in an object a field of it claims (WIRE.md 5.5).
func unclaimedIn(n *jsonsrc.Node, v value.Value) bool {
	rec, ok := v.(*value.Record)
	if n == nil || n.Kind != jsonsrc.Object || !ok || rec == nil {
		return false
	}
	for _, mem := range n.Members {
		inner, claimed := claimedBy(rec, mem.Key)
		if !claimed || unclaimedIn(mem.Value, inner) {
			return true
		}
	}
	return false
}

// claimedBy reports key claimed in an object stating rec: a field's first key, an inline case's
// tag or key (WIRE.md 5.5, 5.8), with the value a field's one-key path holds. Every key counts as
// claimed beside a pairs field, whose slot keys the wire matches.
func claimedBy(rec *value.Record, key string) (value.Value, bool) {
	if c, isCase := rec.T.Base().(*types.CaseType); isCase && key == c.Variant.Tag {
		return nil, true
	}
	for j, f := range fieldsOf(rec.T) {
		if j >= len(rec.Fields) || f.Input != nil {
			continue
		}
		if v, claimed := fieldClaims(f, rec.Fields[j], key); claimed {
			return v, true
		}
	}
	return nil, false
}

// fieldClaims is claimedBy for field f, holding v.
func fieldClaims(f *types.Field, v value.Value, key string) (value.Value, bool) {
	c, isRec := v.(*value.Record)
	switch {
	case f.Pairs != nil:
		return nil, true
	case f.Inline && isRec && c != nil:
		return claimedBy(c, key)
	case f.Inline:
		return nil, false
	case len(f.WirePath) == 1 && f.WirePath[0] == key:
		return v, true
	}
	return nil, len(f.WirePath) > 1 && f.WirePath[0] == key // an object @json(path:) creates (LOD-09)
}
