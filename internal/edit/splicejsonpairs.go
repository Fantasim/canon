package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// pairs turns the slot members of old's pairs field i in object n into nw's: a changed one written
// again, one past the new length removed, a new one inserted after the slots kept (WIRE.md 5.14;
// M1, M3, M4; log-2026-09-29 M4 B10-r3, B11-r).
func (d *jsonDiff) pairs(old, nw *value.Record, i int, n *jsonsrc.Node) error {
	f := fieldsOf(nw.T)[i]
	ol, nl := d.a.slotElems(writtenValue(old, i)), d.a.slotElems(writtenValue(nw, i))
	if sameValues(ol, nl) {
		return nil
	}
	was, err := d.a.slotMembers(writtenValue(old, i), f)
	if err != nil {
		return err
	}
	now, err := d.a.slotMembers(writtenValue(nw, i), f)
	if err != nil {
		return err
	}
	for _, m := range was {
		if j := objMember(n, m.Key); j >= 0 && !slices.ContainsFunc(now, keyed(m.Key)) {
			d.remove(n.Members[j].Value, int(n.Members[j].KeySpan.Start))
		}
	}
	at, err := d.slotsEnd(n, old, i, was, now)
	if err != nil {
		return err
	}
	per := len(f.Pairs.Keys)
	for k, m := range now {
		j := objMember(n, m.Key)
		switch {
		case j < 0:
			d.insert(n, at, m.Key, m.Value) // at one place, inserts apply in reverse: slot order (applyOrder)
		case k/per >= len(ol) || !sameValue(pairField(ol[k/per], k%per), pairField(nl[k/per], k%per)):
			d.setNode(n.Members[j].Value, m.Value) // by value, never by its text (M1)
		}
	}
	return nil
}

// pairField is field j of pair e, e itself when it is no record.
func pairField(e value.Value, j int) value.Value {
	if rec, ok := e.(*value.Record); ok && j < len(rec.Fields) {
		return rec.Fields[j]
	}
	return e
}

// slotElems are the elements of pairs list v as its slots write them, none for nil.
func (a *applier) slotElems(v value.Value) []value.Value {
	l, ok := v.(*value.List)
	if !ok {
		return nil
	}
	return a.pairsWritten(l).Elems
}

// keyed matches a member whose key is key.
func keyed(key string) func(jsonsrc.Member) bool {
	return func(m jsonsrc.Member) bool { return m.Key == key }
}

// slotsEnd is where the new slots of old's pairs field i go in object n: after the last slot
// member n holds, else at the field's place among the keys of the fields old writes (WIRE.md
// 5.14 "at the field's position", M8).
func (d *jsonDiff) slotsEnd(n *jsonsrc.Node, old *value.Record, i int, was, now []jsonsrc.Member) (int, error) {
	last := -1
	for _, m := range was {
		last = max(last, objMember(n, m.Key))
	}
	if last >= 0 || len(now) == 0 {
		return last + 1, nil
	}
	declared, err := d.slotPlaces(old, i, now[0].Key)
	if err != nil {
		return 0, err
	}
	return jsonsrc.Place(n, declared, now[0].Key), nil
}

// slotPlaces are the keys of old's fields in declaration order, field i's first slot key at its
// place, another pairs field's by the slots it writes (FMT-02, WIRE.md 5.14).
func (d *jsonDiff) slotPlaces(old *value.Record, i int, key string) ([]string, error) {
	var declared []string
	for j, g := range fieldsOf(old.T) {
		switch {
		case j == i:
			declared = append(declared, key)
		case g.Pairs != nil:
			ms, err := d.a.slotMembers(writtenValue(old, j), g)
			if err != nil {
				return nil, err
			}
			for _, m := range ms {
				declared = append(declared, m.Key)
			}
		case g.Input != nil || g.Inline || len(g.WirePath) == 0:
		case !slices.Contains(declared, g.WirePath[0]):
			declared = append(declared, g.WirePath[0])
		}
	}
	return declared, nil
}

// slotMembers are the members pairs list v writes in its parent object, in slot order (WIRE.md
// 5.14): each element with both fields, a field left to its default holding it; none for nil.
func (a *applier) slotMembers(v value.Value, f *types.Field) ([]jsonsrc.Member, error) {
	l, ok := v.(*value.List)
	if !ok || len(l.Elems) == 0 {
		return nil, nil
	}
	node, err := encodeWire(a.pairsWritten(l), f)
	if err != nil {
		return nil, err
	}
	return node.Members, nil
}

// pairsWritten is pairs list l as its slots write it: every field of each element written, one
// left out holding its default (WIRE.md 5.14: both keys of a filled slot are present).
func (a *applier) pairsWritten(l *value.List) *value.List {
	out := &value.List{T: l.T, Elems: make([]value.Value, len(l.Elems)), P: l.P}
	for i, e := range l.Elems {
		out.Elems[i] = e
		rec, ok := e.(*value.Record)
		if !ok {
			continue
		}
		full := &value.Record{T: rec.T, Fields: slices.Clone(rec.Fields), Set: slices.Clone(rec.Set), Ident: rec.Ident, P: rec.P}
		for j := range full.Fields {
			if full.Fields[j] == nil {
				full.Fields[j], _ = a.defaultOf(full, j)
			}
			full.Set[j] = full.Fields[j] != nil
		}
		out.Elems[i] = full
	}
	return out
}
