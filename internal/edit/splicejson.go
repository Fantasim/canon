package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// jsonDiff finds the smallest set of members and elements whose value changed in a JSON source
// (API.md M1, FMT-02) and edits them in the source wire (M8).
type jsonDiff struct {
	a      *applier
	out    []jsonEdit
	stars  map[string][]atStep // the containers an `at:` `*` reads, by pointer (WIRE.md 6.3)
	places []fieldPlace        // new members, placed once their record's removals are known
	top    *inlineTop          // the records whose object an inline case being compared shares
}

// inlineTop is the old and new records whose object holds the inline case being compared, the
// new one with that case in place: its declaration order places the case's keys (FORMATTER 14.2).
type inlineTop struct {
	old, nw *value.Record
}

// value turns n, stating old at field f's place (nil for none), into nw's source wire.
func (d *jsonDiff) value(old, nw value.Value, n *jsonsrc.Node, f *types.Field) error {
	if d.a.unchanged(old, nw) {
		return nil
	}
	done, err := d.structural(old, nw, n, f)
	if done || err != nil {
		return err
	}
	return d.set(nw, n, f)
}

// structural compares item by item a record, table, map or list n states as an object or
// array; false when n is written again whole.
func (d *jsonDiff) structural(old, nw value.Value, n *jsonsrc.Node, f *types.Field) (bool, error) {
	switch o := old.(type) {
	case *value.Record:
		r, ok := nw.(*value.Record)
		switch {
		case !ok || n.Kind != jsonsrc.Object:
		case sameShape(o.T, r.T):
			return true, d.record(o, r, n)
		case unclaimedIn(n, o) || inlined(r):
			return true, d.swapCase(n, swap{old: o, nw: r}, f) // beside kept keys (M8), an inline case's tag (WIRE.md 5.6)
		}
	case *value.Table:
		if t, ok := nw.(*value.Table); ok && n.Kind == jsonsrc.Object {
			return d.tableIn(o, t, n)
		}
	case *value.Map:
		if m, ok := nw.(*value.Map); ok && n.Kind == jsonsrc.Object {
			return d.mapIn(o, m, n, itemScope(f))
		}
	case *value.List:
		if l, ok := nw.(*value.List); ok && n.Kind == jsonsrc.Array && len(n.Elems) == len(o.Elems) {
			return d.listIn(o, l, n, itemScope(f))
		}
	}
	return false, nil
}

// set writes nw's source wire over n, item by item where an `at:` `*` reads n (WIRE.md 6.3).
func (d *jsonDiff) set(nw value.Value, n *jsonsrc.Node, f *types.Field) error {
	if after, star := d.starOf(n); star {
		return d.starSet(nw, n, f, after)
	}
	node, err := d.a.wireNode(nw, f)
	if err != nil {
		return err
	}
	d.setNode(n, node)
	return nil
}

// setNode writes node over n.
func (d *jsonDiff) setNode(n, node *jsonsrc.Node) {
	d.out = append(d.out, jsonEdit{e: jsonsrc.Edit{Kind: jsonsrc.Set, Pointer: n.Pointer(), Value: node}, anchor: int(n.Span.Start)})
}

func (d *jsonDiff) remove(n *jsonsrc.Node, at int) {
	d.out = append(d.out, jsonEdit{e: jsonsrc.Edit{Kind: jsonsrc.Remove, Pointer: n.Pointer()}, anchor: at})
}

// insert adds v as item at of container c, keyed key in an object.
func (d *jsonDiff) insert(c *jsonsrc.Node, at int, key string, v *jsonsrc.Node) {
	d.out = append(d.out, jsonEdit{e: jsonsrc.Edit{Kind: jsonsrc.Insert, Pointer: c.Pointer(), Key: key, At: at, Value: v}, anchor: itemAnchor(c, at), insert: true})
}

// itemAnchor is where an item goes in c: at the item it goes before, else at c's end.
func itemAnchor(c *jsonsrc.Node, at int) int {
	switch {
	case c.Kind == jsonsrc.Object && at < len(c.Members):
		return int(c.Members[at].KeySpan.Start)
	case c.Kind == jsonsrc.Array && at < len(c.Elems):
		return int(c.Elems[at].Span.Start)
	}
	return int(c.Span.End) - 1
}

// inlined reports a record one of whose fields is inline: written again whole, with each inline
// variant's tag (WIRE.md 5.6, 5.8).
func inlined(r *value.Record) bool {
	return slices.ContainsFunc(fieldsOf(r.T), func(f *types.Field) bool { return f.Inline })
}

// record compares field by field at each field's key path (M1, M8), a pairs field slot key by
// slot key (WIRE.md 5.14), then places its new members and drops the path objects it empties.
func (d *jsonDiff) record(old, nw *value.Record, n *jsonsrc.Node) error {
	from, placed, top := len(d.out), len(d.places), d.top
	d.top = nil
	err := d.fields(old, nw, n)
	d.top = top
	if err != nil {
		return err
	}
	d.settle(n, nw, from, placed)
	return nil
}

// fields is record's field by field comparison.
func (d *jsonDiff) fields(old, nw *value.Record, n *jsonsrc.Node) error {
	fields := fieldsOf(nw.T)
	for i, f := range fields {
		var err error
		switch {
		case f.Pairs != nil:
			err = d.pairs(old, nw, i, n)
		case f.Inline:
			err = d.inline(old, nw, i, n)
		case f.Input == nil:
			err = d.member(old, nw, fields, i, n)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// member compares field i at its key path in object n: a field left or set to its default is
// removed (E6), a new one placed after the previous declared key (FMT-02).
func (d *jsonDiff) member(old, nw *value.Record, fields []*types.Field, i int, n *jsonsrc.Node) error {
	f := fields[i]
	m, holder, depth := memberAt(n, f.WirePath)
	written := nw.Set[i] && nw.Fields[i] != nil
	switch {
	case !written && m != nil:
		return d.dropMember(old, nw, i, m, holder)
	case !written:
	case m != nil && !sameValue(old.Fields[i], nw.Fields[i]) && d.omits(nw, i, f):
		return d.dropMember(old, nw, i, m, holder)
	case m != nil:
		return d.value(old.Fields[i], nw.Fields[i], m, f)
	case !d.omits(nw, i, f):
		return d.insertField(nw, i, holder, depth)
	}
	return nil
}

// writtenValue is field i of rec as its source writes it, nil when left to its default.
func writtenValue(rec *value.Record, i int) value.Value {
	if !rec.Set[i] {
		return nil
	}
	return rec.Fields[i]
}

// omits reports field i of nw left out of its object: it holds its default, and it is not none
// with a @json(none:) marker, which is written as the marker (API.md E6, E7).
func (d *jsonDiff) omits(nw *value.Record, i int, f *types.Field) bool {
	_, none := nw.Fields[i].(*value.None)
	return d.a.isDefault(nw, i) && (!none || f.NoneWire == nil)
}

// memberAt is the value at key path p under object n, the object holding its last key and how
// many keys lead to that object; when the value is absent, the deepest object of the path.
func memberAt(n *jsonsrc.Node, p []string) (*jsonsrc.Node, *jsonsrc.Node, int) {
	holder := n
	for i, key := range p {
		j := objMember(holder, key)
		if j < 0 {
			return nil, holder, i
		}
		v := holder.Members[j].Value
		if i == len(p)-1 {
			return v, holder, i
		}
		if v.Kind != jsonsrc.Object {
			return nil, holder, i
		}
		holder = v
	}
	return nil, holder, 0
}

// memberStart is the offset of the member whose key ends path p in holder.
func memberStart(holder *jsonsrc.Node, p []string) int {
	if j := objMember(holder, p[len(p)-1]); j >= 0 {
		return int(holder.Members[j].KeySpan.Start)
	}
	return int(holder.Span.Start)
}

func objMember(n *jsonsrc.Node, key string) int {
	return slices.IndexFunc(n.Members, func(m jsonsrc.Member) bool { return m.Key == key })
}

// insertField adds field i of rec to holder, the deepest object of its key path present, creating
// the objects the path still lacks (LOD-09), each key placed in declaration order (FMT-02).
func (d *jsonDiff) insertField(rec *value.Record, i int, holder *jsonsrc.Node, depth int) error {
	fields := fieldsOf(rec.T)
	node, err := d.a.wireNode(rec.Fields[i], fields[i])
	if err != nil {
		return err
	}
	p := fields[i].WirePath
	if j := d.pendingAt(holder, p[depth]); j >= 0 && depth+1 < len(p) {
		mergeMember(d.out[j].e.Value, p[depth+1:], node) // an object an earlier field creates (LOD-09)
		return nil
	}
	d.placed(holder, d.owner(rec), p[:depth], p[depth], nested(p[depth+1:], node))
	return nil
}

// owner is the record whose declaration order places a new member of rec's object: rec, or the
// record whose object an inline case rec is shares (WIRE.md 5.6).
func (d *jsonDiff) owner(rec *value.Record) *value.Record {
	if d.top != nil {
		return d.top.nw
	}
	return rec
}

// placed inserts v keyed key into holder, the object at prefix of one stating rec, in rec's
// declaration order, its place settled once the record's removals are known (FMT-02).
func (d *jsonDiff) placed(holder *jsonsrc.Node, rec *value.Record, prefix []string, key string, v *jsonsrc.Node) {
	d.insert(holder, jsonsrc.Place(holder, declaredKeys(rec, prefix), key), key, v)
	d.places = append(d.places, fieldPlace{at: len(d.out) - 1, holder: holder, rec: rec, prefix: prefix})
}

// pendingAt is the index in d.out of the new member keyed key that a record being compared adds
// to holder, -1 for none.
func (d *jsonDiff) pendingAt(holder *jsonsrc.Node, key string) int {
	for _, p := range d.places {
		if p.holder == holder && d.out[p.at].e.Key == key {
			return p.at
		}
	}
	return -1
}

// nested is v under the objects keys name, outermost first.
func nested(keys []string, v *jsonsrc.Node) *jsonsrc.Node {
	for _, key := range slices.Backward(keys) {
		v = &jsonsrc.Node{Kind: jsonsrc.Object, Members: []jsonsrc.Member{{Key: key, Value: v}}}
	}
	return v
}

// mergeMember adds v at key path keys under obj, a new object, after the members it holds: the
// objects of the path it already holds shared, the others created (LOD-09).
func mergeMember(obj *jsonsrc.Node, keys []string, v *jsonsrc.Node) {
	for len(keys) > 1 {
		j := objMember(obj, keys[0])
		if j < 0 || obj.Members[j].Value.Kind != jsonsrc.Object {
			break
		}
		obj, keys = obj.Members[j].Value, keys[1:]
	}
	obj.Members = append(obj.Members, jsonsrc.Member{Key: keys[0], Value: nested(keys[1:], v)})
}
