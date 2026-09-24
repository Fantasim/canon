package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/value"
)

// History is path's last value, then each value amendments replaced to set it, newest first (CLI.md §3.7).
func (e *Evaluator) History(path ...value.Value) []value.Value {
	var out []value.Value
	e.historyOf(path, map[value.Value]bool{}, &out)
	return out
}

// historyOf walks the value's own replacements, then, nearest first, those of each ancestor an
// amendment replaced whole, at the same place in what it replaced (orchestrator log, call d).
func (e *Evaluator) historyOf(path []value.Value, seen map[value.Value]bool, out *[]value.Value) {
	if len(path) == 0 {
		return
	}
	for v := path[len(path)-1]; v != nil && !seen[v]; v = e.before(v) {
		seen[v] = true
		*out = append(*out, v)
	}
	ancestors := path[:len(path)-1]
	for i := range slices.Backward(ancestors) {
		old := e.before(path[i])
		if old == nil || seen[old] {
			continue // an older ancestor already walked leads nowhere new
		}
		if sub := samePlace(path[i:], old); sub != nil {
			seen[old] = true
			e.historyOf(sub, seen, out)
		}
	}
}

// before is the value an amendment replaced with v, nil for none.
func (e *Evaluator) before(v value.Value) value.Value {
	if old, ok := e.history[v]; ok {
		return old
	}
	if e.parent != nil {
		return e.parent.before(v)
	}
	return nil
}

// remember records that an amendment set nv where old was (EVALUATION.md §9.3 step 2).
func (e *Evaluator) remember(nv, old value.Value) {
	if nv != nil && old != nil && nv != old {
		e.history[nv] = old
	}
}

// samePlace is path's steps taken again from old: old, then the value at each same place, nil
// when one is missing.
func samePlace(path []value.Value, old value.Value) []value.Value {
	out := []value.Value{old}
	for i := 1; i < len(path) && old != nil; i++ {
		old = placeIn(old, path[i-1], path[i])
		out = append(out, old)
	}
	if old == nil {
		return nil
	}
	return out
}

// placeIn is the component of o at the place child has in parent: a field by name, an entry
// by key, a map value or key by key, an element by index.
func placeIn(o, parent, child value.Value) value.Value {
	switch p := parent.(type) {
	case *value.Record:
		return fieldPlace(o, p, child)
	case *value.Map:
		return mapPlace(o, p, child)
	}
	for i, el := range std.Elems(parent) {
		if el == child {
			return elemPlace(o, el, i)
		}
	}
	return nil
}

func fieldPlace(o value.Value, p *value.Record, child value.Value) value.Value {
	rec, ok := o.(*value.Record)
	if !ok {
		return nil
	}
	for i, f := range fieldsOf(p.T) {
		if i < len(p.Fields) && p.Fields[i] == child {
			if j := fieldIndex(rec.T, f.Name); j >= 0 {
				return rec.Fields[j]
			}
		}
	}
	return nil
}

func mapPlace(o value.Value, p *value.Map, child value.Value) value.Value {
	m, ok := o.(*value.Map)
	if !ok {
		return nil
	}
	for i := range p.Keys {
		if p.Keys[i] != child && p.Vals[i] != child {
			continue
		}
		if j := keyPosition(m, p.Keys[i]); j >= 0 && p.Keys[i] == child {
			return m.Keys[j]
		} else if j >= 0 {
			return m.Vals[j]
		}
	}
	return nil
}

// elemPlace is o's entry of el's key, else o's element at index i.
func elemPlace(o, el value.Value, i int) value.Value {
	elems := std.Elems(o)
	if rec, ok := el.(*value.Record); ok && rec.Ident != nil {
		for _, x := range elems {
			if xr, isRec := x.(*value.Record); isRec && xr.Ident != nil && xr.Ident.Key == rec.Ident.Key {
				return x
			}
		}
		return nil
	}
	if i < len(elems) {
		return elems[i]
	}
	return nil
}
