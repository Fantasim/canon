package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// hasDefault reports a field that may be left out: it has a default, or it is optional (WIRE.md §5.4).
func hasDefault(f *types.Field) bool {
	return f.Input == nil && (f.Default != nil || isOptional(f.Type))
}

// defaultOf is field i's default in rec, computed from rec's earlier fields, each left out
// taking its own default first (TYP-15); false when the field has none or the host cannot
// compute it (a default reading a field no value was given).
func (a *applier) defaultOf(rec *value.Record, i int) (value.Value, bool) {
	return a.defaultIn(rec, i, nil)
}

// defaultIn is defaultOf in a record bound to params, as the decoder computes it (TYPES.md 11.1).
func (a *applier) defaultIn(rec *value.Record, i int, params map[*types.Param]value.Value) (value.Value, bool) {
	fields := fieldsOf(rec.T)
	if i >= len(fields) || !hasDefault(fields[i]) {
		return nil, false
	}
	full := &value.Record{T: rec.T, Fields: slices.Clone(rec.Fields), Set: slices.Clone(rec.Set), Ident: rec.Ident, P: rec.P}
	for j := range i {
		if full.Fields[j] == nil && hasDefault(fields[j]) {
			full.Fields[j], _ = a.host.Default(a.ctx, fields[j], wire.Instance{Record: full, Params: params}, nil)
		}
	}
	return a.host.Default(a.ctx, fields[i], wire.Instance{Record: full, Params: params}, nil)
}

// isDefault reports field i of rec holding its default (E6), by sameDefault under an edit layer;
// never a value a base Set pins, nor a record materialized for it (setsDefault).
func (a *applier) isDefault(rec *value.Record, i int) bool {
	if i < len(rec.Fields) && slices.Contains(a.pinned, rec.Fields[i]) {
		return false
	}
	d, ok := a.defaultOf(rec, i)
	if a.env.EditLayer == "" {
		return ok && sameValue(d, rec.Fields[i])
	}
	return ok && a.sameDefault(d, rec.Fields[i])
}

// unchanged reports nw, a value the diff compares, equal to old (sameValue); while a Set pins a
// value equal to the base's default (pinned), only old itself, so the path down to it is walked.
func (a *applier) unchanged(old, nw value.Value) bool {
	if len(a.pinned) > 0 {
		return old == nw
	}
	return sameValue(old, nw)
}

// sameDefault is sameValue with a record's field left out read as its default, the equality E6
// names: a literal `{}` equals a default whose evaluation gives every field.
func (a *applier) sameDefault(x, y value.Value) bool {
	switch xv := x.(type) {
	case *value.Record:
		yv, ok := y.(*value.Record)
		return ok && a.sameFields(xv, yv)
	case *value.List:
		yv, ok := y.(*value.List)
		return ok && slices.EqualFunc(xv.Elems, yv.Elems, a.sameDefault)
	case *value.Map:
		yv, ok := y.(*value.Map)
		return ok && sameValues(xv.Keys, yv.Keys) && slices.EqualFunc(xv.Vals, yv.Vals, a.sameDefault)
	case *value.Table:
		yv, ok := y.(*value.Table)
		return ok && slices.EqualFunc(xv.Entries, yv.Entries, func(p, q *value.Record) bool {
			return entryKey(p) == entryKey(q) && isRetired(p) == isRetired(q) && a.sameFields(p, q)
		})
	}
	return sameValue(x, y)
}

// sameFields reports records of one shape whose fields are sameDefault, each left out as its default;
// one whose default no call computes out of its record's context (a parameter it reads) equals the
// other side's unwritten value, which the evaluation computed in that context.
func (a *applier) sameFields(x, y *value.Record) bool {
	if !sameShape(x.T, y.T) || len(x.Fields) != len(y.Fields) {
		return false
	}
	for j := range x.Fields {
		xv, yv := a.fieldOrDefault(x, j), a.fieldOrDefault(y, j)
		if xv == nil && unwritten(y, j) || yv == nil && unwritten(x, j) {
			continue
		}
		if !a.sameDefault(xv, yv) {
			return false
		}
	}
	return true
}

// unwritten reports field j of r holding a value its source does not write: its default.
func unwritten(r *value.Record, j int) bool {
	return r.Fields[j] != nil && (j >= len(r.Set) || !r.Set[j])
}

// fieldOrDefault is field j of r, or its default in r when r leaves it out; nil for none computed.
func (a *applier) fieldOrDefault(r *value.Record, j int) value.Value {
	if r.Fields[j] != nil {
		return r.Fields[j]
	}
	d, _ := a.defaultOf(r, j)
	return d
}

// printed reports a field a new literal writes: written, and not equal to its default (M7).
func (a *applier) printed(rec *value.Record, i int) bool {
	written := i < len(rec.Set) && rec.Set[i] && i < len(rec.Fields) && rec.Fields[i] != nil
	return written && fieldsOf(rec.T)[i].Input == nil && !a.isDefault(rec, i)
}

// sameValue is value equality (TYP-08) of values that may carry no identity or
// provenance, records field by field and a ref by its key; maps and tables in order, since
// their order is text an edit keeps.
func sameValue(x, y value.Value) bool {
	if x == nil || y == nil {
		return x == y
	}
	switch a := x.(type) {
	case *value.Record:
		b, ok := y.(*value.Record)
		return ok && sameRecord(a, b)
	case *value.List:
		b, ok := y.(*value.List)
		return ok && sameValues(a.Elems, b.Elems)
	case *value.Map:
		b, ok := y.(*value.Map)
		return ok && sameMap(a, b)
	case *value.Table:
		b, ok := y.(*value.Table)
		return ok && sameTable(a, b)
	case *value.Ref:
		b, ok := y.(*value.Ref)
		return ok && a.Key == b.Key
	}
	return value.Equal(x, y)
}

func sameValues(xs, ys []value.Value) bool {
	return slices.EqualFunc(xs, ys, sameValue)
}

// sameRecord compares the case or record and every field.
func sameRecord(a, b *value.Record) bool {
	return sameShape(a.T, b.T) && sameValues(a.Fields, b.Fields)
}

// sameShape reports two record or case types of one declaration.
func sameShape(x, y types.Type) bool {
	switch a := x.Base().(type) {
	case *types.CaseType:
		b, ok := y.Base().(*types.CaseType)
		return ok && a.Variant == b.Variant && a.Index == b.Index
	case *types.AppliedRecord:
		b, ok := y.Base().(*types.AppliedRecord)
		return ok && a.Rec == b.Rec
	}
	return x.Base() == y.Base()
}

// sameMap is the same keys in the same order, each key's values equal.
func sameMap(a, b *value.Map) bool {
	return sameValues(a.Keys, b.Keys) && sameValues(a.Vals, b.Vals)
}

func sameTable(a, b *value.Table) bool {
	return slices.EqualFunc(a.Entries, b.Entries, func(x, y *value.Record) bool {
		return entryKey(x) == entryKey(y) && isRetired(x) == isRetired(y) && sameRecord(x, y)
	})
}

// entryKey is a table entry's key, zero for a record without identity.
func entryKey(r *value.Record) value.Key {
	if r.Ident == nil {
		return value.Key{}
	}
	return r.Ident.Key
}

// isRetired reports a retired table entry.
func isRetired(r *value.Record) bool {
	return r.Ident != nil && r.Ident.Retired
}
