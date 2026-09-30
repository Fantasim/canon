package eval

import (
	"reflect"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// memoGraph is an entry's record as the memo keeps it, a copy no evaluator holds: its nodes'
// templates and parts' slots, the values it shares, where the nodes of the values it read sit, and
// their marks. Each replay copies it again (colls.go sets identities in place; memo_copy.go).
type memoGraph struct {
	root    slot // the entry's record or the load's value; nilSlot for a failed entry
	nodes   []memoNode
	parts   []slot
	shared  []value.Value
	foreign []foreignAt
	marks   memoMarks
	counts  nodeCounts
	size    int
}

// slot is where a copy finds a value: in its table, nilSlot, then the graph's nodes, then the
// nodes of the values read (foreign); below zero, the value shared as it is at ^slot.
type slot int

// nodeKind is the kind of a node a graph copies.
type nodeKind uint8

// memoNode is a node the graph copies: its kind, what each copy takes as it is, its identity's or
// ref's owner, and its parts at lo in the graph's parts: n of them, then m more (a map's values).
type memoNode struct {
	kind  nodeKind
	t     types.Type
	p     *value.Prov
	set   []bool          // a record's written fields
	ident *value.Identity // a record's identity, less its owner
	key   value.Key       // a ref's key
	owner slot
	lo    int
	n, m  int
}

// nodeCounts are how many nodes of each kind, identities, parts, table entries and set flags a
// copy of a graph allocates.
type nodeCounts struct {
	kinds                        [kindCount]int
	idents, vals, entries, bools int
}

// foreignAt stands for the node, of type typ, at position pos of the fingerprint walk of the value
// the entry read i-th: a replay puts there the node of the value it forced, as evaluating would
// share it.
type foreignAt struct {
	typ       reflect.Type
	read, pos int
}

// memoMarks are the evaluator's marks on a graph's nodes (EVALUATION.md §7.3, §9.3, TYPES.md §11).
type memoMarks struct {
	invalid, written []slot
	history, rebuilt []memoLink
	origin           []memoLink
	bound            []memoBound
}

// memoLink is a mark naming another value: the one amended, rebuilt or copied.
type memoLink struct {
	from, to slot
}

// memoBound is an applied record instance's arguments (TYPES.md §11.1).
type memoBound struct {
	rec    slot
	params []*types.Param
	args   []slot
}

// freezer walks an entry's record with the values its marks name, on an explicit stack.
type freezer struct {
	e       *Evaluator
	reads   []*readInfo
	seen    map[value.Value]bool
	stack   []value.Value
	nodes   []value.Value
	marks   frozenMarks
	foreign map[value.Value]foreignAt
	ok      bool
}

// frozenMarks are the marks a freezer met, on the nodes it walked.
type frozenMarks struct {
	invalid, written []value.Value
	history, rebuilt []frozenLink
	origin           []frozenLink
	bound            []*value.Record
}

// frozenLink is a mark a freezer met: its node, and the value it names.
type frozenLink struct {
	from, to value.Value
}

// newFreezer is a freezer about to walk v.
func (e *Evaluator) newFreezer(v value.Value, reads []*readInfo) *freezer {
	return &freezer{e: e, reads: reads, seen: map[value.Value]bool{}, stack: []value.Value{v}, foreign: map[value.Value]foreignAt{}, ok: true}
}

// walk visits every node from the stack, false when it met a function value.
func (f *freezer) walk() bool {
	for len(f.stack) > 0 && f.ok {
		n := f.stack[len(f.stack)-1]
		f.stack = f.stack[:len(f.stack)-1]
		f.visit(n)
	}
	return f.ok
}

// freeze is rec kept apart with its marks, the nodes it shares with the values it read left to
// a replay's; false when it holds a function value.
func (e *Evaluator) freeze(rec *value.Record, reads []*readInfo) (memoGraph, bool) {
	f := e.newFreezer(rec, reads)
	if !f.walk() {
		return memoGraph{}, false
	}
	return f.kept(rec), true
}

// visit takes one node: the place of a node read, else its marks, then its parts.
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
			f.foreign[n] = foreignAt{typ: reflect.TypeOf(n), read: i, pos: pos}
			return
		}
	}
	f.nodes = append(f.nodes, n)
	f.mark(n)
	f.stack = append(f.stack, parts(n)...)
}

// mark keeps n's marks and walks the values they name.
func (f *freezer) mark(n value.Value) {
	e, m := f.e, &f.marks
	if e.invalid[n] {
		m.invalid = append(m.invalid, n)
	}
	if e.written[n] {
		m.written = append(m.written, n)
	}
	if old, ok := e.history[n]; ok {
		m.history = append(m.history, frozenLink{from: n, to: old})
		f.stack = append(f.stack, old)
	}
	if from, ok := e.rebuilt[n]; ok {
		m.rebuilt = append(m.rebuilt, frozenLink{from: n, to: from})
		f.stack = append(f.stack, from)
	}
	rec, ok := n.(*value.Record)
	if !ok {
		return
	}
	if of, copied := e.origin[rec]; copied {
		m.origin = append(m.origin, frozenLink{from: rec, to: of})
		f.stack = append(f.stack, of)
	}
	if params, bound := e.bound[rec]; bound {
		m.bound = append(m.bound, rec)
		for _, arg := range params { //canon:unordered walked into a set
			f.stack = append(f.stack, arg)
		}
	}
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
