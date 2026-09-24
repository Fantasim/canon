package std

import "github.com/fantasim/canonlang/internal/value"

// keyed is an element with its sort key.
type keyed struct {
	key, elem value.Value
}

// seqSortBy is the stable merge sort of STDLIB.md §4.5.
func seqSortBy(h Host, c *Call) (value.Value, bool) {
	xs := Elems(c.Recv)
	keys, ok := keysOf(h, c.arg(0), xs, 0)
	if !ok {
		return nil, false
	}
	a := make([]keyed, len(xs))
	for i, x := range xs {
		a[i] = keyed{key: keys[i], elem: x}
	}
	sorted, ok := mergeSort(h, a)
	if !ok {
		return nil, false
	}
	out := make([]value.Value, len(sorted))
	for i, k := range sorted {
		out[i] = k.elem
	}
	return c.list(out), true
}

// mergeSort is the top-down merge sort of STDLIB.md §4.5; equal keys keep the left side first.
func mergeSort(h Host, a []keyed) ([]keyed, bool) {
	if len(a) <= 1 {
		return a, true
	}
	mid := len(a) / halfDenominator
	l, ok := mergeSort(h, a[:mid])
	if !ok {
		return nil, false
	}
	r, ok := mergeSort(h, a[mid:])
	if !ok {
		return nil, false
	}
	out := make([]keyed, 0, len(a))
	for len(l) > 0 && len(r) > 0 {
		if !h.Charge(1) {
			return nil, false
		}
		if Less(r[0].key, l[0].key) {
			out, r = append(out, r[0]), r[1:]
		} else {
			out, l = append(out, l[0]), l[1:]
		}
	}
	return append(append(out, l...), r...), true
}
