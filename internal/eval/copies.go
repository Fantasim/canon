package eval

import (
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/value"
)

// copies are a let's copy facts, from values and origins alone so warm, cold and child evaluators
// agree (DECISIONS 79, 486): the instances its base value holds twice, the visit of each one's first
// place in walk order, the child visit at each place, and the node sets of the values it read.
type copies struct {
	e      *Evaluator
	shared map[*value.Record]bool
	first  map[*value.Record]int
	visits map[place]int
	reads  []*nodeSet
}

// place is a child of a visit of the base walk: the visit and the child's index (indexed).
type place struct {
	visit, index int
}

// copiesOf are the copy facts of base, a let's value as evaluated, which read the values reads;
// a part another root's value holds is not walked: what it holds is that root's.
func (e *Evaluator) copiesOf(base value.Value, reads []value.Value) *copies {
	c := &copies{e: e, shared: map[*value.Record]bool{}, first: map[*value.Record]int{}, visits: map[place]int{}}
	for _, v := range reads {
		c.read(v)
	}
	w := &copyWalk{c: c, seen: map[value.Value]bool{}, marked: map[value.Value]bool{}}
	if isComposite(base) && !c.readNode(base) {
		w.visit(base, rootVisit)
	}
	return c
}

// read adds v, a value the let read, to the facts: what it holds is another root's.
func (c *copies) read(v value.Value) {
	if isComposite(v) {
		c.reads = append(c.reads, c.e.nodesOf(v))
	}
}

// copyWalk walks a base value, numbering its visits, and a part it meets again once more.
type copyWalk struct {
	c      *copies
	seen   map[value.Value]bool
	marked map[value.Value]bool
	next   int
}

// visit notes v, met at visit id, and its children, but none a value the let read holds; a
// composite met again is held twice, all it holds too.
func (w *copyWalk) visit(v value.Value, id int) {
	if w.seen[v] {
		w.mark(v)
		return
	}
	w.seen[v] = true
	if rec, ok := v.(*value.Record); ok {
		inst := w.c.e.instance(rec)
		if _, met := w.c.first[inst]; met {
			w.c.shared[inst] = true
		} else {
			w.c.first[inst] = id
		}
	}
	indexed(v, func(i int, k value.Value) {
		if isComposite(k) && !w.c.readNode(k) {
			w.next++
			w.c.visits[place{visit: id, index: i}] = w.next
			w.visit(k, w.next)
		}
	})
}

// mark notes every instance v holds as held twice.
func (w *copyWalk) mark(v value.Value) {
	if !isComposite(v) || w.marked[v] || w.c.readNode(v) {
		return
	}
	w.marked[v] = true
	if rec, ok := v.(*value.Record); ok {
		w.c.shared[w.c.e.instance(rec)] = true
	}
	indexed(v, func(_ int, k value.Value) { w.mark(k) })
}

// indexed calls fn with each value v holds and its index: a record's field, an element, a map's
// key and value side by side (a key added moves none), a pair's sides; amendments count the same.
func indexed(v value.Value, fn func(int, value.Value)) {
	switch x := v.(type) {
	case *value.Record:
		for i, f := range x.Fields {
			if f != nil {
				fn(i, f)
			}
		}
	case *value.Map:
		for i, k := range x.Keys {
			fn(mapStride*i, k)
			fn(mapStride*i+1, x.Vals[i])
		}
	case *value.Pair:
		fn(0, x.A)
		fn(1, x.B)
	default:
		for i, el := range std.Elems(v) {
			fn(i, el)
		}
	}
}

// child is the visit of the child index of visit, noVisit when the walk numbered none there: a
// place below a part met again, or another root's.
func (c *copies) child(visit int, index int) int {
	if c == nil || visit == noVisit || index < 0 {
		return noVisit
	}
	if id, ok := c.visits[place{visit: visit, index: index}]; ok {
		return id
	}
	return noVisit
}

// readNode reports v a composite a value the let read holds.
func (c *copies) readNode(v value.Value) bool {
	for _, s := range c.reads {
		if s.nodes[v] {
			return true
		}
	}
	return false
}

// foreign reports inst an instance a value the let read holds: another root's.
func (c *copies) foreign(inst *value.Record) bool {
	for _, s := range c.reads {
		if s.insts[inst] {
			return true
		}
	}
	return false
}

// none reports facts with no instance held twice or read: every entry is the let's own.
func (c *copies) none() bool {
	if len(c.shared) > 0 {
		return false
	}
	for _, s := range c.reads {
		if len(s.insts) > 0 {
			return false
		}
	}
	return true
}

// shared reports rec, a base record or an amendment's copy of one, whose instance another place
// holds: the let's value elsewhere, or a value it read; never outside an amendment.
func (e *Evaluator) shared(rec *value.Record, mv *moves) bool {
	if mv == nil || mv.copies == nil {
		return false
	}
	inst := e.instance(mv.baseOf(rec))
	return mv.copies.shared[inst] || mv.copies.foreign(inst)
}

// isCopy reports rec, met at visit, a copy: its instance is another root's, or held first at
// another place of the let. Its source supplied its fields (DECISIONS 486).
func (e *Evaluator) isCopy(rec *value.Record, visit int, mv *moves) bool {
	if mv == nil || mv.copies == nil {
		return false
	}
	inst := e.instance(mv.baseOf(rec))
	if mv.copies.foreign(inst) {
		return true
	}
	first, met := mv.copies.first[inst]
	return mv.copies.shared[inst] && (!met || first != visit)
}

// noteRead keeps v, the value of st this run read, for its amendments' copy facts, at once when
// they are taken (a right-hand side's read); under a layer only.
func (r *run) noteRead(st *rootState, v value.Value) {
	if len(r.ev.opt.Layers) == 0 || v == nil || r.readFrom[st] {
		return
	}
	if r.readFrom == nil {
		r.readFrom = map[*rootState]bool{}
	}
	r.readFrom[st] = true
	r.reads = append(r.reads, v)
	if r.mv != nil && r.mv.copies != nil {
		r.mv.copies.read(v)
	}
}
