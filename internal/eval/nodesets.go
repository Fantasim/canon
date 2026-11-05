package eval

import "github.com/fantasim/canonlang/internal/value"

// nodeSet is what a root's value holds: its composites and its records' instances, taken once per
// evaluator and shared by every let reading that value (copies.go).
type nodeSet struct {
	nodes map[value.Value]bool
	insts map[*value.Record]bool
}

// nodesOf is v's node set, a root value a let read, walked once.
func (e *Evaluator) nodesOf(v value.Value) *nodeSet {
	if s, ok := e.nodeSets[v]; ok {
		return s
	}
	s := &nodeSet{nodes: map[value.Value]bool{}, insts: map[*value.Record]bool{}}
	s.add(e, v)
	if e.nodeSets == nil {
		e.nodeSets = map[value.Value]*nodeSet{}
	}
	e.nodeSets[v] = s
	return s
}

// add notes v and every composite it holds, once each.
func (s *nodeSet) add(e *Evaluator, v value.Value) {
	if !isComposite(v) || s.nodes[v] {
		return
	}
	s.nodes[v] = true
	if rec, ok := v.(*value.Record); ok {
		s.insts[e.instance(rec)] = true
	}
	indexed(v, func(_ int, k value.Value) { s.add(e, k) })
}
