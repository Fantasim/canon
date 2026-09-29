package eval

import (
	"maps"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// deref is the entry a ref names: E3501 or E3505 when none, both hard (EVALUATION.md §7.1).
func (r *run) deref(v value.Value, at syntax.Node) value.Value {
	return r.derefAt(v, at, at)
}

// derefAt is deref forcing the target at at, E3501 and E3505 at loc, or at the ref itself for nil.
func (r *run) derefAt(v value.Value, at, loc syntax.Node) value.Value {
	ref, ok := v.(*value.Ref)
	if !ok {
		return v
	}
	rt, ok := ref.T.Base().(*types.RefType)
	if !ok {
		r.bug(at)
		return nil
	}
	coll := r.collValue(ref, rt.Target, at, loc)
	if coll == nil {
		return nil
	}
	e, found := r.ev.entry(coll, ref.Key)
	if !found {
		r.refFail(diag.E3501.At(r.refSpan(ref, loc), ref, r.collName(rt.Target)), ref, loc)
		return nil
	}
	return r.read(e)
}

// refSpan locates a dereference: the expression at, else the ref's own provenance (EVALUATION.md §13).
func (r *run) refSpan(ref *value.Ref, at syntax.Node) source.Span {
	if at != nil {
		return r.span(at)
	}
	return located(ref, source.Span{})
}

// refFail reports a hard dereference error; one located at a loaded ref carries its pointer.
func (r *run) refFail(b *diag.Builder, ref *value.Ref, at syntax.Node) {
	if p := origin(ref.P); at == nil && p != nil {
		b.Pointer(p.Pointer)
	}
	r.fail(b)
}

// collValue is the collection a ref's target names, forced at at, E3505 at loc (EVALUATION.md §3.4).
func (r *run) collValue(ref *value.Ref, c *types.Collection, at, loc syntax.Node) value.Value {
	if c.Kind != types.CollField && r.nonConstant() { // a key dereference in a fold reads a let (DECISIONS 210)
		return nil
	}
	var base value.Value
	if c.Kind == types.CollField {
		if ref.Owner == nil {
			r.refFail(diag.E3505.At(r.refSpan(ref, loc), elemName(c), ownerName(c)), ref, loc)
			return nil
		}
		base = r.mv.latest(ref.Owner) // a fresh instance's entries name it until its let settles
	} else {
		st := r.ev.rootState(Root{Pkg: c.Pkg, Name: c.Name})
		if st == nil {
			r.bug(at)
			return nil
		}
		v, ok := r.forceRead(st, at)
		if !ok {
			r.readPoisoned(c.Pkg, c.Name, at)
			return nil
		}
		base = v
	}
	for _, name := range c.FieldPath {
		rec, ok := base.(*value.Record)
		i := -1
		if ok {
			i = fieldIndex(rec.T, name)
		}
		if i < 0 {
			r.bug(at)
			return nil
		}
		if rec.Fields[i] == nil { // its instance is still being decoded (EVALUATION.md §3.2)
			name := r.collName(c)
			r.refFail(diag.E4301.At(r.refSpan(ref, loc), []string{name, name}), ref, loc)
			return nil
		}
		base = rec.Fields[i]
	}
	return base
}

// collName names a collection in E3501: qualified when it is another package's.
func (r *run) collName(c *types.Collection) string {
	name := r.qualified(c.Pkg, c.Name)
	if c.Kind == types.CollField && c.Owner != nil {
		name = r.qualified(c.Owner.Pkg, c.Owner.Name)
	}
	return strings.Join(append([]string{name}, c.FieldPath...), dot)
}

func elemName(c *types.Collection) string {
	if rt, ok := c.Elem.Base().(*types.RecordType); ok {
		return rt.Name
	}
	return c.Elem.String()
}

func ownerName(c *types.Collection) string {
	if c.Owner != nil {
		return c.Owner.Name
	}
	return c.Name
}

// entry is the entry of key k of a table or keyed list, through an index built once per
// collection value when it is large enough to need one.
func (e *Evaluator) entry(coll value.Value, k value.Key) (*value.Record, bool) {
	elems := std.Elems(coll)
	if len(elems) < indexFrom {
		for _, x := range elems {
			if rec, ok := x.(*value.Record); ok && rec.Ident != nil && rec.Ident.Key == k {
				return rec, true
			}
		}
		return nil, false
	}
	idx := e.keyed[coll]
	if idx == nil && e.parent != nil {
		idx = e.parent.keyed[coll] // a parent's value is indexed there once
	}
	if idx == nil {
		idx = keyIndex(elems)
		e.keyed[coll] = idx
	}
	rec, ok := idx[k]
	return rec, ok
}

// keyIndex maps each key to its first entry.
func keyIndex(elems []value.Value) map[value.Key]*value.Record {
	idx := map[value.Key]*value.Record{}
	for _, x := range elems {
		rec, ok := x.(*value.Record)
		if !ok || rec.Ident == nil {
			continue
		}
		if _, dup := idx[rec.Ident.Key]; !dup {
			idx[rec.Ident.Key] = rec
		}
	}
	return idx
}

// Entry is the evaluator's lookup for the standard library (std.Host).
func (h *stdHost) Entry(coll value.Value, k value.Key) (*value.Record, bool) {
	return h.r.ev.entry(coll, k)
}

// bindRefs binds a completed instance's level-1 refs, one binder for all fields (EVALUATION.md §3.4).
func (e *Evaluator) bindRefs(rec *value.Record) {
	if owned := e.owned(rec.T); len(owned) > 0 {
		e.rebindFields(rec, &binder{e: e, owned: owned, rt: recordOf(rec.T), owner: rec})
	}
}

// bindTo binds rec's unbound level-1 refs and those bound to the instance of src, which rec copies (EVALUATION.md §4.2).
func (e *Evaluator) bindTo(rec, src *value.Record, mv *moves) {
	owned := e.owned(rec.T)
	if len(owned) == 0 {
		return
	}
	b := &binder{e: e, owned: owned, rt: recordOf(rec.T), owner: rec, inst: e.instance(src), mode: modeRebind, mv: mv}
	e.rebindFields(rec, b)
}

// unbindFrom unbinds rec's refs to the instances of path, the records it copies and encloses, as when first built (EVALUATION.md §3.4).
func (e *Evaluator) unbindFrom(rec *value.Record, path []*value.Record, mv *moves) {
	b := &binder{e: e, owned: map[*types.Collection]bool{}, owner: rec, mode: modeUnbind, owners: map[*value.Record]bool{}, mv: mv}
	for _, in := range path {
		maps.Copy(b.owned, e.owned(in.T))
		b.owners[e.instance(in)] = true
	}
	if len(b.owned) > 0 {
		e.rebindFields(rec, b)
	}
}

// rebindFields rebinds rec's fields in place through b.
func (e *Evaluator) rebindFields(rec *value.Record, b *binder) {
	b.done = map[value.Value]value.Value{}
	for i, f := range rec.Fields {
		rec.Fields[i] = b.bind(f)
	}
}

// cleanKey is a node and the record type whose owned collections it holds no unbound ref
// into; values are immutable, so a node found clean stays clean (DECISIONS 199).
type cleanKey struct {
	node value.Value
	rt   *types.RecordType
}

// binder binds refs into owned to owner, copying what changes (DECISIONS 199); rebinding, it
// also moves to owner the refs bound to another copy of its instance inst; unbinding, it
// unbinds the refs bound to one of the instances owners.
type binder struct {
	e      *Evaluator
	owned  map[*types.Collection]bool
	rt     *types.RecordType
	owner  *value.Record
	inst   *value.Record
	mode   bindMode
	owners map[*value.Record]bool
	mv     *moves
	done   map[value.Value]value.Value
}

// binding is a composite being rebound: its components, and the next one to take.
type binding struct {
	v    value.Value
	kids []value.Value
	next int
}

// bind is v rebound, walked post-order on an explicit stack; a subtree the evaluator already
// found clean for this record type is not walked again.
func (b *binder) bind(v value.Value) value.Value {
	if !isComposite(v) {
		return b.leaf(v)
	}
	if !b.pending(v) {
		return b.done[v]
	}
	stack := []binding{{v: v, kids: components(v)}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if top.next < len(top.kids) {
			k := top.kids[top.next]
			top.next++
			if isComposite(k) && b.pending(k) {
				stack = append(stack, binding{v: k, kids: components(k)})
			}
			continue
		}
		nv := b.rebuild(top.v)
		b.done[top.v] = nv
		if b.mode != modeUnbind { // an unbinding leaves unbound refs: nothing it builds is clean
			b.e.clean[cleanKey{node: nv, rt: b.rt}] = true
		}
		stack = stack[:len(stack)-1]
	}
	return b.done[v]
}

// pending reports a composite still to walk: neither rebound in this call nor known clean,
// which counts as rebound to itself; a clean node may still hold refs to another copy.
func (b *binder) pending(v value.Value) bool {
	if _, seen := b.done[v]; seen {
		return false
	}
	if b.mode == modeBind && b.e.clean[cleanKey{node: v, rt: b.rt}] || !b.e.mayHoldRef(v.Type(), b.rt) || b.shadowed(v) {
		b.done[v] = v
		return false
	}
	return true
}

// shadowed reports, rebinding, an instance of the owner's type below it: nearer to its refs (TYPES.md §10.2).
func (b *binder) shadowed(v value.Value) bool {
	rec, ok := v.(*value.Record)
	return ok && b.mode == modeRebind && recordOf(rec.T) == b.rt
}

// get is a component rebound: from done for a composite, else at once.
func (b *binder) get(v value.Value) value.Value {
	if isComposite(v) {
		return b.done[v]
	}
	return b.leaf(v)
}

// leaf is a ref into owned bound to owner, or unbound from it; any other value is itself.
func (b *binder) leaf(v value.Value) value.Value {
	x, ok := v.(*value.Ref)
	if !ok {
		return v
	}
	rt, ok := x.T.Base().(*types.RefType)
	if !ok || !b.owned[rt.Target] {
		return v
	}
	if b.mode == modeUnbind {
		if x.Owner == nil || !b.owners[b.e.instance(x.Owner)] {
			return v
		}
		return b.e.carry(v, &value.Ref{T: x.T, Key: x.Key, P: x.P})
	}
	if x.Owner == nil || b.mode == modeRebind && x.Owner != b.owner && b.e.instance(x.Owner) == b.inst {
		return b.e.carry(v, &value.Ref{T: x.T, Key: x.Key, Owner: b.owner, P: x.P})
	}
	return v
}

// rebuild is a composite whose components are all rebound: itself when none changed, else a
// copy, fresh when v is (settle.go); a record copied is the instance it copies (instance.go).
func (b *binder) rebuild(v value.Value) value.Value {
	nv := b.copyOf(v)
	if nv == v {
		return v
	}
	b.mv.follow(v, nv)
	rec, isRec := v.(*value.Record)
	if cp, ok := nv.(*value.Record); ok && isRec {
		b.e.transfer(rec, cp, b.mv)
	}
	return nv
}

// copyOf is v with its components rebound: itself when none changed, else a copy.
func (b *binder) copyOf(v value.Value) value.Value {
	switch x := v.(type) {
	case *value.Record:
		if fields, changed := b.all(x.Fields); changed {
			cp := *x
			cp.Fields = fields
			return b.e.carry(v, &cp)
		}
	case *value.List:
		if elems, changed := b.all(x.Elems); changed {
			return b.e.carry(v, &value.List{T: x.T, Elems: elems, P: x.P})
		}
	case *value.Map:
		keys, kc := b.all(x.Keys)
		vals, vc := b.all(x.Vals)
		if kc || vc {
			return b.e.carry(v, &value.Map{T: x.T, Keys: keys, Vals: vals, P: x.P})
		}
	case *value.Pair:
		if ab, changed := b.all([]value.Value{x.A, x.B}); changed {
			return b.e.carry(v, &value.Pair{T: x.T, A: ab[0], B: ab[1], P: x.P})
		}
	case *value.Table:
		return b.table(x)
	}
	return v
}

// all is vs rebound, copied only when one changed, and whether one did.
func (b *binder) all(vs []value.Value) ([]value.Value, bool) {
	var out []value.Value
	for i, v := range vs {
		nv := v
		if v != nil {
			nv = b.get(v)
		}
		if nv != v && out == nil {
			out = slices.Clone(vs)
		}
		if out != nil {
			out[i] = nv
		}
	}
	if out == nil {
		return vs, false
	}
	return out, true
}

// table is a table whose entries are rebound.
func (b *binder) table(t *value.Table) value.Value {
	entries := make([]*value.Record, len(t.Entries))
	changed := false
	for i, en := range t.Entries {
		rb, ok := b.get(en).(*value.Record)
		if !ok {
			rb = en
		}
		entries[i], changed = rb, changed || rb != en
	}
	if !changed {
		return t
	}
	return b.e.carry(t, &value.Table{T: t.T, Entries: entries, P: t.P})
}

// isComposite reports a value that holds others.
func isComposite(v value.Value) bool {
	switch v.(type) {
	case *value.Record, *value.List, *value.Map, *value.Pair, *value.Table:
		return true
	}
	return false
}

// components are the values a composite holds, nil ones (input fields) left out.
func components(v value.Value) []value.Value {
	var all []value.Value
	switch x := v.(type) {
	case *value.Record:
		all = x.Fields
	case *value.Map:
		all = append(append([]value.Value(nil), x.Keys...), x.Vals...)
	case *value.Pair:
		all = []value.Value{x.A, x.B}
	default:
		all = std.Elems(v)
	}
	out := make([]value.Value, 0, len(all))
	for _, c := range all {
		if c != nil {
			out = append(out, c)
		}
	}
	return out
}
