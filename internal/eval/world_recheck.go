package eval

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// recheck walks a substituted copy by its declared types, once per container, running again
// the conversions and soft checks its construction ran at nested storage points.
type recheck struct {
	r    *run
	seen map[value.Value]bool
}

// recheckCopy is v, a world's copy of let type t at path at, converted and checked again deep (EVALUATION.md §10.5 item 2, §4.3); nil: a hard error.
func (r *run) recheckCopy(v value.Value, t types.Type, at *vpath) value.Value {
	c := &recheck{r: r, seen: map[value.Value]bool{}}
	v = c.value(v, t, at)
	if r.failed {
		return nil
	}
	return v
}

// declaredBase is t's base under its optionals: the shape a slot of type t holds.
func declaredBase(t types.Type) types.Type {
	for t != nil {
		opt, ok := t.Base().(*types.OptionalType)
		if !ok {
			return t.Base()
		}
		t = opt.Elem
	}
	return nil
}

// value is v at a slot of declared type t: an entry where a ref is declared converted again
// (E3503), else v with its parts rechecked.
func (c *recheck) value(v value.Value, t types.Type, at *vpath) value.Value {
	if v == nil || isNone(v) || c.r.failed {
		return v
	}
	base := declaredBase(t)
	if rt, isRef := base.(*types.RefType); isRef {
		if _, entry := v.(*value.Record); entry {
			return c.r.entryToRef(v, rt, at, nil)
		}
		return v
	}
	if c.seen[v] {
		return v
	}
	c.seen[v] = true
	c.parts(v, base, at)
	return v
}

// parts rechecks the parts of v, each at the type base declares for it, or its own.
func (c *recheck) parts(v value.Value, base types.Type, at *vpath) {
	switch x := v.(type) {
	case *value.Record:
		c.record(x, at)
	case *value.List:
		c.list(x, base, at)
	case *value.Map:
		c.mapParts(x, base, at)
	case *value.Table:
		for _, en := range x.Entries {
			c.value(en, en.T, at.entry(en.Ident.Key))
		}
	case *value.Pair:
		pt, _ := base.(*types.PairType)
		if pt == nil {
			pt, _ = x.T.Base().(*types.PairType)
		}
		if pt != nil {
			x.A, x.B = c.value(x.A, pt.A, at), c.value(x.B, pt.B, at)
		}
	}
}

// record converts and stores each field again at its declared type, as the literal that built it did (records.go fieldItem).
func (c *recheck) record(rec *value.Record, at *vpath) {
	r := c.r
	for i, f := range fieldsOf(rec.T) {
		if i >= len(rec.Fields) || rec.Fields[i] == nil {
			continue
		}
		fat := at.field(f.Name)
		v := c.value(rec.Fields[i], f.Type, fat)
		if v == nil || r.failed {
			return
		}
		if v = r.store(v, f.Type, r.ev.fieldSite(f, rec.T), fat); v == nil {
			return
		}
		rec.Fields[i] = v
	}
}

// list rechecks a keyed list's keys (E3102), then each element at the declared element type.
func (c *recheck) list(l *value.List, base types.Type, at *vpath) {
	lt, _ := base.(*types.ListType)
	if lt == nil {
		lt, _ = l.T.Base().(*types.ListType)
	}
	if lt == nil {
		return
	}
	first := map[value.Key]value.Value{}
	for i, el := range l.Elems {
		eat := at.index(i)
		if rec, isRec := el.(*value.Record); isRec && lt.KeyedBy != nil {
			if k, has := c.r.repeatedKey(first, rec, lt, at); has {
				eat = at.key(k)
			}
		}
		l.Elems[i] = c.value(el, lt.Elem, eat)
	}
}

// mapParts rechecks each key and value at the declared key and value types.
func (c *recheck) mapParts(m *value.Map, base types.Type, at *vpath) {
	kt, vt := mapTypes(base)
	if vt == nil {
		kt, vt = mapTypes(m.T.Base())
	}
	path := mapKeyType(m.T)
	for i, k := range m.Keys {
		vat := at.mapKey(k, path)
		m.Keys[i] = c.value(k, kt, at)
		m.Vals[i] = c.value(m.Vals[i], vt, vat)
	}
}

// mapTypes is the key and value types a map type declares; nil for another type.
func mapTypes(t types.Type) (types.Type, types.Type) {
	switch x := t.(type) {
	case *types.MapType:
		return x.Key, x.Value
	case *types.DepMapType:
		return nil, x.Value
	}
	return nil, nil
}
