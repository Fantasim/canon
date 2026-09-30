package eval

import (
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// locate follows an amendment's path in cur without replacing anything, so its value is evaluated
// with the type arguments where the path lands; the keys it evaluates are kept for the replace,
// which reports a missing segment.
func (r *run) locate(cur value.Value, t types.Type, m *amending) {
	m.quiet = true
	defer func() { m.quiet, m.at = false, nil }()
	for i, seg := range m.a.Path {
		var ok bool
		if cur, t, ok = r.locStep(cur, t, seg, m); !ok || i == len(m.a.Path)-1 || cur == nil || isNone(cur) {
			return
		}
		t = unwrapOptional(t)
	}
}

// locStep is the value one segment names in cur and its type, m.dep then where it lands.
func (r *run) locStep(cur value.Value, t types.Type, seg *syntax.AmendSegment, m *amending) (value.Value, types.Type, bool) {
	switch x := cur.(type) {
	case *value.Record:
		return r.locField(x, seg, m)
	case *value.Map:
		return r.locMapKey(x, t, seg, m)
	}
	if !keyedColl(cur) && !isPlainList(cur) {
		return nil, nil, false
	}
	j, key, ok := r.slotOf(cur, seg, m)
	if ok {
		m.step(elementPath(m.located(), cur, j, key))
	}
	if !ok || j < 0 {
		return nil, nil, false
	}
	return std.Elems(cur)[j], elementType(t), true
}

// locField is the field a segment names in rec.
func (r *run) locField(rec *value.Record, seg *syntax.AmendSegment, m *amending) (value.Value, types.Type, bool) {
	j := -1
	if seg.Name != nil {
		j = fieldIndex(rec.T, seg.Name.Name)
	}
	if j < 0 {
		return nil, nil, false
	}
	f := fieldsOf(rec.T)[j]
	m.step(m.located().field(f.Name))
	m.at = append(m.at, rec)
	m.dep = &depCtx{rec: rec, params: r.ev.boundParams(rec), field: f.Type, at: m.a.Value}
	return rec.Fields[j], f.Type, true
}

// locMapKey is the value a segment names in map mp, whose key binds a dependent type's parameter.
func (r *run) locMapKey(mp *value.Map, t types.Type, seg *syntax.AmendSegment, m *amending) (value.Value, types.Type, bool) {
	j, key, ok := r.mapSlot(mp, seg, m)
	if ok {
		m.step(m.located().key(mapKey(key)))
	}
	if ok && binderOf(t) != "" {
		m.dep = m.dep.withBinder(binderOf(t), key, m.a.Value)
	}
	if !ok || j < 0 {
		return nil, nil, false
	}
	return mp.Vals[j], mapValueType(t), true
}

// mapValueType is the value type of a map or dependent map type.
func mapValueType(t types.Type) types.Type {
	if d, ok := t.Base().(*types.DepMapType); ok {
		return d.Value
	}
	return elementType(t)
}
