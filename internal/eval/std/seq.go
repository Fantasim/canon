package std

import (
	"github.com/fantasim/canonlang/internal/value"
)

// seqMethods are STDLIB.md §4: every method of Seq(T), in its order.
func seqMethods() map[string]builtin {
	return map[string]builtin{
		bLen: seqLen, bIsEmpty: seqIsEmpty, bFirst: seqFirst, bLast: seqLast,
		bContains: seqContains, bIndexOf: seqIndexOf, bMap: seqMap, bFilter: seqFilter,
		bFlatMap: seqFlatMap, bFlatten: seqFlatten, bReverse: seqReverse, bSortBy: seqSortBy,
		bUnique: seqUnique, bEnumerate: seqEnumerate, bPairs: seqPairs, bZip: seqZip,
		bIntersect: seqIntersect, bUnion: seqUnion, bDiff: seqDiff, bGroupBy: seqGroupBy,
		bToMap: seqToMap, bJoin: seqJoin, bAny: seqAny, bAll: seqAll, bCount: seqCount,
		bIsUnique: seqIsUnique, bSum: seqSum, bMin: seqMin, bMax: seqMax, bMinBy: seqMinBy,
		bMaxBy: seqMaxBy,
	}
}

func seqLen(h Host, c *Call) (value.Value, bool) {
	return c.intv(int64(len(Elems(c.Recv)))), h.Charge(1)
}

func seqIsEmpty(h Host, c *Call) (value.Value, bool) {
	return c.boolv(len(Elems(c.Recv)) == 0), h.Charge(1)
}

// seqFirst is first() (overload 0) or first(pred) (overload 1), which costs the elements visited.
func seqFirst(h Host, c *Call) (value.Value, bool) {
	xs := Elems(c.Recv)
	if c.Overload == 0 {
		if len(xs) == 0 {
			return c.none(), h.Charge(1)
		}
		return xs[0], h.Charge(1)
	}
	i, ok := findIndex(h, xs, c.arg(0))
	if !ok {
		return nil, false
	}
	if i < 0 {
		return c.none(), true
	}
	return xs[i], true
}

// findIndex is the index of the first element pred holds for, -1 for none; one step each visited.
func findIndex(h Host, xs []value.Value, pred value.Value) (int, bool) {
	for i, x := range xs {
		if !h.Charge(1) {
			return 0, false
		}
		yes, ok := holds(h, pred, x)
		if !ok {
			return 0, false
		}
		if yes {
			return i, true
		}
	}
	return -1, true
}

func seqLast(h Host, c *Call) (value.Value, bool) {
	xs := Elems(c.Recv)
	if len(xs) == 0 {
		return c.none(), h.Charge(1)
	}
	return xs[len(xs)-1], h.Charge(1)
}

// seqContains tests an element, or a key of a keyed collection (STDLIB.md §5).
func seqContains(h Host, c *Call) (value.Value, bool) {
	i, ok := position(h, c.Recv, c.arg(0))
	return c.boolv(i >= 0), ok
}

// position is the index of the first element equal to x, or of the entry of key x; each
// element visited costs one step.
func position(h Host, recv, x value.Value) (int, bool) {
	byKey := isKey(recv, x)
	k, _ := KeyOf(x)
	for i, e := range Elems(recv) {
		if !h.Charge(1) {
			return 0, false
		}
		if byKey {
			if ek, _ := KeyOf(e); ek == k {
				return i, true
			}
		} else if value.Equal(e, x) {
			return i, true
		}
	}
	return -1, true
}

// isKey reports x used as a key of a keyed receiver.
func isKey(recv, x value.Value) bool {
	if familyOf(recv) != famKeyed {
		return false
	}
	switch x.(type) {
	case *value.Record, *value.Ref:
		return false
	}
	return true
}

func seqIndexOf(h Host, c *Call) (value.Value, bool) {
	i, ok := position(h, c.Recv, c.arg(0))
	return c.orNone(c.intv(int64(i)), i >= 0), ok
}

func seqMap(h Host, c *Call) (value.Value, bool) {
	xs := Elems(c.Recv)
	out := make([]value.Value, 0, len(xs))
	for _, x := range xs {
		if !h.Charge(1) {
			return nil, false
		}
		y, ok := h.Invoke(c.arg(0), x)
		if !ok {
			return nil, false
		}
		out = append(out, y)
	}
	return c.list(out), true
}

func seqFilter(h Host, c *Call) (value.Value, bool) {
	var out []value.Value
	for _, x := range Elems(c.Recv) {
		if !h.Charge(1) {
			return nil, false
		}
		yes, ok := holds(h, c.arg(0), x)
		if !ok {
			return nil, false
		}
		if yes {
			out = append(out, x)
		}
	}
	return c.list(out), true
}

// seqFlatMap costs n plus the length of the result.
func seqFlatMap(h Host, c *Call) (value.Value, bool) {
	var out []value.Value
	for _, x := range Elems(c.Recv) {
		if !h.Charge(1) {
			return nil, false
		}
		y, ok := h.Invoke(c.arg(0), x)
		if !ok {
			return nil, false
		}
		inner := Elems(y)
		if !h.Charge(len(inner)) {
			return nil, false
		}
		out = append(out, inner...)
	}
	return c.list(out), true
}

func seqFlatten(h Host, c *Call) (value.Value, bool) {
	var out []value.Value
	for _, x := range Elems(c.Recv) {
		inner := Elems(x)
		if !h.Charge(1 + len(inner)) {
			return nil, false
		}
		out = append(out, inner...)
	}
	return c.list(out), true
}

// each charges one step per element as it is taken, in order (DECISIONS 185), and calls fn
// on it; false once the root is aborted.
func each(h Host, xs []value.Value, fn func(i int, x value.Value)) bool {
	for i, x := range xs {
		if !h.Charge(1) {
			return false
		}
		fn(i, x)
	}
	return true
}

func seqReverse(h Host, c *Call) (value.Value, bool) {
	xs := Elems(c.Recv)
	out := make([]value.Value, len(xs))
	ok := each(h, xs, func(i int, x value.Value) { out[len(xs)-1-i] = x })
	return c.list(out), ok
}

func seqUnique(h Host, c *Call) (value.Value, bool) {
	seen := newSet()
	var out []value.Value
	ok := each(h, Elems(c.Recv), func(_ int, x value.Value) {
		if _, added := seen.add(x); added {
			out = append(out, x)
		}
	})
	return c.list(out), ok
}

// seqIsUnique costs n, whatever it finds.
func seqIsUnique(h Host, c *Call) (value.Value, bool) {
	seen := newSet()
	unique := true
	ok := each(h, Elems(c.Recv), func(_ int, x value.Value) {
		if _, added := seen.add(x); !added {
			unique = false
		}
	})
	return c.boolv(unique), ok
}

func seqEnumerate(h Host, c *Call) (value.Value, bool) {
	xs := Elems(c.Recv)
	pt := elemType(c.Result)
	out := make([]value.Value, len(xs))
	ok := each(h, xs, func(i int, x value.Value) { out[i] = pairOf(pt, c.intv(int64(i)), x, c.Prov) })
	return c.list(out), ok
}

// seqPairs costs one step per pair, charged as it is made.
func seqPairs(h Host, c *Call) (value.Value, bool) {
	xs := Elems(c.Recv)
	pt := elemType(c.Result)
	var out []value.Value
	for i := range xs {
		for _, y := range xs[i+1:] {
			if !h.Charge(1) {
				return nil, false
			}
			out = append(out, pairOf(pt, xs[i], y, c.Prov))
		}
	}
	return c.list(out), true
}
