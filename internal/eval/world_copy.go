package eval

import (
	"cmp"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// worldCopier copies the test's values into a world: every node anew, sharing kept, each
// identity's and ref's owner its copy; the construction facts carried, never a verdict.
type worldCopier struct {
	from, to *Evaluator
	copies   map[value.Value]value.Value
	order    []value.Value
	stack    []value.Value
}

// copyInto copies each value of subs, one copier for all so a node two values share stays one,
// and is every node's copy; false when one holds a node no let can hold.
func copyInto(from, to *Evaluator, subs map[check.Object]value.Value) (map[check.Object]value.Value, map[value.Value]value.Value, bool) {
	objs := slices.SortedFunc(maps.Keys(subs), func(a, b check.Object) int { return cmp.Compare(a.Name(), b.Name()) })
	vs := make([]value.Value, len(objs))
	for i, obj := range objs {
		vs[i] = subs[obj]
	}
	cps, nodes, ok := copyValues(from, to, vs)
	if !ok {
		return nil, nil, false
	}
	out := make(map[check.Object]value.Value, len(subs))
	for i, obj := range objs {
		out[obj] = cps[i]
	}
	return out, nodes, true
}

// copyValues is the copy of each of vs, made by one copier, and every node's copy.
func copyValues(from, to *Evaluator, vs []value.Value) ([]value.Value, map[value.Value]value.Value, bool) {
	c := &worldCopier{from: from, to: to, copies: map[value.Value]value.Value{}}
	for _, v := range vs {
		if !c.walk(v) {
			return nil, nil, false
		}
	}
	for _, n := range c.order {
		c.fill(n, c.copies[n])
	}
	c.mark()
	out := make([]value.Value, len(vs))
	for i, v := range vs {
		out[i] = c.copies[v]
	}
	return out, c.copies, true
}

// walk makes a copy of every node v reaches: its parts, its owners and the values its facts
// name; false at a node no let holds, a function value.
func (c *worldCopier) walk(v value.Value) bool {
	c.stack = append(c.stack, v)
	for len(c.stack) > 0 {
		n := c.stack[len(c.stack)-1]
		c.stack = c.stack[:len(c.stack)-1]
		if n == nil || c.copies[n] != nil {
			continue
		}
		cp := shallow(n)
		if cp == nil {
			return false
		}
		c.copies[n] = cp
		c.order = append(c.order, n)
		c.stack = append(c.stack, parts(n)...)
		c.stack = append(c.stack, c.named(n)...)
	}
	return true
}

// named is what the facts of n name: the value an amendment replaced, the container it
// copies, the instance it copies and its arguments.
func (c *worldCopier) named(n value.Value) []value.Value {
	out := []value.Value{c.from.before(n), c.from.copyOf(n)}
	rec, ok := n.(*value.Record)
	if !ok {
		return out
	}
	if o := c.from.originOf(rec); o != nil {
		out = append(out, o)
	}
	for _, arg := range c.from.boundParams(rec) { //canon:unordered walked into a set
		out = append(out, arg)
	}
	return out
}

// shallow is a new node of n's kind and its own fields, its parts not yet linked; nil for a
// kind no let holds.
func shallow(n value.Value) value.Value {
	switch x := n.(type) {
	case *value.Record:
		return &value.Record{T: x.T, Fields: make([]value.Value, len(x.Fields)), Set: slices.Clone(x.Set), P: x.P}
	case *value.List:
		return &value.List{T: x.T, Elems: make([]value.Value, len(x.Elems)), P: x.P}
	case *value.Map:
		return &value.Map{T: x.T, Keys: make([]value.Value, len(x.Keys)), Vals: make([]value.Value, len(x.Vals)), P: x.P}
	case *value.Table:
		return &value.Table{T: x.T, Entries: make([]*value.Record, len(x.Entries)), P: x.P}
	case *value.Pair:
		return &value.Pair{T: x.T, P: x.P}
	case *value.Ref:
		return &value.Ref{T: x.T, Key: x.Key, P: x.P}
	}
	return shallowScalar(n)
}

// shallowScalar is a new scalar equal to n, nil for a function value or an unknown kind.
func shallowScalar(n value.Value) value.Value {
	switch x := n.(type) {
	case *value.Bool:
		cp := *x
		return &cp
	case *value.Int:
		cp := *x
		return &cp
	case *value.Float:
		cp := *x
		return &cp
	case *value.Str:
		cp := *x
		return &cp
	case *value.Dur:
		cp := *x
		return &cp
	}
	return shallowSymbol(n)
}

// shallowSymbol is shallowScalar for the named scalars.
func shallowSymbol(n value.Value) value.Value {
	switch x := n.(type) {
	case *value.Member:
		cp := *x
		return &cp
	case *value.CaseKind:
		cp := *x
		return &cp
	case *value.Symbol:
		cp := *x
		return &cp
	case *value.None:
		cp := *x
		return &cp
	case *value.Range:
		cp := *x
		return &cp
	}
	return nil
}

// at is the copy of n, nil for none.
func (c *worldCopier) at(n value.Value) value.Value {
	if n == nil {
		return nil
	}
	return c.copies[n]
}

// record is the copy of rec, nil for none.
func (c *worldCopier) record(rec *value.Record) *value.Record {
	if rec == nil {
		return nil
	}
	cp, _ := c.copies[rec].(*value.Record)
	return cp
}

// fill links cp, n's copy, to the copies of n's parts and owners (TYPES.md §6.3).
func (c *worldCopier) fill(n, cp value.Value) {
	switch x := n.(type) {
	case *value.Record:
		to := cp.(*value.Record)
		for i, f := range x.Fields {
			to.Fields[i] = c.at(f)
		}
		if x.Ident != nil {
			id := *x.Ident
			id.Owner = c.record(x.Ident.Owner)
			to.Ident = &id
		}
	case *value.List:
		for i, el := range x.Elems {
			cp.(*value.List).Elems[i] = c.at(el)
		}
	case *value.Map:
		c.fillMap(x, cp.(*value.Map))
	case *value.Table:
		for i, en := range x.Entries {
			cp.(*value.Table).Entries[i] = c.record(en)
		}
	case *value.Pair:
		to := cp.(*value.Pair)
		to.A, to.B = c.at(x.A), c.at(x.B)
	case *value.Ref:
		cp.(*value.Ref).Owner = c.record(x.Owner)
	}
}

func (c *worldCopier) fillMap(m, to *value.Map) {
	for i, k := range m.Keys {
		to.Keys[i] = c.at(k)
	}
	for i, v := range m.Vals {
		to.Vals[i] = c.at(v)
	}
}

// mark carries each node's construction facts to its copy in the world: kept as written, the
// value an amendment replaced, the container it copies, the instance it copies, its arguments.
func (c *worldCopier) mark() {
	from, to := c.from, c.to
	for _, n := range c.order {
		cp := c.copies[n]
		if from.isWritten(n) {
			to.written[cp], to.gens.written = true, to.gens.written+1
		}
		if site := from.writtenAt(n.Prov()); site != n.Prov() {
			to.symSites[cp.Prov()] = site
		}
		if old := from.before(n); old != nil {
			to.history[cp] = c.at(old)
		}
		if src := from.copyOf(n); src != nil {
			to.rebuilt[cp] = c.at(src)
		}
		if rec, ok := n.(*value.Record); ok {
			c.markRecord(rec, cp.(*value.Record))
		}
	}
}

// markRecord carries rec's origin and arguments to cp.
func (c *worldCopier) markRecord(rec, cp *value.Record) {
	if o := c.from.originOf(rec); o != nil {
		c.to.origin[cp] = c.record(o)
	}
	params := c.from.boundParams(rec)
	if params == nil {
		return
	}
	args := make(map[*types.Param]value.Value, len(params))
	for p, arg := range params { //canon:unordered copied into a map
		args[p] = c.at(arg)
	}
	c.to.bound[cp] = args
}
