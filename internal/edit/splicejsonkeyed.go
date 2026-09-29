package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// jkept is a member or element of a JSON container: its identity, value, position and node.
type jkept struct {
	id   string
	v    value.Value
	pos  int
	node *jsonsrc.Node
	at   int // the offset of its member's key, or of the element
}

// jfresh is an item of the new value: its identity, its key in an object, its value.
type jfresh struct {
	id, key string
	v       value.Value
	node    func() (*jsonsrc.Node, error)
}

// keyed compares container c item by item by identity (M1, FMT-02), as the
// canon diff does; f is the field whose scope the items' wire takes.
func (d *jsonDiff) keyed(c *jsonsrc.Node, olds []jkept, news []jfresh, f *types.Field) (bool, error) {
	oldAt, newAt := map[string]int{}, map[string]int{}
	for i, o := range olds {
		oldAt[o.id] = i
	}
	for i, n := range news {
		newAt[n.id] = i
	}
	if len(oldAt) != len(olds) || len(newAt) != len(news) || !slices.Equal(keptIDs(olds, newAt), freshIDs(news, oldAt)) {
		return false, nil
	}
	for _, o := range olds {
		if _, ok := newAt[o.id]; !ok {
			d.remove(o.node, o.at)
		}
	}
	at := 0
	for _, n := range news {
		if i, ok := oldAt[n.id]; ok {
			if err := d.value(olds[i].v, n.v, olds[i].node, f); err != nil {
				return true, err
			}
			at = olds[i].pos + 1
			continue
		}
		node, err := n.node()
		if err != nil {
			return true, err
		}
		d.insert(c, at, n.key, node)
	}
	return true, nil
}

func keptIDs(olds []jkept, newAt map[string]int) []string {
	var out []string
	for _, o := range olds {
		if _, ok := newAt[o.id]; ok {
			out = append(out, o.id)
		}
	}
	return out
}

func freshIDs(news []jfresh, oldAt map[string]int) []string {
	var out []string
	for _, n := range news {
		if _, ok := oldAt[n.id]; ok {
			out = append(out, n.id)
		}
	}
	return out
}

// tableIn compares a table's object member by member, each keyed by its entry's id; an entry
// retired or brought back is written again (WIR-06).
func (d *jsonDiff) tableIn(old, nw *value.Table, n *jsonsrc.Node) (bool, error) {
	var olds []jkept
	for pos, m := range n.Members {
		i := entryIndex(old.Entries, m.Key)
		if i < 0 {
			return false, nil
		}
		olds = append(olds, jkept{id: entryID(old.Entries[i]), v: old.Entries[i], pos: pos, node: m.Value, at: int(m.KeySpan.Start)})
	}
	news := make([]jfresh, len(nw.Entries))
	for i, e := range nw.Entries {
		news[i] = jfresh{id: entryID(e), key: entryKey(e).Text(), v: e, node: func() (*jsonsrc.Node, error) { return d.a.entryNode(e) }}
	}
	return d.keyed(n, olds, news, nil)
}

// entryID is an entry's identity in a diff: its key, marked when it is retired.
func entryID(e *value.Record) string {
	if isRetired(e) {
		return retiredWord + space + entryKey(e).Text()
	}
	return entryKey(e).Text()
}

// entryNode is a table entry's object, `"$retired": true` first when it is retired (E10).
func (a *applier) entryNode(e *value.Record) (*jsonsrc.Node, error) {
	n, err := a.wireNode(e, nil)
	if err != nil || !isRetired(e) {
		return n, err
	}
	m, err := retiredMember()
	if err != nil {
		return nil, err
	}
	n.Members = slices.Insert(n.Members, 0, m)
	return n, nil
}

// mapIn compares a map's object key by key, each by its wire key (WIRE.md §5.8).
func (d *jsonDiff) mapIn(old, nw *value.Map, n *jsonsrc.Node, f *types.Field) (bool, error) {
	var olds []jkept
	for pos, m := range n.Members {
		i := slices.IndexFunc(old.Keys, func(k value.Value) bool { t, err := wire.KeyText(k); return err == nil && t == m.Key })
		if i < 0 {
			return false, nil
		}
		olds = append(olds, jkept{id: m.Key, v: old.Vals[i], pos: pos, node: m.Value, at: int(m.KeySpan.Start)})
	}
	news := make([]jfresh, len(nw.Keys))
	for i, k := range nw.Keys {
		key, err := wire.KeyText(k)
		if err != nil {
			return true, err
		}
		v := nw.Vals[i]
		news[i] = jfresh{id: key, key: key, v: v, node: func() (*jsonsrc.Node, error) { return d.a.wireNode(v, f) }}
	}
	return d.keyed(n, olds, news, f)
}

// listIn compares a keyed list element by key, a plain one position by position (M1).
func (d *jsonDiff) listIn(old, nw *value.List, n *jsonsrc.Node, f *types.Field) (bool, error) {
	lt, _ := old.T.Base().(*types.ListType)
	if lt != nil && lt.KeyedBy != nil {
		olds := make([]jkept, len(old.Elems))
		for i, e := range old.Elems {
			olds[i] = jkept{id: elemID(e, lt.KeyedBy, i), v: e, pos: i, node: n.Elems[i], at: int(n.Elems[i].Span.Start)}
		}
		news := make([]jfresh, len(nw.Elems))
		for i, e := range nw.Elems {
			news[i] = jfresh{id: elemID(e, lt.KeyedBy, -1-i), v: e, node: func() (*jsonsrc.Node, error) { return d.a.wireNode(e, f) }}
		}
		return d.keyed(n, olds, news, f)
	}
	if len(nw.Elems) != len(old.Elems) {
		return false, nil
	}
	for i, e := range old.Elems {
		if err := d.value(e, nw.Elems[i], n.Elems[i], f); err != nil {
			return true, err
		}
	}
	return true, nil
}

// itemScope is the field whose unit and encodings an item of f's collection takes: f without
// its own none marker, which only f's outer optional has (WIR-02).
func itemScope(f *types.Field) *types.Field {
	if f == nil {
		return nil
	}
	cp := *f
	cp.NoneWire, cp.Inline, cp.Pairs = nil, false, nil
	return &cp
}
