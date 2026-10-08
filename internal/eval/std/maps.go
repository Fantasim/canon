package std

import (
	"slices"

	"github.com/fantasim/canonlang/internal/value"
)

// mapMethods are STDLIB.md §6; predicates take (k, v).
func mapMethods() map[string]builtin {
	return map[string]builtin{
		bLen: mapLen, bIsEmpty: mapIsEmpty, bKeys: mapKeys, bValues: mapValues, bGet: mapGet,
		bContains: mapContains, bMap: mapMap, bFilter: mapFilter, bAny: mapAny, bAll: mapAll,
		bCount: mapCount, bUnion: mapUnion,
	}
}

func mapLen(h Host, c *Call) (value.Value, bool) {
	return c.intv(int64(len(c.Recv.(*value.Map).Keys))), h.Charge(1)
}

func mapIsEmpty(h Host, c *Call) (value.Value, bool) {
	return c.boolv(len(c.Recv.(*value.Map).Keys) == 0), h.Charge(1)
}

func mapKeys(h Host, c *Call) (value.Value, bool) {
	m := c.Recv.(*value.Map)
	ok := each(h, m.Keys, func(int, value.Value) {})
	return c.list(append([]value.Value(nil), m.Keys...)), ok
}

func mapValues(h Host, c *Call) (value.Value, bool) {
	m := c.Recv.(*value.Map)
	ok := each(h, m.Vals, func(int, value.Value) {})
	return c.list(append([]value.Value(nil), m.Vals...)), ok
}

func mapGet(h Host, c *Call) (value.Value, bool) {
	m := c.Recv.(*value.Map)
	i, ok := MapIndex(h, m, c.arg(0))
	if !ok {
		return nil, false
	}
	var v value.Value
	if i >= 0 {
		v = m.Vals[i]
	}
	return c.orNone(v, i >= 0), h.Charge(1)
}

func mapContains(h Host, c *Call) (value.Value, bool) {
	i, ok := MapIndex(h, c.Recv.(*value.Map), c.arg(0))
	return c.boolv(i >= 0), ok && h.Charge(1)
}

// MapIndex is the position of key k in m, -1 when it is not a key: found through m's hash
// index, each key compared charged (DECISIONS 199); false once the root aborted.
func MapIndex(h Host, m *value.Map, k value.Value) (int, bool) {
	return m.Lookup(h.Key(m, k), h.Equal)
}

// mapMap keeps the keys and maps each value.
func mapMap(h Host, c *Call) (value.Value, bool) {
	m := c.Recv.(*value.Map)
	out := &value.Map{T: c.Result, Keys: append([]value.Value(nil), m.Keys...), P: c.Prov}
	for _, v := range m.Vals {
		if !h.Charge(1) {
			return nil, false
		}
		y, ok := h.Invoke(c.arg(0), v)
		if !ok {
			return nil, false
		}
		out.Vals = append(out.Vals, y)
	}
	return out, true
}

func mapFilter(h Host, c *Call) (value.Value, bool) {
	m := c.Recv.(*value.Map)
	out := &value.Map{T: c.Result, P: c.Prov}
	for i, k := range m.Keys {
		if !h.Charge(1) {
			return nil, false
		}
		yes, ok := holds(h, c.arg(0), k, m.Vals[i])
		if !ok {
			return nil, false
		}
		if yes {
			out.Keys, out.Vals = append(out.Keys, k), append(out.Vals, m.Vals[i])
		}
	}
	return out, true
}

// mapScan invokes pred on entries in order until it returns stop: the entries visited cost a
// step each; it returns how many held.
func mapScan(h Host, c *Call, stop func(yes bool) bool) (int, bool, bool) {
	m := c.Recv.(*value.Map)
	n := 0
	for i, k := range m.Keys {
		if !h.Charge(1) {
			return 0, false, false
		}
		yes, ok := holds(h, c.arg(0), k, m.Vals[i])
		if !ok {
			return 0, false, false
		}
		if yes {
			n++
		}
		if stop(yes) {
			return n, true, true
		}
	}
	return n, false, true
}

// mapAny stops at the first true, mapAll at the first false.
func mapAny(h Host, c *Call) (value.Value, bool) {
	_, stopped, ok := mapScan(h, c, func(yes bool) bool { return yes })
	return c.boolv(stopped), ok
}

func mapAll(h Host, c *Call) (value.Value, bool) {
	_, stopped, ok := mapScan(h, c, func(yes bool) bool { return !yes })
	return c.boolv(!stopped), ok
}

func mapCount(h Host, c *Call) (value.Value, bool) {
	n, _, ok := mapScan(h, c, func(bool) bool { return false })
	return c.intv(int64(n)), ok
}

// mapUnion is the receiver's entries, then b's whose key it lacks, in b order (DECISIONS 338).
func mapUnion(h Host, c *Call) (value.Value, bool) {
	a, b := c.Recv.(*value.Map), c.arg(0).(*value.Map)
	out := &value.Map{T: c.Result, Keys: slices.Clone(a.Keys), Vals: slices.Clone(a.Vals), P: c.Prov}
	if !each(h, a.Keys, func(int, value.Value) {}) {
		return nil, false
	}
	kt, vt := mapKeyType(c.Result), mapValueType(c.Result)
	ok := eachWhile(h, b.Keys, func(i int, k value.Value) bool {
		ck, ok := h.Coerce(k, kt)
		if !ok {
			return false
		}
		j, ok := MapIndex(h, a, ck)
		if !ok || j >= 0 {
			return ok
		}
		v, ok := h.Coerce(b.Vals[i], vt)
		if !ok {
			return false
		}
		out.Keys, out.Vals = append(out.Keys, ck), append(out.Vals, v)
		return true
	})
	if !ok {
		return nil, false
	}
	return out, true
}
