package eval

import (
	"cmp"
	"hash/maphash"
	"maps"
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// fingerprint is two hashes of a value an entry read, standing for its whole graph: structure,
// types and collections by pointer, provenance, sharing, arguments and instances, which a reader
// may keep and value.Equal ignores.
type fingerprint struct {
	a, b uint64
}

// fpSeeds seed the two hashes; a fingerprint only compares values of one process.
var fpSeeds = [...]maphash.Seed{maphash.MakeSeed(), maphash.MakeSeed()}

// readInfo is what an evaluator learnt of a value its entries read: its fingerprint, taken
// while no identity was set in place (retagged), its nodes, and whether none is marked.
type readInfo struct {
	retagged int
	fp       fingerprint
	ok       bool
	nodes    []value.Value
	at       map[value.Value]int
	marks    int
	checked  bool
	clean    bool
}

// memoGens count, for an evaluator, the written marks ever set and the identities set in place
// (colls.go), which change a value already read: a readInfo is taken again when they grew.
type memoGens struct {
	written, retagged int
}

// info is what u knows of v, taken again once an identity was set in place since.
func (u *memoUse) info(e *Evaluator, v value.Value) *readInfo {
	in := u.reads[v]
	if in == nil || in.retagged != e.gens.retagged {
		in = e.fingerprintOf(v)
		u.reads[v] = in
	}
	return in
}

// cleanIn reports that no node of the value is marked invalid, written or amended in e: reading
// it taints no reader, and a reader sharing one of its scalars shares no mark.
func (in *readInfo) cleanIn(e *Evaluator) bool {
	if g := e.marksGen(); !in.checked || in.marks != g {
		in.clean, in.marks, in.checked = e.unmarked(in.nodes), g, true
	}
	return in.clean
}

// position is n's place among the value's nodes; false when n is not one.
func (in *readInfo) position(n value.Value) (int, bool) {
	if in.at == nil {
		in.at = make(map[value.Value]int, len(in.nodes))
		for i, x := range in.nodes {
			in.at[x] = i
		}
	}
	pos, ok := in.at[n]
	return pos, ok
}

// marksGen grows whenever a value is marked: the invalid, history and rebuilt marks are never
// removed, and every written mark is counted.
func (e *Evaluator) marksGen() int {
	return len(e.invalid) + len(e.history) + len(e.rebuilt) + e.gens.written
}

// unmarked reports that no node carries a mark.
func (e *Evaluator) unmarked(nodes []value.Value) bool {
	for _, n := range nodes {
		_, amended := e.history[n]
		_, copied := e.rebuilt[n]
		if e.invalid[n] || e.written[n] || amended || copied {
			return false
		}
	}
	return true
}

// fpWalk hashes a value graph pre-order on an explicit stack; a node met again is hashed as
// its first position, so the sharing is part of the fingerprint.
type fpWalk struct {
	e     *Evaluator
	h     [len(fpSeeds)]maphash.Hash
	seen  map[value.Value]int
	nodes []value.Value
	stack []value.Value
	ok    bool
}

// fingerprintOf walks v whole.
func (e *Evaluator) fingerprintOf(v value.Value) *readInfo {
	w := &fpWalk{e: e, seen: map[value.Value]int{}, ok: true, stack: []value.Value{v}}
	for i := range w.h {
		w.h[i].SetSeed(fpSeeds[i])
	}
	for len(w.stack) > 0 && w.ok {
		x := w.stack[len(w.stack)-1]
		w.stack = w.stack[:len(w.stack)-1]
		w.node(x)
	}
	return &readInfo{retagged: e.gens.retagged, fp: fingerprint{a: w.h[0].Sum64(), b: w.h[1].Sum64()}, ok: w.ok, nodes: w.nodes}
}

// write hashes x into both hashes.
func write[T comparable](w *fpWalk, x T) {
	for i := range w.h {
		maphash.WriteComparable(&w.h[i], x)
	}
}

// push queues the components of a node, first one popped first.
func (w *fpWalk) push(vs ...value.Value) {
	write(w, len(vs))
	for _, v := range slices.Backward(vs) {
		w.stack = append(w.stack, v)
	}
}

// node hashes one node: nil, a node met before, or a new one with its data.
func (w *fpWalk) node(x value.Value) {
	if x == nil {
		write(w, fpNil)
		return
	}
	if at, met := w.seen[x]; met {
		write(w, fpBack)
		write(w, at)
		return
	}
	w.seen[x] = len(w.nodes)
	w.nodes = append(w.nodes, x)
	write(w, x.Type())
	w.prov(x.Prov())
	if !w.scalar(x) && !w.composite(x) {
		w.ok = false
	}
}

// scalar hashes a value that holds no other, a ref with its owner; false for another kind.
func (w *fpWalk) scalar(x value.Value) bool {
	switch v := x.(type) {
	case *value.Bool:
		write(w, v.V)
	case *value.Int:
		write(w, v.V)
	case *value.Float:
		write(w, math.Float64bits(v.V))
	case *value.Str:
		write(w, v.V)
	case *value.Dur:
		write(w, v.Ms)
	case *value.Member:
		write(w, v.Index)
	case *value.CaseKind:
		write(w, v.Index)
	case *value.Symbol:
		write(w, v.Name)
	case *value.None:
		write(w, fpNone)
	case *value.Range:
		write(w, [...]int64{v.Start, v.End})
		write(w, v.HasEnd)
	case *value.Ref:
		write(w, v.Key)
		w.push(ownerValue(v.Owner))
	default:
		return false
	}
	return true
}

// composite hashes a value holding others, a record with its identity, instance and arguments.
func (w *fpWalk) composite(x value.Value) bool {
	switch v := x.(type) {
	case *value.Record:
		w.record(v)
	case *value.List:
		w.push(v.Elems...)
	case *value.Map:
		w.push(v.Keys...)
		w.push(v.Vals...)
	case *value.Table:
		entries := make([]value.Value, len(v.Entries))
		for i, en := range v.Entries {
			entries[i] = en
		}
		w.push(entries...)
	case *value.Pair:
		w.push(v.A, v.B)
	default:
		return false
	}
	return true
}

// record hashes which fields are set, the identity, then pushes the owner, the instance it
// copies, its arguments by parameter order and its fields.
func (w *fpWalk) record(v *value.Record) {
	write(w, len(v.Set))
	for _, set := range v.Set {
		write(w, set)
	}
	var owner value.Value
	write(w, v.Ident != nil)
	if id := v.Ident; id != nil {
		write(w, id.Coll)
		write(w, id.Key)
		write(w, id.Retired)
		owner = ownerValue(id.Owner)
	}
	params := w.e.boundParams(v)
	order := slices.SortedFunc(maps.Keys(params), func(a, b *types.Param) int { return cmp.Compare(a.Index, b.Index) })
	args := make([]value.Value, len(order))
	for i, p := range order {
		write(w, p)
		args[i] = params[p]
	}
	w.push(owner, ownerValue(w.e.originOf(v)))
	w.push(args...)
	w.push(v.Fields...)
}

// prov hashes a provenance and the ones it goes through.
func (w *fpWalk) prov(p *value.Prov) {
	for ; p != nil; p = p.Via {
		write(w, p.Kind)
		write(w, p.Span)
		write(w, p.Pointer)
		write(w, p.Layer)
		write(w, p.MoreFrames)
		write(w, len(p.Stack))
		for _, f := range p.Stack {
			write(w, f)
		}
	}
	write(w, fpNil)
}

// ownerValue is rec as a value, nil for none (a typed nil is not a nil value).
func ownerValue(rec *value.Record) value.Value {
	if rec == nil {
		return nil
	}
	return rec
}
