package verify

import (
	"slices"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// entryAt verifies e, an entry at p of the table at at, through the memo when the table is a
// top-level value.
func (w *walker) entryAt(at *Path, e *value.Record, t types.Type, p *Path, sc scope) value.Value {
	if at == nil || at.parent != nil {
		return w.walk(e, t, p, sc)
	}
	return w.entry(e, t, p, sc)
}

// entry verifies e, an entry of a top-level table, at t: replayed when the memo keeps its
// verification and what that read is unchanged, else walked, and kept when a replay can be.
func (w *walker) entry(e *value.Record, t types.Type, at *Path, sc scope) value.Value {
	key, ok := w.entryKey(e, t, sc)
	if !ok {
		return w.walk(e, t, at, sc)
	}
	if kept := w.memo.lookup(key); kept != nil && w.replayEntry(e, kept) {
		w.memo.hit()
		return e
	}
	return w.recordEntry(key, e, t, at, sc)
}

// entryKey is the memo key of e verified at t in sc, for an entry of a top-level table only.
func (w *walker) entryKey(e *value.Record, t types.Type, sc scope) (entryKey, bool) {
	if w.memo == nil || w.rec != nil || len(w.recording) > 0 || sc.env != nil || sc.dep != nil || w.stopped() {
		return entryKey{}, false
	}
	token, ok := w.memo.tokens.EntryToken(e)
	if !ok {
		return entryKey{}, false
	}
	return entryKey{token: token, root: w.root, elem: t, retired: sc.retired}, true
}

// recordEntry walks e as walk does, keeping what it read and reported when a replay can.
func (w *walker) recordEntry(key entryKey, e *value.Record, t types.Type, at *Path, sc scope) value.Value {
	rec := &entryRec{}
	charged, unbound := w.charged, len(w.res.Unbound)
	w.rec = rec
	nv := w.walk(e, t, at, sc)
	w.rec = nil
	if rec.void || nv != e || w.stopped() || w.charged != charged || len(w.res.Unbound) != unbound {
		return nv
	}
	marks, ok := positions(e, rec.marked)
	if ok {
		w.memo.store(key, &entryKept{reads: rec.reads, found: rec.found, marks: marks})
	}
	return nv
}

// replayEntry asks again what e's kept verification read, in order; when every answer is the
// same, it reports what the verification found and marks what it marked. False: a read differs,
// and nothing but those reads was done.
func (w *walker) replayEntry(e *value.Record, kept *entryKept) bool {
	for _, rd := range kept.reads {
		if !w.sameRead(rd) {
			return false
		}
	}
	var nodes []value.Value
	if len(kept.marks) > 0 {
		nodes = nodeOrder(e)
	}
	for _, i := range kept.marks {
		if i >= len(nodes) {
			return false // a token's graphs are one shape: never
		}
	}
	for _, b := range kept.found {
		b.Report(w.bag)
	}
	for _, i := range kept.marks {
		w.invalid(nodes[i])
	}
	return true
}

// sameRead reports that asking rd again gives what it gave.
func (w *walker) sameRead(rd entryRead) bool {
	if rd.coll == nil {
		display, found := w.findAsset(rd.asset, rd.name)
		return display == rd.display && found == rd.found
	}
	how, target := w.target(rd.coll, rd.key)
	return how == rd.how && (target != nil) == rd.found && (target != nil && target.Ident.Retired) == rd.retired
}

// target forces a top-level collection and looks key up in it.
func (w *walker) target(c *types.Collection, key value.Key) (reach, *value.Record) {
	v, ok := w.ev.Force(w.ctx, eval.Root{Pkg: c.Pkg, Name: c.Name})
	if !ok {
		return poisoned, nil
	}
	return reached, w.index(follow(v, c.FieldPath))[key]
}

// noteRef records a ref's target looked up; one owned by an instance (TYPES.md §10.2) voids it.
func (w *walker) noteRef(c *types.Collection, key value.Key, how reach, target *value.Record) {
	if w.rec == nil {
		return
	}
	if c.Kind == types.CollField {
		w.rec.void = true
		return
	}
	rd := entryRead{coll: c, key: key, how: how, found: target != nil, retired: target != nil && target.Ident.Retired}
	w.rec.reads = append(w.rec.reads, rd)
}

// noteAsset records an asset looked up.
func (w *walker) noteAsset(a *types.AssetSpec, name, display string, found bool) {
	if w.rec != nil {
		w.rec.reads = append(w.rec.reads, entryRead{asset: a, name: name, display: display, found: found})
	}
}

// positions is the place of each node of marked in e's nodeOrder; false when one is not there.
func positions(e *value.Record, marked []value.Value) ([]int, bool) {
	if len(marked) == 0 {
		return nil, true
	}
	at := map[value.Value]int{}
	for i, n := range nodeOrder(e) {
		at[n] = i
	}
	out := make([]int, len(marked))
	for i, n := range marked {
		pos, ok := at[n]
		if !ok {
			return nil, false
		}
		out[i] = pos
	}
	return out, true
}

// nodeOrder is v's nodes, refs not followed, each at its first reach depth-first: one order for
// every copy of one graph.
func nodeOrder(v value.Value) []value.Value {
	var out []value.Value
	seen := map[value.Value]bool{}
	stack := []value.Value{v}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == nil || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
		for _, kid := range slices.Backward(children(n)) {
			stack = append(stack, kid)
		}
	}
	return out
}

// children is what a node holds, in order: fields, elements, keys then values, entries, halves.
func children(n value.Value) []value.Value {
	switch x := n.(type) {
	case *value.Record:
		return x.Fields
	case *value.List:
		return x.Elems
	case *value.Map:
		return append(append([]value.Value(nil), x.Keys...), x.Vals...)
	case *value.Table:
		out := make([]value.Value, 0, len(x.Entries))
		for _, e := range x.Entries {
			if e != nil {
				out = append(out, e)
			}
		}
		return out
	case *value.Pair:
		return []value.Value{x.A, x.B}
	}
	return nil
}
