package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// convert applies the conversion the checker recorded at e (TYPES.md §6.2).
func (r *run) convert(v value.Value, c *check.Conversion, e syntax.Expr, at *vpath) value.Value {
	if c == nil || v == nil {
		return v
	}
	switch c.Kind {
	case check.ConvWrap:
		return r.convert(v, c.Inner, e, at)
	case check.ConvPresent:
		if isNone(v) {
			return v
		}
		return r.convert(v, c.Inner, e, at)
	case check.ConvDeref:
		return r.deref(v, e)
	case check.ConvEntryToRef:
		return r.entryToRef(v, c.To, at)
	case check.ConvIntLitToFloat:
		if i, ok := v.(*value.Int); ok {
			return r.ev.carry(v, &value.Float{V: float64(i.V), T: types.FloatType, P: i.P})
		}
	case check.ConvToList:
		return r.ev.carry(v, &value.List{T: c.To, Elems: std.Elems(v), P: v.Prov()})
	case check.ConvElements:
		return r.convertElements(v, c, e, at)
	default:
	}
	return v
}

// entryToRef is `T ≤ ref T`; anything but an entry of the target is E3503 (TYPES.md §6.2).
func (r *run) entryToRef(v value.Value, to types.Type, at *vpath) value.Value {
	rt, isRef := to.Base().(*types.RefType)
	rec, isRec := v.(*value.Record)
	if !isRef || !isRec {
		return v
	}
	if rec.Ident == nil || rec.Ident.Coll != rt.Target {
		b := diag.E3503.At(located(v, source.Span{}), rec.T, r.collName(rt.Target))
		r.soft(b, rec, at)
		return rec
	}
	return r.ev.mark(rec, &value.Ref{T: to, Key: rec.Ident.Key, Owner: rec.Ident.Owner, P: rec.P})
}

// convertElements converts each element of a list, each key and value of a map, or the two
// halves of a pair; a list converted to a keyed list takes identities and unique keys.
func (r *run) convertElements(v value.Value, c *check.Conversion, e syntax.Expr, at *vpath) value.Value {
	switch x := v.(type) {
	case *value.List:
		return r.convertList(x, c, e, at)
	case *value.Map:
		out := &value.Map{T: c.To, Keys: make([]value.Value, len(x.Keys)), Vals: make([]value.Value, len(x.Vals)), P: x.P}
		for i, k := range x.Keys {
			out.Keys[i] = r.convert(k, c.Key, e, at)
			out.Vals[i] = r.convert(x.Vals[i], c.Inner, e, at.mapKey(k, mapKeyType(c.To)))
			if out.Keys[i] == nil || out.Vals[i] == nil {
				return nil
			}
		}
		return r.ev.carry(v, out)
	case *value.Pair:
		a, b := r.convert(x.A, c.Key, e, at), r.convert(x.B, c.Inner, e, at)
		if a == nil || b == nil {
			return nil
		}
		return r.ev.carry(v, &value.Pair{T: c.To, A: a, B: b, P: x.P})
	}
	return v
}

// convertList converts each element; a list converted to a keyed list takes its identities.
func (r *run) convertList(x *value.List, c *check.Conversion, e syntax.Expr, at *vpath) value.Value {
	elems := make([]value.Value, len(x.Elems))
	lt, _ := c.To.Base().(*types.ListType)
	for i, el := range x.Elems {
		eat := at.element(lt, i)
		eat.learnFrom(el)
		if elems[i] = r.convert(el, c.Inner, e, eat); elems[i] == nil {
			return nil
		}
	}
	if lt != nil && lt.KeyedBy != nil {
		return r.ev.carry(x, r.keyedList(elems, c.To, nil, x.P, at))
	}
	return r.ev.carry(x, &value.List{T: c.To, Elems: elems, P: x.P})
}

// keyedList gives records the identities of a keyed list, E3102 on a repeated key (TYPES.md §9.1).
func (r *run) keyedList(elems []value.Value, t types.Type, coll *types.Collection, p *value.Prov, at *vpath) value.Value {
	lt, _ := t.Base().(*types.ListType)
	if coll == nil {
		coll = &types.Collection{Kind: types.CollLet, Elem: lt.Elem, KeyedBy: lt.KeyedBy}
	}
	first := map[value.Key]value.Value{}
	out := make([]value.Value, len(elems))
	for i, el := range elems {
		out[i] = el
		rec, ok := el.(*value.Record)
		if !ok || lt.KeyedBy.Index >= len(rec.Fields) || rec.Fields[lt.KeyedBy.Index] == nil {
			continue
		}
		kv := rec.Fields[lt.KeyedBy.Index]
		k, _ := std.KeyOf(kv)
		if prev, dup := first[k]; dup {
			b := diag.E3102.AtKey(located(kv, source.Span{}), kv, located(prev, source.Span{}))
			r.soft(b, kv, at.key(k).field(lt.KeyedBy.Name))
		} else {
			first[k] = kv
		}
		out[i] = r.ev.withIdentity(rec, &value.Identity{Coll: coll, Key: k})
	}
	return &value.List{T: t, Elems: out, P: p}
}

func isNone(v value.Value) bool {
	_, ok := v.(*value.None)
	return ok
}

// coerce converts a value a built-in or a function value passes on as a storage point would (TYPES.md §6.2).
func (r *run) coerce(v value.Value, t types.Type, at syntax.Node) value.Value {
	if v == nil || t == nil || isNone(v) {
		return v
	}
	if s, ok := v.(*value.Symbol); ok {
		return r.valueAs(s, t)
	}
	switch b := unwrapOptional(t).Base().(type) {
	case *types.RefType:
		if _, isRec := v.(*value.Record); isRec {
			return r.entryToRef(v, b, nil)
		}
	case *types.RecordType, *types.CaseType, *types.AppliedRecord:
		if _, isRef := v.(*value.Ref); isRef {
			return r.deref(v, at)
		}
	}
	return v
}
