package std

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// graph is one call of reachable, cycles or topoSort (STDLIB.md §2.3).
type graph struct {
	h     Host
	c     *Call
	next  value.Value
	nodes *valueSet
	succ  map[int][]int
}

// identKey is a node's identity (TYPES.md §7.5): refs and entries compare by it.
type identKey struct {
	coll  *types.Collection
	owner *value.Record
	key   value.Key
}

func newGraph(h Host, c *Call, next value.Value) *graph {
	return &graph{h: h, c: c, next: next, nodes: newSet(), succ: map[int][]int{}}
}

// id numbers a node, the same number for equal nodes (TYPES.md §7.5).
func (g *graph) id(v value.Value) int {
	i, _ := g.nodes.add(v)
	return i
}

// node is the value of node i.
func (g *graph) node(i int) value.Value {
	return g.nodes.vals[i]
}

// successors invokes next once for node i: a list (overload 0) or at most one successor.
// Each successor examined costs a step.
func (g *graph) successors(i int) ([]int, bool) {
	if s, done := g.succ[i]; done {
		return s, true
	}
	v, ok := g.h.Invoke(g.next, g.node(i))
	if !ok {
		return nil, false
	}
	var out []value.Value
	switch {
	case g.c.Overload == 0:
		out = Elems(v)
	case !isNone(v):
		out = []value.Value{v}
	}
	if !g.h.Charge(len(out)) {
		return nil, false
	}
	node := elemType(g.c.Result)
	ids := make([]int, len(out))
	for j, s := range out {
		n, ok := g.h.Coerce(s, node)
		if !ok {
			return nil, false
		}
		ids[j] = g.id(n)
	}
	g.succ[i] = ids
	return ids, true
}

func isNone(v value.Value) bool {
	_, ok := v.(*value.None)
	return ok
}

// graphReachable lists every node reachable from `from`, depth-first preorder; one step per
// node listed and per successor examined.
func graphReachable(h Host, c *Call) (value.Value, bool) {
	g := newGraph(h, c, c.arg(1))
	listed := map[int]bool{}
	var out []value.Value
	var visit func(i int) bool
	visit = func(i int) bool {
		listed[i] = true
		out = append(out, g.node(i))
		if !h.Charge(1) {
			return false
		}
		succ, ok := g.successors(i)
		for _, s := range succ {
			if ok && !listed[s] {
				ok = visit(s)
			}
		}
		return ok
	}
	if !visit(g.id(c.arg(0))) {
		return nil, false
	}
	return c.list(out), true
}

// seqNodes numbers the distinct elements of xs, in order.
func (g *graph) seqNodes(xs value.Value) []int {
	var order []int
	seen := map[int]bool{}
	for _, x := range Elems(xs) {
		if i := g.id(x); !seen[i] {
			seen[i] = true
			order = append(order, i)
		}
	}
	return order
}

// nodeValues are the values of numbered nodes.
func (g *graph) nodeValues(ids []int) []value.Value {
	out := make([]value.Value, len(ids))
	for j, i := range ids {
		out[j] = g.node(i)
	}
	return out
}

// chain is the text of nodes for a Chain argument.
func (g *graph) chain(ids []int) []string {
	out := make([]string, len(ids))
	for j, i := range ids {
		out[j] = g.node(i).CanonText()
	}
	return out
}

// fail reports err at the call and aborts.
func (g *graph) fail(b *diag.Builder) (value.Value, bool) {
	g.h.Fail(b)
	return nil, false
}
