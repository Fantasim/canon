package edit

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// verdict is E6's judgement of a value against a default: unequal, equal or undecided.
type verdict uint8

// and is v and w judged together: unequal wins, then undecided.
func (v verdict) and(w verdict) verdict {
	if v == vUnequal || w == vUnequal {
		return vUnequal
	}
	return max(v, w)
}

// verdictOf is a two-valued judgement as a verdict.
func verdictOf(equal bool) verdict {
	if equal {
		return vEqual
	}
	return vUnequal
}

// sameIn judges x and y, held where frames fx and fy stand, by TYPES.md 7.5 with a field left out
// read as its default, its record's parameters bound (TYPES.md 11.1); undecided where a side needs
// a default no call computes (API.md E6).
func (a *applier) sameIn(x, y value.Value, fx, fy depFrame) verdict {
	switch xv := x.(type) {
	case *value.Record:
		yv, ok := y.(*value.Record)
		if !ok || yv == nil {
			return vUnequal
		}
		return a.sameRecordIn(xv, yv, fx.enter(xv, nil), fy.enter(yv, nil))
	case *value.List:
		yv, ok := y.(*value.List)
		if !ok || len(xv.Elems) != len(yv.Elems) {
			return vUnequal
		}
		return a.sameAllIn(xv.Elems, yv.Elems, fx, fy)
	case *value.Map:
		return a.sameMapIn(xv, y, fx, fy)
	case *value.Table:
		return a.sameTableIn(xv, y, fx, fy)
	}
	return verdictOf(sameValue(x, y))
}

// sameAllIn judges xs and ys position by position.
func (a *applier) sameAllIn(xs, ys []value.Value, fx, fy depFrame) verdict {
	out := vEqual
	for i := range xs {
		if out = out.and(a.sameIn(xs[i], ys[i], fx, fy)); out == vUnequal {
			return out
		}
	}
	return out
}

// sameMapIn judges maps of the same keys in the same order value by value, a dependent map's
// binder bound to each key (TYPES.md 11.5).
func (a *applier) sameMapIn(x *value.Map, y value.Value, fx, fy depFrame) verdict {
	yv, ok := y.(*value.Map)
	if !ok || !sameValues(x.Keys, yv.Keys) {
		return vUnequal
	}
	dm, dep := x.T.Base().(*types.DepMapType)
	out := vEqual
	for i, k := range x.Keys {
		bx, by := fx, fy
		if dep {
			bx, by = fx.bind(dm.Binder, k), fy.bind(dm.Binder, k)
		}
		if out = out.and(a.sameIn(x.Vals[i], yv.Vals[i], bx, by)); out == vUnequal {
			return out
		}
	}
	return out
}

// sameTableIn judges tables of the same keys in the same order, retired alike, entry by entry.
func (a *applier) sameTableIn(x *value.Table, y value.Value, fx, fy depFrame) verdict {
	yv, ok := y.(*value.Table)
	if !ok || len(x.Entries) != len(yv.Entries) {
		return vUnequal
	}
	out := vEqual
	for i, p := range x.Entries {
		q := yv.Entries[i]
		if entryKey(p) != entryKey(q) || isRetired(p) != isRetired(q) {
			return vUnequal
		}
		if out = out.and(a.sameRecordIn(p, q, fx.enter(p, nil), fy.enter(q, nil))); out == vUnequal {
			return out
		}
	}
	return out
}

// sameRecordIn judges records of one shape field by field, fx and fy their own frames.
func (a *applier) sameRecordIn(x, y *value.Record, fx, fy depFrame) verdict {
	if !sameShape(x.T, y.T) || len(x.Fields) != len(y.Fields) {
		return vUnequal
	}
	fields := fieldsOf(x.T)
	out := vEqual
	for j := range x.Fields {
		if j >= len(fields) || fields[j].Input != nil {
			continue // an input field holds no value (TYP-17)
		}
		if out = out.and(a.sameFieldIn(x, y, fields[j], fx, fy)); out == vUnequal {
			return out
		}
	}
	return out
}

// sameFieldIn judges field f of x and y, each its value or its default. Under an edit layer, a
// default no call computes equals the other side's unwritten value, which the evaluation computed
// in the line's context; anything else that lacks a value is undecided.
func (a *applier) sameFieldIn(x, y *value.Record, f *types.Field, fx, fy depFrame) verdict {
	xv, yv := fx.field(f), fy.field(f)
	j := f.Index
	switch {
	case xv != nil && yv != nil:
		return a.sameIn(xv, yv, fx, fy)
	case a.env.EditLayer != "" && (xv == nil && unwritten(y, j) || yv == nil && unwritten(x, j)):
		return vEqual
	}
	return vUndecided
}
