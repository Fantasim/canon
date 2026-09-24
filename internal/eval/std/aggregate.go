package std

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// seqZip is E4105 when the lengths differ.
func seqZip(h Host, c *Call) (value.Value, bool) {
	xs, ys := Elems(c.Recv), Elems(c.arg(0))
	if len(xs) != len(ys) {
		h.Fail(diag.E4105.At(h.Site(), int64(len(xs)), int64(len(ys))))
		return nil, false
	}
	pt := elemType(c.Result)
	out := make([]value.Value, len(xs))
	ok := each(h, xs, func(i int, x value.Value) { out[i] = pairOf(pt, x, ys[i], c.Prov) })
	return c.list(out), ok
}

// setOf takes b's elements into a set, one step each.
func setOf(h Host, b []value.Value) (*valueSet, bool) {
	s := newSet()
	ok := each(h, b, func(_ int, y value.Value) { s.add(y) })
	return s, ok
}

// filterBy keeps the receiver's elements that are (or are not) in b, in receiver order; it
// costs n + len(b).
func filterBy(h Host, c *Call, in bool) (value.Value, bool) {
	s, ok := setOf(h, Elems(c.arg(0)))
	if !ok {
		return nil, false
	}
	var out []value.Value
	ok = each(h, Elems(c.Recv), func(_ int, x value.Value) {
		if (s.index(x) >= 0) == in {
			out = append(out, x)
		}
	})
	return c.list(out), ok
}

func seqIntersect(h Host, c *Call) (value.Value, bool) {
	return filterBy(h, c, true)
}

func seqDiff(h Host, c *Call) (value.Value, bool) {
	return filterBy(h, c, false)
}

func seqUnion(h Host, c *Call) (value.Value, bool) {
	s := newSet()
	var out []value.Value
	take := func(_ int, x value.Value) {
		if _, added := s.add(x); added {
			out = append(out, x)
		}
	}
	xs := Elems(c.Recv)
	ok := each(h, xs, func(_ int, x value.Value) { s.add(x) })
	out = append(out, xs...)
	ok = ok && each(h, Elems(c.arg(0)), take)
	return c.list(out), ok
}

// seqGroupBy keeps keys in first-seen order and elements in receiver order.
func seqGroupBy(h Host, c *Call) (value.Value, bool) {
	m := &value.Map{T: c.Result, P: c.Prov}
	lt := mapValueType(c.Result)
	keys := newSet()
	for _, x := range Elems(c.Recv) {
		if !h.Charge(1) {
			return nil, false
		}
		k, ok := h.Invoke(c.arg(0), x)
		if !ok {
			return nil, false
		}
		i, added := keys.add(k)
		if !added {
			l, _ := m.Vals[i].(*value.List)
			l.Elems = append(l.Elems, x)
			continue
		}
		m.Keys = append(m.Keys, k)
		m.Vals = append(m.Vals, &value.List{T: lt, Elems: []value.Value{x}, P: c.Prov})
	}
	return m, true
}

// seqToMap is E4502 on a key produced twice.
func seqToMap(h Host, c *Call) (value.Value, bool) {
	m := &value.Map{T: c.Result, P: c.Prov}
	keys := newSet()
	for _, x := range Elems(c.Recv) {
		if !h.Charge(1) {
			return nil, false
		}
		k, ok := h.Invoke(c.arg(0), x)
		if !ok {
			return nil, false
		}
		v, ok := h.Invoke(c.arg(1), x)
		if !ok {
			return nil, false
		}
		if _, added := keys.add(k); !added {
			h.Fail(diag.E4502.At(h.Site(), k))
			return nil, false
		}
		m.Keys, m.Vals = append(m.Keys, k), append(m.Vals, v)
	}
	return m, true
}

func mapValueType(t types.Type) types.Type {
	if m, ok := t.Base().(*types.MapType); ok {
		return m.Value
	}
	return types.AnyType
}

func seqJoin(h Host, c *Call) (value.Value, bool) {
	xs := Elems(c.Recv)
	parts := make([]string, len(xs))
	ok := each(h, xs, func(i int, x value.Value) { parts[i] = strOf(x) })
	return c.strv(strings.Join(parts, strOf(c.arg(0)))), ok
}

// seqAny stops at the first true, seqAll at the first false: both cost the elements visited.
func seqAny(h Host, c *Call) (value.Value, bool) {
	i, ok := findIndex(h, Elems(c.Recv), c.arg(0))
	return c.boolv(i >= 0), ok
}

func seqAll(h Host, c *Call) (value.Value, bool) {
	for _, x := range Elems(c.Recv) {
		if !h.Charge(1) {
			return nil, false
		}
		yes, ok := holds(h, c.arg(0), x)
		if !ok {
			return nil, false
		}
		if !yes {
			return c.boolv(false), true
		}
	}
	return c.boolv(true), true
}

func seqCount(h Host, c *Call) (value.Value, bool) {
	n := 0
	for _, x := range Elems(c.Recv) {
		if !h.Charge(1) {
			return nil, false
		}
		yes, ok := holds(h, c.arg(0), x)
		if !ok {
			return nil, false
		}
		if yes {
			n++
		}
	}
	return c.intv(int64(n)), true
}

// seqSum adds left to right: E4101 for Int and Duration, E4104 for Float (STDLIB.md §4.3).
func seqSum(h Host, c *Call) (value.Value, bool) {
	acc := zeroOf(c.Result, c.Prov)
	for _, x := range Elems(c.Recv) {
		if !h.Charge(1) {
			return nil, false
		}
		next, ok := add(h, acc, x, c.Prov)
		if !ok {
			return nil, false
		}
		acc = next
	}
	return acc, true
}

// zeroOf is 0, 0.0 or 0s of a numeric type.
func zeroOf(t types.Type, p *value.Prov) value.Value {
	switch t.Base().Kind() {
	case types.Float:
		return &value.Float{T: types.FloatType, P: p}
	case types.Duration:
		return &value.Dur{P: p}
	default:
	}
	return &value.Int{T: types.IntType, P: p}
}

// seqMin and seqMax keep the first of ties; -0.0 orders before +0.0 (STDLIB.md §4.3).
func seqMin(h Host, c *Call) (value.Value, bool) {
	return extreme(h, c, before)
}

func seqMax(h Host, c *Call) (value.Value, bool) {
	return extreme(h, c, func(a, b value.Value) bool { return before(b, a) })
}

func extreme(h Host, c *Call, better func(a, b value.Value) bool) (value.Value, bool) {
	var best value.Value
	ok := each(h, Elems(c.Recv), func(_ int, x value.Value) {
		if best == nil || better(x, best) {
			best = x
		}
	})
	return c.orNone(best, best != nil), ok
}

func seqMinBy(h Host, c *Call) (value.Value, bool) {
	return extremeBy(h, c, Less)
}

func seqMaxBy(h Host, c *Call) (value.Value, bool) {
	return extremeBy(h, c, func(a, b value.Value) bool { return Less(b, a) })
}

// extremeBy invokes the key function once per element in order, then keeps the first best.
func extremeBy(h Host, c *Call, better func(a, b value.Value) bool) (value.Value, bool) {
	xs := Elems(c.Recv)
	keys, ok := keysOf(h, c.arg(0), xs, 1)
	if !ok {
		return nil, false
	}
	if len(xs) == 0 {
		return c.none(), true
	}
	best := 0
	for i := 1; i < len(xs); i++ {
		if better(keys[i], keys[best]) {
			best = i
		}
	}
	return xs[best], true
}

// keysOf invokes f once per element in order, cost steps each first (STDLIB.md §4.4).
func keysOf(h Host, f value.Value, xs []value.Value, cost int) ([]value.Value, bool) {
	keys := make([]value.Value, len(xs))
	for i, x := range xs {
		if !h.Charge(cost) {
			return nil, false
		}
		k, ok := h.Invoke(f, x)
		if !ok {
			return nil, false
		}
		keys[i] = k
	}
	return keys, true
}
