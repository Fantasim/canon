package std

import (
	"container/heap"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/value"
)

// tarjan finds the strongly connected components of the nodes reachable from xs, on an
// explicit stack of visits (DECISIONS 195).
type tarjan struct {
	*graph
	index, low map[int]int
	onStack    map[int]bool
	stack      []int
	cyclic     map[int]bool
	visits     []visit
}

// graphCycles lists the elements of xs on a cycle (a component of more than one node, or a
// self-loop), in xs order; one step per node explored and per successor examined.
func graphCycles(h Host, c *Call) (value.Value, bool) {
	t := &tarjan{
		graph: newGraph(h, c, c.arg(1)), index: map[int]int{}, low: map[int]int{},
		onStack: map[int]bool{}, cyclic: map[int]bool{},
	}
	order, ok := t.seqNodes(c.arg(0))
	if !ok {
		return nil, false
	}
	for _, i := range order {
		if _, seen := t.index[i]; !seen && !t.strong(i) {
			return nil, false
		}
	}
	var out []int
	for _, i := range order {
		if t.cyclic[i] {
			out = append(out, i)
		}
	}
	return c.list(t.nodeValues(out)), true
}

// strong is Tarjan's visit from node root.
func (t *tarjan) strong(root int) bool {
	if !t.open(root) {
		return false
	}
	for len(t.visits) > 0 {
		top := &t.visits[len(t.visits)-1]
		if top.next == len(top.succ) {
			t.finish()
			continue
		}
		v, w := top.node, top.succ[top.next]
		top.next++
		if !t.edge(v, w) {
			return false
		}
	}
	return true
}

// open numbers node v, charges it and takes its successors onto the visit stack.
func (t *tarjan) open(v int) bool {
	t.index[v], t.low[v] = len(t.index), len(t.index)
	t.stack, t.onStack[v] = append(t.stack, v), true
	if !t.h.Charge(1) {
		return false
	}
	succ, ok := t.successors(v)
	t.visits = append(t.visits, visit{node: v, succ: succ})
	return ok
}

// edge follows v -> w: a self-loop makes v cyclic; an unseen w is opened.
func (t *tarjan) edge(v, w int) bool {
	if w == v {
		t.cyclic[v] = true
	}
	if _, seen := t.index[w]; !seen {
		return t.open(w)
	}
	if t.onStack[w] {
		t.low[v] = min(t.low[v], t.index[w])
	}
	return true
}

// finish closes the top visit: its component is popped at its root, and its caller's low
// takes its own.
func (t *tarjan) finish() {
	v := t.visits[len(t.visits)-1].node
	t.visits = t.visits[:len(t.visits)-1]
	if t.low[v] == t.index[v] {
		t.pop(v)
	}
	if len(t.visits) > 0 {
		p := t.visits[len(t.visits)-1].node
		t.low[p] = min(t.low[p], t.low[v])
	}
}

// pop removes v's component from the stack; a component of several nodes is cyclic.
func (t *tarjan) pop(v int) {
	var comp []int
	for {
		w := t.stack[len(t.stack)-1]
		t.stack = t.stack[:len(t.stack)-1]
		t.onStack[w] = false
		comp = append(comp, w)
		if w == v {
			break
		}
	}
	if len(comp) > 1 {
		for _, w := range comp {
			t.cyclic[w] = true
		}
	}
}

// graphTopoSort places each distinct element of xs after the elements of xs it points to,
// the first ready one in xs order at each step (Kahn); E4501 names one cycle.
func graphTopoSort(h Host, c *Call) (value.Value, bool) {
	g := newGraph(h, c, c.arg(1))
	order, ok := g.seqNodes(c.arg(0))
	if !ok {
		return nil, false
	}
	inXs := map[int]bool{}
	for _, i := range order {
		inXs[i] = true
	}
	deps := map[int][]int{}
	for _, i := range order {
		if !h.Charge(1) {
			return nil, false
		}
		succ, ok := g.successors(i)
		if !ok {
			return nil, false
		}
		for _, s := range succ {
			if inXs[s] {
				deps[i] = append(deps[i], s)
			}
		}
	}
	placed, out := kahn(order, deps)
	if len(out) < len(order) {
		return g.fail(diag.E4501.At(h.Site(), g.chain(loopFrom(order, deps, placed))))
	}
	return c.list(g.nodeValues(out)), true
}

// kahn orders the nodes, the first ready one in xs order each time (STDLIB.md §2.3).
func kahn(order []int, deps map[int][]int) (map[int]bool, []int) {
	pos := map[int]int{}
	for p, i := range order {
		pos[i] = p
	}
	waiting, dependents := map[int]int{}, map[int][]int{}
	ready := &positions{pos: pos}
	for _, i := range order {
		for _, d := range deps[i] {
			waiting[i]++
			dependents[d] = append(dependents[d], i)
		}
		if waiting[i] == 0 {
			heap.Push(ready, i)
		}
	}
	placed, out := map[int]bool{}, []int(nil)
	for ready.Len() > 0 {
		i, _ := heap.Pop(ready).(int)
		placed[i] = true
		out = append(out, i)
		for _, d := range dependents[i] {
			if waiting[d]--; waiting[d] == 0 {
				heap.Push(ready, d)
			}
		}
	}
	return placed, out
}

// positions is a min-heap of nodes by their position in xs.
type positions struct {
	pos   map[int]int
	nodes []int
}

func (p *positions) Len() int           { return len(p.nodes) }
func (p *positions) Less(i, j int) bool { return p.pos[p.nodes[i]] < p.pos[p.nodes[j]] }
func (p *positions) Swap(i, j int)      { p.nodes[i], p.nodes[j] = p.nodes[j], p.nodes[i] }
func (p *positions) Push(x any)         { n, _ := x.(int); p.nodes = append(p.nodes, n) }

func (p *positions) Pop() any {
	n := p.nodes[len(p.nodes)-1]
	p.nodes = p.nodes[:len(p.nodes)-1]
	return n
}

// loopFrom follows, from the first unplaced node, the first unplaced successor until a node
// repeats, and returns that loop, its first node written again at the end.
func loopFrom(order []int, deps map[int][]int, placed map[int]bool) []int {
	var path []int
	at := map[int]int{}
	for _, i := range order {
		if !placed[i] {
			path = append(path, i)
			break
		}
	}
	for {
		cur := path[len(path)-1]
		at[cur] = len(path) - 1
		next := -1
		for _, d := range deps[cur] {
			if !placed[d] {
				next = d
				break
			}
		}
		if next < 0 {
			return path
		}
		if start, seen := at[next]; seen {
			return append(path[start:], next)
		}
		path = append(path, next)
	}
}
