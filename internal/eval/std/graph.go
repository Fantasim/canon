package std

import (
	"github.com/fantasim/canonlang/internal/diag"
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

func newGraph(h Host, c *Call, next value.Value) *graph {
	return &graph{h: h, c: c, next: next, nodes: newSet(h, elemType(c.Result)), succ: map[int][]int{}}
}

// id numbers a node, the same number for equal nodes (TYPES.md §7.5); false: the root aborted.
func (g *graph) id(v value.Value) (int, bool) {
	i, _, ok := g.nodes.add(v)
	return i, ok
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
	ids := make([]int, len(out))
	for j, s := range out {
		if ids[j], ok = g.id(s); !ok {
			return nil, false
		}
	}
	g.succ[i] = ids
	return ids, true
}

func isNone(v value.Value) bool {
	_, ok := v.(*value.None)
	return ok
}

// visit is a node being explored on an explicit stack: its successors and the next one to take.
type visit struct {
	node, next int
	succ       []int
}

// preorder is a depth-first walk from one node, on an explicit stack (DECISIONS 195).
type preorder struct {
	*graph
	listed map[int]bool
	out    []value.Value
	stack  []visit
}

// graphReachable lists every node reachable from `from`, depth-first preorder; one step per
// node listed and per successor examined.
func graphReachable(h Host, c *Call) (value.Value, bool) {
	w := &preorder{graph: newGraph(h, c, c.arg(1)), listed: map[int]bool{}}
	from, ok := w.id(c.arg(0))
	if !ok || !w.open(from) {
		return nil, false
	}
	for len(w.stack) > 0 {
		top := &w.stack[len(w.stack)-1]
		if top.next == len(top.succ) {
			w.stack = w.stack[:len(w.stack)-1]
			continue
		}
		s := top.succ[top.next]
		top.next++
		if !w.listed[s] && !w.open(s) {
			return nil, false
		}
	}
	return c.list(w.out), true
}

// open lists node i, charges it, then takes its successors onto the stack.
func (w *preorder) open(i int) bool {
	w.listed[i] = true
	w.out = append(w.out, w.node(i))
	if !w.h.Charge(1) {
		return false
	}
	succ, ok := w.successors(i)
	w.stack = append(w.stack, visit{node: i, succ: succ})
	return ok
}

// seqNodes numbers the distinct elements of xs, in order; false: the root aborted.
func (g *graph) seqNodes(xs value.Value) ([]int, bool) {
	var order []int
	seen := map[int]bool{}
	for _, x := range Elems(xs) {
		i, ok := g.id(x)
		if !ok {
			return nil, false
		}
		if !seen[i] {
			seen[i] = true
			order = append(order, i)
		}
	}
	return order, true
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
