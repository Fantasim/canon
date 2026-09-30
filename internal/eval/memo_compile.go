package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/value"
)

// compiler keeps a freezer's walk apart as a memoGraph: a slot for each node it copies, for each
// node of a value read and for each value shared as it is, and a template of each node copied.
type compiler struct {
	f  *freezer
	g  memoGraph
	at map[value.Value]slot
}

// nodeCompilers keep each kind's template: what a copy takes as it is, and its parts' slots.
var nodeCompilers = [kindCount]func(*compiler, value.Value, *memoNode){
	kindRecord: compileRecord,
	kindList:   compileList,
	kindMap:    compileMap,
	kindTable:  compileTable,
	kindPair:   compilePair,
	kindRef:    compileRef,
}

// copyKind is the kind of node a graph copies n as, false for a node shared as it is.
func copyKind(n value.Value) (nodeKind, bool) {
	switch x := n.(type) {
	case *value.Record:
		return kindRecord, true
	case *value.List:
		return kindList, true
	case *value.Map:
		return kindMap, true
	case *value.Table:
		return kindTable, true
	case *value.Pair:
		return kindPair, true
	case *value.Ref:
		return kindRef, x.Owner != nil
	}
	return kindRecord, false
}

// kept is the graph walked from root, kept apart: each node copied a template at its slot, each
// node read a slot a replay fills, each other value shared at a negative slot of its own.
func (f *freezer) kept(root value.Value) memoGraph {
	c := &compiler{f: f, at: make(map[value.Value]slot, len(f.seen))}
	var copied []value.Value
	parts := 0
	for _, n := range f.nodes {
		if _, ok := copyKind(n); ok {
			copied = append(copied, n)
			c.at[n] = slot(len(copied))
			parts += partCount(n)
		}
	}
	c.g.parts = make([]slot, 0, parts)
	c.g.foreign = make([]foreignAt, 0, len(f.foreign))
	for n, fa := range f.foreign { //canon:unordered each read node at a slot of its own, all filled by a replay
		c.g.foreign = append(c.g.foreign, fa)
		c.at[n] = slot(len(copied) + len(c.g.foreign))
	}
	c.g.nodes = make([]memoNode, len(copied))
	for i, n := range copied {
		c.node(n, &c.g.nodes[i])
	}
	c.g.root = c.slotOf(root)
	c.g.marks = c.marksOf(&f.marks)
	c.g.shared = append(make([]value.Value, 0, len(c.g.shared)), c.g.shared...)
	c.g.size = len(f.seen)
	return c.g
}

// partCount is how many slots a copied node's parts take in the graph's parts.
func partCount(n value.Value) int {
	switch x := n.(type) {
	case *value.Record:
		return len(x.Fields)
	case *value.List:
		return len(x.Elems)
	case *value.Map:
		return len(x.Keys) + len(x.Vals)
	case *value.Table:
		return len(x.Entries)
	case *value.Pair:
		return pairParts
	}
	return 0
}

// node keeps n's template in nd.
func (c *compiler) node(n value.Value, nd *memoNode) {
	k, _ := copyKind(n)
	nd.kind = k
	nodeCompilers[k](c, n, nd)
	c.g.counts.kinds[k]++
}

// slotOf is v's slot: nilSlot for none, v's own when copied or read, else a new shared one.
func (c *compiler) slotOf(v value.Value) slot {
	if v == nil {
		return nilSlot
	}
	if s, ok := c.at[v]; ok {
		return s
	}
	s := ^slot(len(c.g.shared))
	c.g.shared = append(c.g.shared, v)
	c.at[v] = s
	return s
}

// recordSlot is rec's slot, nilSlot for none.
func (c *compiler) recordSlot(rec *value.Record) slot {
	if rec == nil {
		return nilSlot
	}
	return c.slotOf(rec)
}

// values appends the slots of vs to the graph's parts: where they start, and how many.
func (c *compiler) values(vs []value.Value) (lo, n int) {
	lo = len(c.g.parts)
	for _, v := range vs {
		c.g.parts = append(c.g.parts, c.slotOf(v))
	}
	c.g.counts.vals += len(vs)
	return lo, len(vs)
}

func compileRecord(c *compiler, n value.Value, nd *memoNode) {
	x := n.(*value.Record)
	nd.t, nd.p = x.T, x.P
	if x.Set != nil {
		nd.set = slices.Clone(x.Set)
		c.g.counts.bools += len(x.Set)
	}
	if x.Ident != nil {
		id := *x.Ident
		id.Owner = nil
		nd.ident, nd.owner = &id, c.recordSlot(x.Ident.Owner)
		c.g.counts.idents++
	}
	nd.lo, nd.n = c.values(x.Fields)
}

func compileList(c *compiler, n value.Value, nd *memoNode) {
	x := n.(*value.List)
	nd.t, nd.p = x.T, x.P
	nd.lo, nd.n = c.values(x.Elems)
}

func compileMap(c *compiler, n value.Value, nd *memoNode) {
	x := n.(*value.Map)
	nd.t, nd.p = x.T, x.P
	nd.lo, nd.n = c.values(x.Keys)
	_, nd.m = c.values(x.Vals)
}

func compileTable(c *compiler, n value.Value, nd *memoNode) {
	x := n.(*value.Table)
	nd.t, nd.p = x.T, x.P
	nd.lo, nd.n = len(c.g.parts), len(x.Entries)
	for _, en := range x.Entries {
		c.g.parts = append(c.g.parts, c.recordSlot(en))
	}
	c.g.counts.entries += len(x.Entries)
}

func compilePair(c *compiler, n value.Value, nd *memoNode) {
	x := n.(*value.Pair)
	nd.t, nd.p = x.T, x.P
	nd.lo = len(c.g.parts)
	c.g.parts = append(c.g.parts, c.slotOf(x.A), c.slotOf(x.B))
}

func compileRef(c *compiler, n value.Value, nd *memoNode) {
	x := n.(*value.Ref)
	nd.t, nd.p, nd.key = x.T, x.P, x.Key
	nd.owner = c.recordSlot(x.Owner)
}

// marksOf is m on the graph's slots.
func (c *compiler) marksOf(m *frozenMarks) memoMarks {
	out := memoMarks{
		invalid: c.slots(m.invalid), written: c.slots(m.written),
		history: c.links(m.history), rebuilt: c.links(m.rebuilt), origin: c.links(m.origin),
	}
	for _, rec := range m.bound {
		b := memoBound{rec: c.slotOf(rec)}
		for p, arg := range c.f.e.bound[rec] { //canon:unordered a replay copies them into a map
			b.params, b.args = append(b.params, p), append(b.args, c.slotOf(arg))
		}
		out.bound = append(out.bound, b)
	}
	return out
}

func (c *compiler) slots(vs []value.Value) []slot {
	out := make([]slot, len(vs))
	for i, v := range vs {
		out[i] = c.slotOf(v)
	}
	return out
}

func (c *compiler) links(ls []frozenLink) []memoLink {
	out := make([]memoLink, len(ls))
	for i, l := range ls {
		out[i] = memoLink{from: c.slotOf(l.from), to: c.slotOf(l.to)}
	}
	return out
}
