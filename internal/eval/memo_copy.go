package eval

import (
	"reflect"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// thawer copies a kept graph into a table of slots, taking each node, identity and slice from
// arrays allocated once per copy; each slice is capped at its own length, so no append reaches
// another's.
type thawer struct {
	g       *memoGraph
	tab     []value.Value
	recs    []value.Record
	lists   []value.List
	maps    []value.Map
	tables  []value.Table
	pairs   []value.Pair
	refs    []value.Ref
	idents  []value.Identity
	vals    []value.Value
	entries []*value.Record
	bools   []bool
}

// table is a table of g's slots, its nodes and nodes read left to fill.
func (g *memoGraph) table() []value.Value {
	return make([]value.Value, int(nilSlot)+1+len(g.nodes)+len(g.foreign))
}

// seed is a table of g's slots holding, for each node read, the node of the values read now,
// infos, at its place; false when one is of another type, which equal fingerprints exclude but
// a replay checks.
func (g *memoGraph) seed(infos []*readInfo) ([]value.Value, bool) {
	tab := g.table()
	at := int(nilSlot) + 1 + len(g.nodes)
	for i, fa := range g.foreign {
		nodes := infos[fa.read].nodes
		if fa.pos >= len(nodes) || reflect.TypeOf(nodes[fa.pos]) != fa.typ {
			return nil, false
		}
		tab[at+i] = nodes[fa.pos]
	}
	return tab, true
}

// thaw is a new copy of g's root for e, linked to the nodes tab seeds (nil for a graph that read
// none), its marks set in e as the entry's evaluation set them.
func (e *Evaluator) thaw(g *memoGraph, tab []value.Value) value.Value {
	if tab == nil {
		tab = g.table()
	}
	th := thawer{g: g, tab: tab}
	th.allocate()
	for i := range g.nodes {
		tab[int(nilSlot)+1+i] = th.make(&g.nodes[i])
	}
	for i := range g.nodes {
		th.fill(&g.nodes[i], tab[int(nilSlot)+1+i])
	}
	th.mark(e)
	return th.at(g.root)
}

// allocate takes at once what a copy of the graph needs of each kind.
func (th *thawer) allocate() {
	c := &th.g.counts
	th.recs = make([]value.Record, c.kinds[kindRecord])
	th.lists = make([]value.List, c.kinds[kindList])
	th.maps = make([]value.Map, c.kinds[kindMap])
	th.tables = make([]value.Table, c.kinds[kindTable])
	th.pairs = make([]value.Pair, c.kinds[kindPair])
	th.refs = make([]value.Ref, c.kinds[kindRef])
	th.idents = make([]value.Identity, c.idents)
	th.vals = make([]value.Value, c.vals)
	th.entries = make([]*value.Record, c.entries)
	th.bools = make([]bool, c.bools)
}

// at is the value at slot s: a copy, a node read, or a value shared.
func (th *thawer) at(s slot) value.Value {
	if s < nilSlot {
		return th.g.shared[^s]
	}
	return th.tab[s]
}

// record is the record at s, nil for none.
func (th *thawer) record(s slot) *value.Record {
	rec, _ := th.at(s).(*value.Record)
	return rec
}

// mark sets in e the marks the graph keeps, on the copies.
func (th *thawer) mark(e *Evaluator) {
	m := &th.g.marks
	for _, s := range m.invalid {
		e.invalid[th.at(s)] = true
	}
	for _, s := range m.written {
		e.written[th.at(s)] = true
		e.gens.written++
	}
	for _, l := range m.history {
		e.history[th.at(l.from)] = th.at(l.to)
	}
	for _, l := range m.rebuilt {
		e.rebuilt[th.at(l.from)] = th.at(l.to)
	}
	for _, l := range m.origin {
		e.origin[th.at(l.from).(*value.Record)] = th.at(l.to).(*value.Record)
	}
	for _, b := range m.bound {
		params := make(map[*types.Param]value.Value, len(b.params))
		for i, p := range b.params {
			params[p] = th.at(b.args[i])
		}
		e.bound[th.at(b.rec).(*value.Record)] = params
	}
}

// values is a new slice of the n values at lo in the graph's parts.
func (th *thawer) values(lo, n int) []value.Value {
	out := th.vals[:n:n]
	th.vals = th.vals[n:]
	for i, s := range th.g.parts[lo : lo+n] {
		out[i] = th.at(s)
	}
	return out
}

// make is nd's copy, its parts not yet linked.
func (th *thawer) make(nd *memoNode) value.Value {
	switch nd.kind {
	case kindRecord:
		return th.makeRecord(nd)
	case kindList:
		l := &th.lists[0]
		th.lists = th.lists[1:]
		l.T, l.P = nd.t, nd.p
		return l
	case kindMap:
		m := &th.maps[0]
		th.maps = th.maps[1:]
		m.T, m.P = nd.t, nd.p
		return m
	case kindTable:
		t := &th.tables[0]
		th.tables = th.tables[1:]
		t.T, t.P = nd.t, nd.p
		return t
	case kindPair:
		p := &th.pairs[0]
		th.pairs = th.pairs[1:]
		p.T, p.P = nd.t, nd.p
		return p
	case kindRef:
		r := &th.refs[0]
		th.refs = th.refs[1:]
		r.T, r.Key, r.P = nd.t, nd.key, nd.p
		return r
	}
	return nil // copyKind makes no other kind
}

func (th *thawer) makeRecord(nd *memoNode) *value.Record {
	rec := &th.recs[0]
	th.recs = th.recs[1:]
	rec.T, rec.P = nd.t, nd.p
	if nd.set != nil {
		n := len(nd.set)
		rec.Set = th.bools[:n:n]
		th.bools = th.bools[n:]
		copy(rec.Set, nd.set)
	}
	return rec
}

// fill links v, nd's copy, to the copies of its parts.
func (th *thawer) fill(nd *memoNode, v value.Value) {
	switch x := v.(type) {
	case *value.Record:
		th.fillRecord(nd, x)
	case *value.List:
		x.Elems = th.values(nd.lo, nd.n)
	case *value.Map:
		x.Keys, x.Vals = th.values(nd.lo, nd.n), th.values(nd.lo+nd.n, nd.m)
	case *value.Table:
		th.fillTable(nd, x)
	case *value.Pair:
		x.A, x.B = th.at(th.g.parts[nd.lo]), th.at(th.g.parts[nd.lo+1])
	case *value.Ref:
		x.Owner = th.record(nd.owner)
	}
}

func (th *thawer) fillRecord(nd *memoNode, rec *value.Record) {
	rec.Fields = th.values(nd.lo, nd.n)
	if nd.ident == nil {
		return
	}
	id := &th.idents[0]
	th.idents = th.idents[1:]
	*id = *nd.ident
	id.Owner = th.record(nd.owner)
	rec.Ident = id
}

func (th *thawer) fillTable(nd *memoNode, t *value.Table) {
	out := th.entries[:nd.n:nd.n]
	th.entries = th.entries[nd.n:]
	for i, s := range th.g.parts[nd.lo : nd.lo+nd.n] {
		out[i] = th.record(s)
	}
	t.Entries = out
}
