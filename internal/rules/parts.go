package rules

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// part is a value inside another, with the type declared where it sits (nil if unknown) and its path.
type part struct {
	v  value.Value
	t  types.Type
	at *verify.Path
}

// parts is what the traversal and the invalid test follow of v, declared dt, in the order of EVALUATION.md §8.1.
func parts(v value.Value, dt types.Type, at *verify.Path) []part {
	switch x := v.(type) {
	case *value.Record:
		return recordParts(x, at)
	case *value.List:
		return listParts(x, dt, at)
	case *value.Map:
		return mapParts(x, dt, at)
	case *value.Table:
		return tableParts(x, at)
	case *value.Pair:
		var a, b types.Type
		if pt, ok := verify.Declared(dt).(*types.PairType); ok {
			a, b = pt.A, pt.B
		}
		return []part{{x.A, a, at}, {x.B, b, at}}
	}
	return nil
}

func recordParts(r *value.Record, at *verify.Path) []part {
	out := make([]part, 0, len(r.Fields))
	for i, f := range verify.Fields(r.T) {
		if i < len(r.Fields) {
			out = append(out, part{r.Fields[i], f.Type, at.Field(f.Name)})
		}
	}
	return out
}

// listParts are a list's elements; a keyed list's, declared or else stored so, are named by key (API.md P8).
func listParts(l *value.List, dt types.Type, at *verify.Path) []part {
	lt, declared := verify.Declared(dt).(*types.ListType)
	var et types.Type
	if declared {
		et = lt.Elem
	} else {
		lt, _ = verify.Declared(l.T).(*types.ListType)
	}
	keyed := lt != nil && lt.KeyedBy != nil
	out := make([]part, 0, len(l.Elems))
	for i, e := range l.Elems {
		p := at.Index(i)
		if r, ok := e.(*value.Record); ok && keyed && r.Ident != nil {
			p = at.Key(r.Ident.Key)
		}
		out = append(out, part{e, et, p})
	}
	return out
}

// mapParts are each key, then its value, named by the key type declared where the map is (API.md P9, log-2026-09-29 M4 U13-r).
func mapParts(m *value.Map, dt types.Type, at *verify.Path) []part {
	var vt types.Type
	if mt, ok := verify.Declared(dt).(*types.MapType); ok {
		vt = mt.Value
	}
	out := make([]part, 0, len(m.Keys)+len(m.Vals))
	kt := verify.KeyTypeAt(dt, m)
	for i, k := range m.Keys {
		p := at.MapKey(k, kt)
		out = append(out, part{k, nil, p})
		if i < len(m.Vals) {
			out = append(out, part{m.Vals[i], vt, p})
		}
	}
	return out
}

func tableParts(t *value.Table, at *verify.Path) []part {
	out := make([]part, 0, len(t.Entries))
	for _, e := range t.Entries {
		if e != nil && e.Ident != nil {
			out = append(out, part{e, nil, at.Entry(e.Ident.Key)})
		}
	}
	return out
}
