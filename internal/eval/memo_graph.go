package eval

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// memoGraph is an entry's record as the memo keeps it, a copy no evaluator holds: its nodes
// (with the values its marks name), the marks the evaluator kept for them, and a stand-in for
// each node of a value it read. Each replay copies it again (colls.go sets identities in place).
type memoGraph struct {
	root    *value.Record
	nodes   []value.Value
	marks   memoMarks
	foreign []foreignAt
	size    int
}

// foreignAt stands for the node at position pos of the fingerprint walk of the value the entry
// read i-th: a replay puts there the node of the value it forced, as evaluating would share it.
type foreignAt struct {
	stand     value.Value
	read, pos int
}

// memoMarks are the evaluator's marks on a graph's nodes (EVALUATION.md §7.3, §9.3, TYPES.md §11).
type memoMarks struct {
	invalid, written []value.Value
	history, rebuilt []memoLink
	origin           []memoLink
	bound            []memoBound
}

// memoLink is a mark naming another value: the one amended, rebuilt or copied.
type memoLink struct {
	from, to value.Value
}

// memoBound is an applied record instance's arguments (TYPES.md §11.1).
type memoBound struct {
	rec    value.Value
	params map[*types.Param]value.Value
}

// freezer walks an entry's record with the values its marks name, on an explicit stack.
type freezer struct {
	e       *Evaluator
	reads   []*readInfo
	seen    map[value.Value]bool
	stack   []value.Value
	graph   memoGraph
	foreign map[value.Value]foreignAt
	ok      bool
}

// freeze is rec kept apart with its marks, the nodes it shares with the values it read left as
// stand-ins; false when it holds a function value.
func (e *Evaluator) freeze(rec *value.Record, reads []*readInfo) (memoGraph, bool) {
	f := &freezer{e: e, reads: reads, seen: map[value.Value]bool{}, stack: []value.Value{rec}, foreign: map[value.Value]foreignAt{}, ok: true}
	for len(f.stack) > 0 && f.ok {
		n := f.stack[len(f.stack)-1]
		f.stack = f.stack[:len(f.stack)-1]
		f.visit(n)
	}
	if !f.ok {
		return memoGraph{}, false
	}
	return f.kept(rec), true
}

// visit takes one node: a stand-in for a node read, else its marks, then its parts.
func (f *freezer) visit(n value.Value) {
	if n == nil || f.seen[n] {
		return
	}
	f.seen[n] = true
	if _, fn := n.(*closure); fn {
		f.ok = false
		return
	}
	for i, in := range f.reads {
		if pos, read := in.position(n); read {
			f.foreign[n] = foreignAt{stand: standIn(n), read: i, pos: pos}
			return
		}
	}
	f.graph.nodes = append(f.graph.nodes, n)
	f.mark(n)
	f.stack = append(f.stack, parts(n)...)
}

// mark keeps n's marks and walks the values they name.
func (f *freezer) mark(n value.Value) {
	e, m := f.e, &f.graph.marks
	if e.invalid[n] {
		m.invalid = append(m.invalid, n)
	}
	if e.written[n] {
		m.written = append(m.written, n)
	}
	if old, ok := e.history[n]; ok {
		m.history = append(m.history, memoLink{from: n, to: old})
		f.stack = append(f.stack, old)
	}
	if from, ok := e.rebuilt[n]; ok {
		m.rebuilt = append(m.rebuilt, memoLink{from: n, to: from})
		f.stack = append(f.stack, from)
	}
	rec, ok := n.(*value.Record)
	if !ok {
		return
	}
	if of, copied := e.origin[rec]; copied {
		m.origin = append(m.origin, memoLink{from: rec, to: of})
		f.stack = append(f.stack, of)
	}
	if params, bound := e.bound[rec]; bound {
		m.bound = append(m.bound, memoBound{rec: rec, params: params})
		for _, arg := range params { //canon:unordered walked into a set
			f.stack = append(f.stack, arg)
		}
	}
}

// kept is the graph walked, copied apart: its nodes new, each node read a stand-in.
func (f *freezer) kept(root *value.Record) memoGraph {
	done := make(map[value.Value]value.Value, len(f.graph.nodes)+len(f.foreign))
	for n, fa := range f.foreign { //canon:unordered filling a map
		done[n] = fa.stand
	}
	copyNodes(f.graph.nodes, done)
	out := memoGraph{root: getRecord(done, root), marks: f.graph.marks.moved(done), size: len(f.seen)}
	for _, n := range f.graph.nodes {
		if nv, ok := done[n]; ok && shell(n) != nil {
			out.nodes = append(out.nodes, nv)
		}
	}
	for _, fa := range f.foreign { //canon:unordered a replay seeds a map with them
		out.foreign = append(out.foreign, fa)
	}
	return out
}

// parts are the values a node holds or names: its components and the instances owning it.
func parts(n value.Value) []value.Value {
	out := components(n)
	switch x := n.(type) {
	case *value.Record:
		if x.Ident != nil && x.Ident.Owner != nil {
			out = append(out, x.Ident.Owner)
		}
	case *value.Ref:
		if x.Owner != nil {
			out = append(out, x.Owner)
		}
	}
	return out
}

// moved is m on the copies done made.
func (m memoMarks) moved(done map[value.Value]value.Value) memoMarks {
	out := memoMarks{
		invalid: getAll(done, m.invalid), written: getAll(done, m.written),
		history: movedLinks(done, m.history), rebuilt: movedLinks(done, m.rebuilt), origin: movedLinks(done, m.origin),
	}
	for _, b := range m.bound {
		out.bound = append(out.bound, memoBound{rec: get(done, b.rec), params: movedParams(done, b.params)})
	}
	return out
}

// movedParams is params on the copies done made.
func movedParams(done map[value.Value]value.Value, params map[*types.Param]value.Value) map[*types.Param]value.Value {
	out := make(map[*types.Param]value.Value, len(params))
	for p, arg := range params { //canon:unordered copied into a map
		out[p] = get(done, arg)
	}
	return out
}

func movedLinks(done map[value.Value]value.Value, links []memoLink) []memoLink {
	out := make([]memoLink, len(links))
	for i, l := range links {
		out[i] = memoLink{from: get(done, l.from), to: get(done, l.to)}
	}
	return out
}
