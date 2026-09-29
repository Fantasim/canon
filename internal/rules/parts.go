package rules

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// part is a value inside another, with the type declared where it sits (nil if unknown) and the
// segment its path adds.
type part struct {
	v value.Value
	t types.Type
	s seg
}

// seg is one segment of a value path, kept unbuilt until a finding needs the path (API.md P8, P9).
type seg struct {
	form segForm
	name string      // a field's name
	i    int         // a plain list element's index
	key  value.Key   // a keyed list element's or a table entry's key
	k    value.Value // a map key
	kt   types.Type  // the key type declared where that map is
}

// segForm is how a segment extends a path.
type segForm uint8

// on is the path p extended by s.
func (s seg) on(p *verify.Path) *verify.Path {
	switch s.form {
	case segField:
		return p.Field(s.name)
	case segIndex:
		return p.Index(s.i)
	case segKey:
		return p.Key(s.key)
	case segMapKey:
		return p.MapKey(s.k, s.kt)
	case segEntry:
		return p.Entry(s.key)
	case segSame:
	}
	return p
}

// eachPart calls fn on each part of v, in order, until fn is false (EVALUATION.md §8.1).
func eachPart(v value.Value, fn func(part) bool) {
	namedParts(v, nil, nil, fn)
}

// namedParts is eachPart, a keyed list's element for which late is true named as one without an
// identity: it had none when its list was first met (API.md P8, log-2026-09-29 M4 P12-r).
func namedParts(v value.Value, dt types.Type, late func(*value.Record) bool, fn func(part) bool) {
	switch x := v.(type) {
	case *value.Record:
		recordParts(x, fn)
	case *value.List:
		listParts(x, dt, late, fn)
	case *value.Map:
		mapParts(x, dt, fn)
	case *value.Table:
		tableParts(x, fn)
	case *value.Pair:
		var a, b types.Type
		if pt, ok := verify.Declared(dt).(*types.PairType); ok {
			a, b = pt.A, pt.B
		}
		same := seg{form: segSame}
		_ = fn(part{v: x.A, t: a, s: same}) && fn(part{v: x.B, t: b, s: same})
	}
}

func recordParts(r *value.Record, fn func(part) bool) {
	for i, f := range verify.Fields(r.T) {
		if i < len(r.Fields) && !fn(part{v: r.Fields[i], t: f.Type, s: seg{form: segField, name: f.Name}}) {
			return
		}
	}
}

// listParts are a list's elements; a keyed list's, declared or else stored so, are named by key (API.md P8).
func listParts(l *value.List, dt types.Type, late func(*value.Record) bool, fn func(part) bool) {
	lt, declared := verify.Declared(dt).(*types.ListType)
	var et types.Type
	if declared {
		et = lt.Elem
	} else {
		lt, _ = verify.Declared(l.T).(*types.ListType)
	}
	keyed := lt != nil && lt.KeyedBy != nil
	for i, e := range l.Elems {
		s := seg{form: segIndex, i: i}
		if r, ok := e.(*value.Record); ok && keyed && r.Ident != nil && (late == nil || !late(r)) {
			s = seg{form: segKey, key: r.Ident.Key}
		}
		if !fn(part{v: e, t: et, s: s}) {
			return
		}
	}
}

// mapParts are each key, then its value, named by the key type declared where the map is (API.md P9, log-2026-09-29 M4 U13-r).
func mapParts(m *value.Map, dt types.Type, fn func(part) bool) {
	var vt types.Type
	if mt, ok := verify.Declared(dt).(*types.MapType); ok {
		vt = mt.Value
	}
	kt := verify.KeyTypeAt(dt, m)
	for i, k := range m.Keys {
		s := seg{form: segMapKey, k: k, kt: kt}
		if !fn(part{v: k, s: s}) || i < len(m.Vals) && !fn(part{v: m.Vals[i], t: vt, s: s}) {
			return
		}
	}
}

func tableParts(t *value.Table, fn func(part) bool) {
	for _, e := range t.Entries {
		if e != nil && e.Ident != nil && !fn(part{v: e, s: seg{form: segEntry, key: e.Ident.Key}}) {
			return
		}
	}
}
