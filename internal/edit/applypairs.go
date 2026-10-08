package edit

import (
	"strconv"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// pairsAt reports the value at cursor k a list its JSON source writes as the parallel slot keys
// of its parent object (WIRE.md 5.14), not as one member.
func (x *opCtx) pairsAt(k int) bool {
	if k <= 0 || k >= len(x.j.cur) || x.j.cur[k].mode != ModeJSON {
		return false
	}
	f := x.fieldAt(k)
	return f != nil && f.Pairs != nil
}

// pairElem reports the value at cursor k an element of such a list: a whole slot.
func (x *opCtx) pairElem(k int) bool {
	return k >= slotBack && x.pairsAt(k-1)
}

// writtenAt is the cursor the value at cursor k is written through: for a slot of a pairs list,
// the record whose object holds its keys (WIRE.md 5.14); for an element of a list its JSON
// source writes as one number (bits, 5.3), the list, written again whole (M1); else k itself.
func (x *opCtx) writtenAt(k int) int {
	switch {
	case x.pairElem(k):
		return x.statedBelow(k - slotBack)
	case x.inWhole(k):
		return k - 1
	}
	return k
}

// inWhole reports the value at cursor k an element of a list its JSON source states as one
// scalar (writtenWhole).
func (x *opCtx) inWhole(k int) bool {
	if k <= 0 || k >= len(x.j.cur) || x.j.cur[k-1].mode != ModeJSON || x.j.cur[k-1].state != stTree {
		return false
	}
	l, ok := x.valueAt(k - 1).(*value.List)
	return ok && x.writtenWhole(l)
}

// pairsOp writes an Add, Insert, Remove or Move on the pairs list at cursor k as the whole list
// nw, its slot keys compared in its parent object (WIRE.md 5.14; log-2026-09-29 M4 B10-r3).
func (x *opCtx) pairsOp(k int, nw value.Value) error {
	if err := x.slotsFit(k, nw); err != nil {
		return err
	}
	if err := x.pairsInverse(k); err != nil {
		return err
	}
	p := x.statedBelow(k - 1)
	old := x.valueAt(p)
	return x.diffAt(p, old, withChange(old, x.res.Steps[p:k], nw))
}

// pairsInverse is the Undo of an operation on the pairs list at cursor k, which keeps no
// position: Set of the list as it was, or Reset of a defaulted field with no slot (E22, E23).
func (x *opCtx) pairsInverse(k int) error {
	path := x.pathAt(k)
	f := x.fieldAt(k)
	if rec, ok := x.valueAt(k - 1).(*value.Record); ok && hasDefault(f) && !rec.Set[f.Index] {
		x.w.undo = []Operation{{Kind: OpReset, Path: path}}
		return nil
	}
	lit, err := x.a.sourceLit(x.valueAt(k), x.scopeAt(k, true), x.statedAt(k))
	if err != nil {
		return err
	}
	x.w.undo = []Operation{{Kind: OpSet, Path: path, Value: lit}}
	return nil
}

// slotsFit refuses v as the pairs list at cursor k, or as one of its elements, when no JSON
// source writes it (API.md V1; WIRE.md 5.14; log-2026-09-29 M4 B11, B11-r). A .canon one is re-checked.
func (x *opCtx) slotsFit(k int, v value.Value) error {
	why := ""
	switch {
	case x.pairsAt(k):
		why = slotsRefusal(x.fieldAt(k), v)
	case x.pairElem(k):
		why = missingDetail(v)
	}
	if why == "" {
		return nil
	}
	return &ValueError{Expected: x.declaredAt(k).String(), Got: describe(x.op.Value), Detail: why}
}

// slotsRefusal is why v, the value of pairs field f, has no wire form: more elements than slots,
// or an element a slot cannot write whole; "" when it has one (WIRE.md 5.14).
func slotsRefusal(f *types.Field, v value.Value) string {
	l, ok := v.(*value.List)
	switch {
	case !ok || f.Pairs == nil:
		return ""
	case len(l.Elems) > f.Pairs.Slots:
		return detailSlots + strconv.Itoa(f.Pairs.Slots)
	}
	for _, e := range l.Elems {
		if why := missingDetail(e); why != "" {
			return why
		}
	}
	return ""
}

// missingDetail is why pair e cannot fill a slot: a field left out that has no default, which
// would leave one of the slot's keys alone (E7117); "" when it can.
func missingDetail(e value.Value) string {
	rec, ok := e.(*value.Record)
	if !ok {
		return ""
	}
	for j, g := range fieldsOf(rec.T) {
		if (j >= len(rec.Fields) || rec.Fields[j] == nil) && !hasDefault(g) {
			return detailSlotField + g.Name
		}
	}
	return ""
}
