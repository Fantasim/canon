package value

import "math"

// Equal is value equality (TYP-08): identities when both sides carry one, else structure; a
// ref is never dereferenced. It is EqualUpTo without a limit.
func Equal(a, b Value) bool {
	eq, _, _ := EqualUpTo(a, b, math.MaxInt)
	return eq
}

// EqualUpTo is Equal on an explicit stack, counting the pairs it compares, scalar pairs
// included, and stopping at limit (ok false); the count is what the evaluator charges
// (DECISIONS 199).
func EqualUpTo(a, b Value, limit int) (equal, ok bool, compared int) {
	if limit < 1 {
		return false, false, limit
	}
	eq, settled := settle(a, b, false)
	if _, isComposite := a.(composite); !settled && !isComposite {
		eq, settled = scalarEqual(a, b), true
	}
	if settled {
		return eq, true, 1
	}
	w := eqWalk{limit: limit}
	w.push(a, b, false)
	for len(w.stack) > 0 && !w.unequal && !w.over {
		p := w.stack[len(w.stack)-1]
		w.stack = w.stack[:len(w.stack)-1]
		w.popped++
		if !w.count() {
			break
		}
		if !p.a.(composite).match(p.b, &w) {
			w.unequal = true
		}
	}
	if w.over {
		return false, false, limit
	}
	return !w.unequal, true, w.compared
}

// settle decides a pair without walking it: the same instance, or two identities (unless
// plain, a table's entries compared field-wise).
func settle(a, b Value, plain bool) (equal, settled bool) {
	if a == b {
		return true, true
	}
	if !plain {
		if ia, ib := identity(a), identity(b); ia != nil && ib != nil {
			return ia.same(ib), true
		}
	}
	return false, false
}

// scalarEqual is the equality of two values that hold no others.
func scalarEqual(a, b Value) bool {
	if e, ok := a.(equaler); ok {
		return e.equal(b)
	}
	return false
}

// eqPair is two composites left to compare; plain compares structure only, identities
// ignored (a table's entries against each other).
type eqPair struct {
	a, b  Value
	plain bool
}

// eqWalk is one equality walk: the pairs left, the pairs already taken once the walk is past
// memoFrom, the pairs compared against the limit, and how it ended.
type eqWalk struct {
	stack    []eqPair
	seen     map[eqPair]bool
	popped   int
	compared int
	limit    int
	unequal  bool
	over     bool
}

// count counts one pair compared; false once that passes the limit.
func (w *eqWalk) count() bool {
	if w.compared >= w.limit {
		w.over = true
		return false
	}
	w.compared++
	return true
}

// push compares a pair at once when it can (the same instance, identities, scalars), else
// keeps it for the walk; past memoFrom popped pairs, a pair already taken is not taken again.
func (w *eqWalk) push(a, b Value, plain bool) {
	if w.unequal || w.over {
		return
	}
	eq, settled := settle(a, b, plain)
	if _, isComposite := a.(composite); !settled && !isComposite {
		eq, settled = scalarEqual(a, b), true
	}
	if settled {
		w.unequal = w.count() && !eq
		return
	}
	p := eqPair{a: a, b: b, plain: plain}
	if w.popped >= memoFrom {
		if w.seen[p] {
			return
		}
		if w.seen == nil {
			w.seen = map[eqPair]bool{}
		}
		w.seen[p] = true
	}
	w.stack = append(w.stack, p)
}

// key is the position of k among m's keys, each key compared counted as a pair.
func (w *eqWalk) key(m *Map, k Value) int {
	i, _ := m.Lookup(k, func(a, b Value) (bool, bool) {
		if !w.count() {
			return false, false
		}
		return Equal(a, b), true
	})
	return i
}

// equaler is the equality of each scalar value type of this package.
type equaler interface {
	equal(o Value) bool
}

// composite is a value holding others: match compares what it holds itself against o and
// pushes the pairs of components left to compare onto w.
type composite interface {
	match(o Value, w *eqWalk) bool
}

// identity is the identity a value carries: a ref's target entry, or an entry's own.
func identity(v Value) *Identity {
	switch x := v.(type) {
	case *Ref:
		return x.identity()
	case *Record:
		return x.Ident
	}
	return nil
}

// pushSlices pushes the pairs of two slices of the same length.
func pushSlices(a, b []Value, w *eqWalk) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		w.push(a[i], b[i], false)
	}
	return true
}
