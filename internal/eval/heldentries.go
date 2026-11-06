package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// collMove is a collection the entries of one of a record's collection fields name, and the one
// adoption gives them: a decoded list's own collection, or the field's (wire.Decoder).
type collMove struct {
	from, to *types.Collection
}

// heldColls are the collections rec's collection fields' entries name, each with the field's,
// taken before adoption; nil when no other field of rt may hold one of those entries.
func (e *Evaluator) heldColls(rec *value.Record, rt *types.RecordType) []collMove {
	fields := fieldsOf(rec.T)
	if !slices.ContainsFunc(fields, func(f *types.Field) bool { return e.fieldColl(rt, f) == nil && e.mayHoldEntry(f.Type, rt) }) {
		return nil
	}
	var out []collMove
	for i, f := range fields {
		c := e.fieldColl(rt, f)
		if c == nil || i >= len(rec.Fields) || rec.Fields[i] == nil {
			continue
		}
		out = addMove(out, c, c)
		for _, el := range std.Elems(rec.Fields[i]) {
			if en, ok := el.(*value.Record); ok && en.Ident != nil {
				out = addMove(out, en.Ident.Coll, c)
			}
		}
	}
	return out
}

// addMove is moves with from moving to to, once.
func addMove(moves []collMove, from, to *types.Collection) []collMove {
	if slices.ContainsFunc(moves, func(m collMove) bool { return m.from == from }) {
		return moves
	}
	return append(moves, collMove{from: from, to: to})
}

// adoptHeld makes the entries of rec's collection fields that its other fields hold, at any depth, owned as adoptInto made those fields' (DECISIONS 498, 501).
func (e *Evaluator) adoptHeld(rec *value.Record, rt *types.RecordType, holds *ownerFilter, colls []collMove, mv *moves) {
	if colls == nil {
		return
	}
	h := &heldWalk{e: e, rt: rt, rec: rec, holds: holds, colls: colls}
	for i, f := range fieldsOf(rec.T) {
		if i >= len(rec.Fields) || rec.Fields[i] == nil || e.fieldColl(rt, f) != nil || !e.mayHoldEntry(f.Type, rt) {
			continue
		}
		old := rec.Fields[i]
		if rec.Fields[i] = h.in(old); mv.isFresh(old) && rec.Fields[i] != old {
			mv.fresh[rec.Fields[i]] = true
		}
	}
}

// heldWalk re-owns, in values rec's other fields hold, the entries of its collection fields holds
// takes; done is each composite walked, to what it became, so a value held at several places is
// walked and copied once.
type heldWalk struct {
	e     *Evaluator
	rt    *types.RecordType
	rec   *value.Record
	holds *ownerFilter
	colls []collMove
	done  map[value.Value]value.Value
}

// in is v with each entry the walk re-owns owned by its record, what holds one copied, once per node.
func (h *heldWalk) in(v value.Value) value.Value {
	if !isComposite(v) || !h.e.mayHoldEntry(v.Type(), h.rt) {
		return v
	}
	if nv, seen := h.done[v]; seen {
		return nv
	}
	if h.done == nil {
		h.done = map[value.Value]value.Value{}
	}
	nv := h.walk(v)
	h.done[v] = nv
	return nv
}

// walk is in for a node not walked yet. A matched entry's own fields are not walked: built before
// rec, from scopes that cannot read it, they hold none of rec's entries, as adoptOf assumes.
func (h *heldWalk) walk(v value.Value) value.Value {
	if en, ok := v.(*value.Record); ok && en.Ident != nil && en.Ident.Owner != nil && en.Ident.Owner != h.rec {
		if to := h.moved(en.Ident.Coll); to != nil && h.holds.has(en) {
			return h.e.withIdentity(en, &value.Identity{Coll: to, Owner: h.rec, Key: en.Ident.Key, Retired: en.Ident.Retired}, false)
		}
	}
	parts := components(v)
	held := make([]value.Value, len(parts))
	for i, k := range parts {
		held[i] = h.in(k)
	}
	if slices.Equal(parts, held) {
		return v
	}
	return h.e.withParts(v, held)
}

// moved is the collection the walk's adoption gives the entries of c, nil when it adopts none.
func (h *heldWalk) moved(c *types.Collection) *types.Collection {
	for _, m := range h.colls {
		if m.from == c {
			return m.to
		}
	}
	return nil
}

// withParts is v holding parts, its components in components' order, a copy sharing v's instance.
func (e *Evaluator) withParts(v value.Value, parts []value.Value) value.Value {
	var out value.Value
	switch x := v.(type) {
	case *value.Record:
		cp := *x
		cp.Fields = refill(x.Fields, parts)
		e.copiedFrom(x, &cp)
		out = &cp
	case *value.Map:
		n := len(x.Keys)
		out = &value.Map{T: x.T, Keys: parts[:n:n], Vals: parts[n:], P: x.P}
	case *value.Pair:
		out = &value.Pair{T: x.T, A: parts[0], B: parts[1], P: x.P}
	case *value.Table:
		out = &value.Table{T: x.T, Entries: entriesOf(x.Entries, parts), P: x.P}
	case *value.List:
		out = &value.List{T: x.T, Elems: parts, P: x.P}
	default:
		return v
	}
	return e.carry(v, out)
}

// refill is fields with its non-nil values replaced, in order, by parts.
func refill(fields, parts []value.Value) []value.Value {
	out := slices.Clone(fields)
	j := 0
	for i, f := range out {
		if f != nil {
			out[i], j = parts[j], j+1
		}
	}
	return out
}

// entriesOf is a table's entries replaced by parts, an entry kept where its part is no record.
func entriesOf(entries []*value.Record, parts []value.Value) []*value.Record {
	out := slices.Clone(entries)
	for i, p := range parts {
		if rec, ok := p.(*value.Record); ok {
			out[i] = rec
		}
	}
	return out
}

// mayHoldEntry reports whether a value of type t may hold an entry of a collection field of rt, by its record type (TYPES.md §6.3).
func (e *Evaluator) mayHoldEntry(t types.Type, rt *types.RecordType) bool {
	key := refHold{t: t, rt: rt}
	if h, ok := e.entryTypes[key]; ok {
		return h
	}
	elems := map[*types.RecordType]bool{}
	for _, f := range fieldsOf(rt) {
		if c := e.fieldColl(rt, f); c != nil {
			if er := recordOf(c.Elem); er != nil {
				elems[er] = true
			}
		}
	}
	h := len(elems) > 0 && holdsElem(t, elems, map[types.Type]bool{})
	e.entryTypes[key] = h
	return h
}

// holdsElem is mayHoldEntry without memo: t is or holds a record type of elems; a type already on
// the way adds none, an unknown one may.
func holdsElem(t types.Type, elems map[*types.RecordType]bool, seen map[types.Type]bool) bool {
	if t == nil {
		return true
	}
	if rt := recordOf(t); rt != nil && elems[rt] {
		return true
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	parts, known := typeParts(t)
	return !known || slices.ContainsFunc(parts, func(p types.Type) bool { return holdsElem(p, elems, seen) })
}
