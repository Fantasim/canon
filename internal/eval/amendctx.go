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
		j := -1
		if seg.Name != nil {
			j = fieldIndex(x.T, seg.Name.Name)
		}
		if j < 0 {
			return nil, nil, false
		}
		f := fieldsOf(x.T)[j]
		m.at = append(m.at, x)
		m.dep = &depCtx{rec: x, params: r.ev.boundParams(x), field: f.Type, at: m.a.Value}
		return x.Fields[j], f.Type, true
	case *value.Map:
		j, key, ok := r.mapSlot(x, seg, m)
		if ok && binderOf(t) != "" {
			m.dep = m.dep.withBinder(binderOf(t), key, m.a.Value)
		}
		if !ok || j < 0 {
			return nil, nil, false
		}
		return x.Vals[j], mapValueType(t), true
	}
	if !keyedColl(cur) && !isPlainList(cur) {
		return nil, nil, false
	}
	j, _, ok := r.slotOf(cur, seg, m)
	if !ok || j < 0 {
		return nil, nil, false
	}
	return std.Elems(cur)[j], elementType(t), true
}

// mapValueType is the value type of a map or dependent map type.
func mapValueType(t types.Type) types.Type {
	if d, ok := t.Base().(*types.DepMapType); ok {
		return d.Value
	}
	return elementType(t)
}
