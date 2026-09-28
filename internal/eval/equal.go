package eval

import (
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// eqPair is two values left to compare; plain compares structure only, a table's entries.
type eqPair struct {
	a, b  value.Value
	plain bool
}

// eqWalk is value.EqualUpTo's walk that first converts a value kept as written meeting a converted one, a step more (TYPES.md §11.6).
type eqWalk struct {
	r       *run
	at      func() source.Span
	stack   []eqPair
	seen    map[eqPair]bool
	popped  int
	unequal bool
	over    bool
}

// equal is value equality, E4401 at at once the budget runs out; false ok: the root aborted.
func (r *run) equal(a, b value.Value, at func() source.Span) (bool, bool) {
	w := &eqWalk{r: r, at: at}
	w.push(a, b, false)
	for len(w.stack) > 0 && !w.unequal && !w.over {
		p := w.stack[len(w.stack)-1]
		w.stack = w.stack[:len(w.stack)-1]
		w.popped++
		if w.count(1) && !w.match(p.a, p.b) {
			w.unequal = true
		}
	}
	if w.over {
		return false, false
	}
	return !w.unequal, true
}

// count charges n pairs compared; false once the budget ran out or the root aborted.
func (w *eqWalk) count(n int) bool {
	if !w.r.spend(n, w.at) {
		w.over = true
		return false
	}
	return true
}

// push compares a pair at once when value equality settles it alone, else keeps it for the
// walk; past eqMemoFrom popped pairs, a pair already taken is not taken again.
func (w *eqWalk) push(a, b value.Value, plain bool) {
	if w.unequal || w.over {
		return
	}
	if a, b = w.resolved(a, b); w.over {
		return
	}
	if !walked(a, b, plain) {
		eq, done, n := value.EqualUpTo(a, b, w.r.remaining())
		w.unequal = w.count(n) && done && !eq
		w.over = w.over || !done
		return
	}
	p := eqPair{a: a, b: b, plain: plain}
	if w.popped >= eqMemoFrom {
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

// walked reports a pair this walk takes apart: two composites, not the same instance nor, unless
// plain, two identities, which value equality settles alone.
func walked(a, b value.Value, plain bool) bool {
	if a == b || !plain && hasIdentity(a) && hasIdentity(b) {
		return false
	}
	switch a.(type) {
	case *value.Record, *value.Table:
		return true
	}
	return IsContainer(a)
}

// hasIdentity reports a ref or an entry, which value equality compares by identity.
func hasIdentity(v value.Value) bool {
	switch x := v.(type) {
	case *value.Ref:
		return true
	case *value.Record:
		return x.Ident != nil
	}
	return false
}

// match compares what a holds itself against b and pushes the pairs of their components.
func (w *eqWalk) match(a, b value.Value) bool {
	switch x := a.(type) {
	case *value.List:
		y, ok := b.(*value.List)
		return ok && w.pushAll(x.Elems, y.Elems)
	case *value.Pair:
		y, ok := b.(*value.Pair)
		return ok && w.pushAll([]value.Value{x.A, x.B}, []value.Value{y.A, y.B})
	case *value.Record:
		y, ok := b.(*value.Record)
		return ok && sameShape(x, y) && w.pushFields(x.Fields, y.Fields)
	case *value.Map:
		y, ok := b.(*value.Map)
		return ok && len(x.Keys) == len(y.Keys) && w.matchMaps(x, y)
	case *value.Table:
		y, ok := b.(*value.Table)
		return ok && w.matchTables(x, y)
	}
	return false
}

// matchTables is the same keys in the same order, each entry pushed to compare field-wise.
func (w *eqWalk) matchTables(x, y *value.Table) bool {
	if len(x.Entries) != len(y.Entries) {
		return false
	}
	for i, e := range x.Entries {
		if e.Ident.Key != y.Entries[i].Ident.Key {
			return false
		}
		w.push(e, y.Entries[i], true)
	}
	return true
}

// pushAll pushes the pairs of two slices of the same length.
func (w *eqWalk) pushAll(a, b []value.Value) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		w.push(a[i], b[i], false)
	}
	return true
}

// pushFields pushes the pairs of two records' fields; a field set on one side only differs.
func (w *eqWalk) pushFields(a, b []value.Value) bool {
	for i := range a {
		if (a[i] == nil) != (b[i] == nil) {
			return false
		}
		if a[i] != nil {
			w.push(a[i], b[i], false)
		}
	}
	return true
}

// sameShape reports two records of one declaration and as many fields (arguments erased).
func sameShape(a, b *value.Record) bool {
	return declOf(a.T) == declOf(b.T) && len(a.Fields) == len(b.Fields)
}

func declOf(t types.Type) types.Type {
	if a, ok := t.Base().(*types.AppliedRecord); ok {
		return a.Rec
	}
	return t.Base()
}

// matchMaps finds each key of the map whose keys are kept as written in the other, converted
// against its key type, and pushes the pairs of their values (DECISIONS 199).
func (w *eqWalk) matchMaps(x, y *value.Map) bool {
	if w.r.ev.dependent(mapKeyType(y.T)) && !w.r.ev.dependent(mapKeyType(x.T)) {
		x, y = y, x
	}
	for i, k := range x.Keys {
		k = w.r.keyFor(y, k)
		j, ok := y.Lookup(k, w.sameKey)
		if !ok || j < 0 {
			return false
		}
		w.push(x.Vals[i], y.Vals[j], false)
	}
	return true
}

// sameKey compares two keys a lookup meets, each pair counted.
func (w *eqWalk) sameKey(a, b value.Value) (bool, bool) {
	if !w.count(1) {
		return false, false
	}
	return value.Equal(a, b), true
}

// resolved is a and b with a value kept as written that meets a converted one converted to its type.
func (w *eqWalk) resolved(a, b value.Value) (value.Value, value.Value) {
	switch {
	case w.keptAs(a, b):
		return w.convert(a, b), b
	case w.keptAs(b, a):
		return a, w.convert(b, a)
	}
	return a, b
}

// keptAs reports x kept as written against like: a symbol, or a written string or integer against a ref or Float (TYPES.md §11.4).
func (w *eqWalk) keptAs(x, like value.Value) bool {
	switch x.(type) {
	case *value.Symbol:
		_, sym := like.(*value.Symbol)
		return !sym
	case *value.Str, *value.Int:
		_, isRef := like.(*value.Ref)
		_, isFloat := like.(*value.Float)
		_, isInt := x.(*value.Int)
		return (isRef || isInt && isFloat) && w.r.ev.isWritten(x)
	}
	return false
}

// convert is x as a value of like's type, a step; x itself when that type holds no such value.
func (w *eqWalk) convert(x, like value.Value) value.Value {
	t := like.Type()
	if t == nil || !w.count(1) {
		return x
	}
	v := x
	if s, ok := x.(*value.Symbol); ok && named(t) {
		v = w.r.symbolAs(s, t)
	} else if nv, fit := ToBranch(x, branchBase(t), true); fit == Fits {
		v = nv
	}
	lr, isRef := like.(*value.Ref)
	if ref, ok := v.(*value.Ref); ok && isRef && v != x {
		ref.T, ref.Owner = lr.T, lr.Owner
	}
	return v
}
