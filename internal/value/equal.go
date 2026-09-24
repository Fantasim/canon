package value

import "math"

// Equal is value equality (TYP-08): identities when both sides carry one, else structure; a
// ref is never dereferenced. It is EqualUpTo without a limit.
func Equal(a, b Value) bool {
	eq, _, _ := EqualUpTo(a, b, math.MaxInt)
	return eq
}

// EqualUpTo is Equal walked on an explicit stack, counting the composite pairs it visits and
// stopping past limit (ok false); the count is what the evaluator charges (DECISIONS 197).
func EqualUpTo(a, b Value, limit int) (equal, ok bool, visited int) {
	if eq, settled := settle(a, b, false); settled {
		return eq, true, 0
	}
	if _, isComposite := a.(composite); !isComposite {
		return scalarEqual(a, b), true, 0
	}
	w := eqWalk{stack: []eqPair{{a: a, b: b}}}
	for len(w.stack) > 0 {
		if visited >= limit {
			return false, false, limit
		}
		p := w.stack[len(w.stack)-1]
		w.stack = w.stack[:len(w.stack)-1]
		visited++
		w.popped = visited
		if !p.a.(composite).match(p.b, w.push) || w.unequal {
			return false, true, visited
		}
	}
	return true, true, visited
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
// memoFrom, and whether a pair already settled unequal.
type eqWalk struct {
	stack   []eqPair
	seen    map[eqPair]bool
	popped  int
	unequal bool
}

// push settles a pair at once when it can, else keeps it for the walk; past memoFrom popped
// pairs, a pair already taken is not taken again.
func (w *eqWalk) push(a, b Value, plain bool) {
	if eq, settled := settle(a, b, plain); settled {
		w.unequal = w.unequal || !eq
		return
	}
	if _, isComposite := a.(composite); !isComposite {
		w.unequal = w.unequal || !scalarEqual(a, b)
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

// equaler is the equality of each scalar value type of this package.
type equaler interface {
	equal(o Value) bool
}

// composite is a value holding others: match compares what it holds itself against o and
// pushes the pairs of components left to compare.
type composite interface {
	match(o Value, push func(a, b Value, plain bool)) bool
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
func pushSlices(a, b []Value, push func(a, b Value, plain bool)) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		push(a[i], b[i], false)
	}
	return true
}
