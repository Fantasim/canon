package edit

import (
	"bytes"
	"slices"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// swap is a change of case, or of record type, of the value an object states, from old to nw;
// outer is the old record whose object an inline old shares (nil when the object states old),
// owner the record whose declaration order places new members (WIRE.md 5.6, FORMATTER.md 14.2).
type swap struct {
	old, nw, outer, owner *value.Record
}

// keptClash refuses a case change whose new value cannot be written beside a kept key: one it
// would decode as part of the new value (DECISIONS 471, rulings of 2026-11-05).
func keptClash() error { return &NotEditableError{Reason: ReasonComputed} }

// inline compares inline field i of old and nw, both stated by object n: one case field by field
// in n, another case swapped member by member beside n's other members (WIRE.md 5.6, M8, 471).
func (d *jsonDiff) inline(old, nw *value.Record, i int, n *jsonsrc.Node) error {
	nv := nw.Fields[i]
	if nv == nil {
		nv, _ = d.a.defaultOf(nw, i)
	}
	if d.a.unchanged(old.Fields[i], nv) {
		return nil
	}
	oc, okOld := old.Fields[i].(*value.Record)
	nc, okNew := nv.(*value.Record)
	if !okOld || !okNew || oc == nil || nc == nil {
		return &NotEditableError{Reason: ReasonComputed}
	}
	top := d.top
	if top == nil {
		d.top = &inlineTop{old: old, nw: withField(nw, i, nc)}
	}
	defer func() { d.top = top }()
	if sameShape(oc.T, nc.T) {
		return d.fields(oc, nc, n)
	}
	plain := *fieldsOf(nw.T)[i]
	plain.Inline = false
	return d.swapCase(n, swap{old: oc, nw: nc, outer: d.top.old, owner: d.top.nw}, &plain)
}

// swapCase turns object n, stating s.old as field f, into s.nw's wire member by member: the old
// value's keys rewritten or removed, the new value's added in declaration order, every other key
// kept in place, where a kept key before the old tag stays before the new one (471).
func (d *jsonDiff) swapCase(n *jsonsrc.Node, s swap, f *types.Field) error {
	s.nw = d.inlineFilled(s.nw)
	node, err := d.recordWire(s.nw, f)
	if err != nil || node.Kind != jsonsrc.Object {
		return &NotEditableError{Reason: ReasonComputed} // no object beside the kept keys decodes to it
	}
	if s.owner == nil {
		s.owner = s.nw
	}
	from, placed := len(d.out), len(d.places)
	if err := d.swapIn(n, node, s, nil); err != nil {
		return err
	}
	if s.outer == nil {
		d.settle(n, s.nw, from, placed) // an inline case's settle is its parent record's
	}
	return nil
}

// inlineFilled is rec with each inline field it leaves out holding its default (WIRE.md 5.8).
func (d *jsonDiff) inlineFilled(rec *value.Record) *value.Record {
	out := rec
	for j, f := range fieldsOf(rec.T) {
		if !f.Inline || fieldValue(rec, j) != nil {
			continue
		}
		if dv, ok := d.a.defaultOf(rec, j); ok {
			out = withField(out, j, dv)
		}
	}
	return out
}

// swapIn is swapCase for n, the object at key path prefix, and node, the new value's object there.
func (d *jsonDiff) swapIn(n, node *jsonsrc.Node, s swap, prefix []string) error {
	for _, mem := range n.Members {
		if err := d.swapMember(n, mem, node, s, prefix); err != nil {
			return err
		}
	}
	for _, m := range node.Members {
		if objMember(n, m.Key) < 0 {
			d.placed(n, s.owner, prefix, m.Key, m.Value)
		}
	}
	return nil
}

// swapMember turns member mem of n, at prefix, into node's member of its key: a key of the old
// value rewritten, or removed with what it holds (ruling), a path's object merged.
func (d *jsonDiff) swapMember(n *jsonsrc.Node, mem jsonsrc.Member, node *jsonsrc.Node, s swap, prefix []string) error {
	oc := claimAt(s.old, prefix, mem.Key)
	j := objMember(node, mem.Key)
	switch {
	case oc.under && mem.Value.Kind == jsonsrc.Object:
		return d.swapUnder(n, mem, node, s, prefix)
	case !oc.claimed:
		return d.swapUnknown(n, mem, node, s, prefix)
	case j < 0:
		d.remove(mem.Value, int(mem.KeySpan.Start))
	case !sameJSON(mem.Value, node.Members[j].Value):
		d.setNode(mem.Value, node.Members[j].Value)
	}
	return nil
}

// swapUnknown is swapMember for a key the old value does not claim, a third case's included: a
// sibling field's left to it, one the new value claims refused, any other kept (rulings 2026-11-05).
func (d *jsonDiff) swapUnknown(n *jsonsrc.Node, mem jsonsrc.Member, node *jsonsrc.Node, s swap, prefix []string) error {
	nc := claimAt(s.nw, prefix, mem.Key)
	switch {
	case s.outer != nil && claimAt(s.outer, prefix, mem.Key).claimed:
	case nc.under && mem.Value.Kind == jsonsrc.Object:
		return d.swapUnder(n, mem, node, s, prefix)
	case nc.claimed:
		return keptClash() // kept, it would decode as the new value's
	}
	return nil
}

// swapUnder is swapMember for an object a path creates: merged with the new value's object of its
// key, its kept keys kept, removed once nothing is left in it (WIRE.md 5.5.3; ruling).
func (d *jsonDiff) swapUnder(n *jsonsrc.Node, mem jsonsrc.Member, node *jsonsrc.Node, s swap, prefix []string) error {
	sub := append(slices.Clone(prefix), mem.Key)
	if nc := claimAt(s.nw, prefix, mem.Key); nc.claimed && !nc.under {
		return d.swapLeaf(mem, node, s, sub)
	}
	inner := &jsonsrc.Node{Kind: jsonsrc.Object}
	if j := objMember(node, mem.Key); j >= 0 {
		inner = node.Members[j].Value
	}
	if inner.Kind != jsonsrc.Object {
		return keptClash()
	}
	from := len(d.out)
	if err := d.swapIn(mem.Value, inner, s, sub); err != nil {
		return err
	}
	d.dropEmptied(n, sub[len(prefix):], from, d.removedSince(from))
	return nil
}

// swapLeaf is swapUnder where the new value holds the key at sub as a field's value: refused when
// the old object keeps a key, which that value cannot hold (ruling of 2026-11-05).
func (d *jsonDiff) swapLeaf(mem jsonsrc.Member, node *jsonsrc.Node, s swap, sub []string) error {
	if keeps(mem.Value, s.old, sub) {
		return keptClash()
	}
	if j := objMember(node, mem.Key); j >= 0 {
		d.setNode(mem.Value, node.Members[j].Value)
		return nil
	}
	d.remove(mem.Value, int(mem.KeySpan.Start))
	return nil
}

// keeps reports n, the object at prefix of one stating rec, holding a key rec does not claim, in
// it or in an object a path creates under it.
func keeps(n *jsonsrc.Node, rec *value.Record, prefix []string) bool {
	return slices.ContainsFunc(n.Members, func(m jsonsrc.Member) bool {
		c := claimAt(rec, prefix, m.Key)
		return !c.claimed || c.under && keeps(m.Value, rec, append(slices.Clone(prefix), m.Key))
	})
}

// sameJSON reports two values written alike in canonical layout (FMT-02).
func sameJSON(x, y *jsonsrc.Node) bool {
	return bytes.Equal(jsonsrc.Format(x), jsonsrc.Format(y))
}
