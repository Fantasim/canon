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
	a   *applier
	out []jsonEdit
}

// value turns n, stating old at field f's place (nil for none), into nw's source wire.
func (d *jsonDiff) value(old, nw value.Value, n *jsonsrc.Node, f *types.Field) error {
	if sameValue(old, nw) {
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
		if ok && n.Kind == jsonsrc.Object && sameShape(o.T, r.T) && !wiredApart(o, r) {
			return true, d.record(o, r, n)
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

// set writes nw's source wire over n.
func (d *jsonDiff) set(nw value.Value, n *jsonsrc.Node, f *types.Field) error {
	node, err := d.a.wireNode(nw, f)
	if err != nil {
		return err
	}
	d.out = append(d.out, jsonEdit{e: jsonsrc.Edit{Kind: jsonsrc.Set, Pointer: n.Pointer(), Value: node}, anchor: int(n.Span.Start)})
	return nil
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

// wiredApart reports a record one of whose changed fields its object does not hold in one
// member (an inline variant, parallel keys): the record is written again whole.
func wiredApart(o, r *value.Record) bool {
	for i, f := range fieldsOf(r.T) {
		if (f.Inline || f.Pairs != nil) && !sameValue(o.Fields[i], r.Fields[i]) {
			return true
		}
	}
	return false
}

// record compares field by field at each field's key path (M1, M8): a field left or set to its
// default is removed (E6), a new one placed after the previous declared key (FMT-02).
func (d *jsonDiff) record(old, nw *value.Record, n *jsonsrc.Node) error {
	fields := fieldsOf(nw.T)
	for i, f := range fields {
		if f.Input != nil || f.Inline || f.Pairs != nil {
			continue
		}
		m, holder, depth := memberAt(n, f.WirePath)
		written := nw.Set[i] && nw.Fields[i] != nil
		var err error
		switch {
		case !written && m != nil:
			d.remove(m, memberStart(holder, f.WirePath))
		case !written:
		case m != nil && !sameValue(old.Fields[i], nw.Fields[i]) && d.omits(nw, i, f):
			d.remove(m, memberStart(holder, f.WirePath))
		case m != nil:
			err = d.value(old.Fields[i], nw.Fields[i], m, f)
		case !d.omits(nw, i, f):
			err = d.insertField(fields, i, nw.Fields[i], holder, depth)
		}
		if err != nil {
			return err
		}
	}
	return nil
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

// insertField adds field i to holder, the deepest object of its key path present, creating the
// objects the path still lacks (LOD-09), each key placed in declaration order (FMT-02).
func (d *jsonDiff) insertField(fields []*types.Field, i int, v value.Value, holder *jsonsrc.Node, depth int) error {
	node, err := d.a.wireNode(v, fields[i])
	if err != nil {
		return err
	}
	p := fields[i].WirePath
	for k := len(p) - 1; k > depth; k-- {
		node = &jsonsrc.Node{Kind: jsonsrc.Object, Members: []jsonsrc.Member{{Key: p[k], Value: node}}}
	}
	at := jsonsrc.Place(holder, declaredKeys(fields, p[:depth]), p[depth])
	d.insert(holder, at, p[depth], node)
	return nil
}

// declaredKeys are the keys at prefix of the record's fields in declaration order, a path's
// first key where its first field stands (FMT-02, LOD-09).
func declaredKeys(fields []*types.Field, prefix []string) []string {
	var out []string
	for _, f := range fields {
		p := f.WirePath
		if f.Inline || f.Pairs != nil || len(p) <= len(prefix) || !slices.Equal(p[:len(prefix)], prefix) {
			continue
		}
		if k := p[len(prefix)]; !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}
