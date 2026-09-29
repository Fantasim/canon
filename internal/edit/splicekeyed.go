package edit

import (
	"slices"
	"strconv"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// kept is an item of a literal: its identity, its value, its position among the list's items,
// the node Remove takes and the node stating the value.
type kept struct {
	id   string
	v    value.Value
	pos  int
	item syntax.Node
	val  syntax.Node
}

// fresh is an item of the new value: its identity, value and text as a new item.
type fresh struct {
	id   string
	v    value.Value
	text func() (string, error)
}

// keyed compares two collections item by item by identity (API.md M1): items gone are
// removed, new ones inserted after the kept item before them, kept ones compared; false
// when the kept items changed order or an identity repeats, and the whole node is re-printed.
func (d *canonDiff) keyed(list syntax.Tok, olds []kept, news []fresh) (bool, error) {
	oldAt, newAt := map[string]int{}, map[string]int{}
	for i, o := range olds {
		oldAt[o.id] = i
	}
	for i, n := range news {
		newAt[n.id] = i
	}
	if len(oldAt) != len(olds) || len(newAt) != len(news) || !sameKept(olds, news, oldAt, newAt) {
		return false, nil
	}
	for _, o := range olds {
		if _, ok := newAt[o.id]; !ok {
			d.remove(o.item)
		}
	}
	at := 0
	for _, n := range news {
		if i, ok := oldAt[n.id]; ok {
			if err := d.value(olds[i].v, n.v, olds[i].val); err != nil {
				return true, err
			}
			at = olds[i].pos + 1
			continue
		}
		text, err := n.text()
		if err != nil {
			return true, err
		}
		d.insert(list, at, text)
	}
	return true, nil
}

// sameKept reports the items both collections hold in the same order.
func sameKept(olds []kept, news []fresh, oldAt, newAt map[string]int) bool {
	var a, b []string
	for _, o := range olds {
		if _, ok := newAt[o.id]; ok {
			a = append(a, o.id)
		}
	}
	for _, n := range news {
		if _, ok := oldAt[n.id]; ok {
			b = append(b, n.id)
		}
	}
	return slices.Equal(a, b)
}

// table compares a table literal entry by entry (M1); an entry stated outside the literal
// (an `entry` declaration) cannot be kept in order by a literal: order.
func (d *canonDiff) table(old, nw *value.Table, lit *syntax.BraceLit) (bool, error) {
	var olds []kept
	for pos, it := range lit.Items {
		ei, ok := it.(*syntax.EntryItem)
		if !ok {
			return false, nil
		}
		if i := entryIndex(old.Entries, ei.Key.Name); i >= 0 {
			olds = append(olds, kept{id: ei.Key.Name, v: old.Entries[i], pos: pos, item: ei, val: ei})
		}
	}
	if len(olds) != len(old.Entries) {
		return true, &NotEditableError{Reason: ReasonOrder}
	}
	news := make([]fresh, len(nw.Entries))
	for i, e := range nw.Entries {
		key := entryKey(e).Text()
		news[i] = fresh{id: key, v: e, text: func() (string, error) { return d.a.entryText(key, e, isRetired(e)) }}
	}
	return d.keyed(lit.First(), olds, news)
}

// entryIndex is the index of the entry with key, -1 when there is none.
func entryIndex(entries []*value.Record, key string) int {
	return slices.IndexFunc(entries, func(e *value.Record) bool { return !entryKey(e).IsInt && entryKey(e).S == key })
}

// mapItems compares a map literal key by key (M1): its items state the keys in order.
func (d *canonDiff) mapItems(old, nw *value.Map, lit *syntax.BraceLit) (bool, error) {
	if len(lit.Items) != len(old.Keys) {
		return false, nil
	}
	olds := make([]kept, len(old.Keys))
	for i, k := range old.Keys {
		var val syntax.Node
		switch x := lit.Items[i].(type) {
		case *syntax.MapItem:
			val = syntax.Unparen(x.Value)
		case *syntax.FieldItem:
			val = syntax.Unparen(x.Value)
		default:
			return false, nil
		}
		olds[i] = kept{id: k.CanonText(), v: old.Vals[i], pos: i, item: lit.Items[i], val: val}
	}
	news := make([]fresh, len(nw.Keys))
	for i, k := range nw.Keys {
		v := nw.Vals[i]
		news[i] = fresh{id: k.CanonText(), v: v, text: func() (string, error) { return d.a.mapItemText(k, v) }}
	}
	return d.keyed(lit.First(), olds, news)
}

// mapItemText is `key: value` as a new item of a map literal.
func (a *applier) mapItemText(k, v value.Value) (string, error) {
	key, err := a.canonText(k)
	if err != nil {
		return "", err
	}
	val, err := a.canonText(v)
	return key + colonSp + val, err
}

// list compares a keyed list element by key, a plain list of equal length position by
// position (M1); any other change re-prints the list.
func (d *canonDiff) list(old, nw *value.List, n syntax.Node) (bool, error) {
	lit, ok := n.(*syntax.ListLit)
	if !ok || len(lit.Elems) != len(old.Elems) {
		return false, nil
	}
	lt, _ := old.T.Base().(*types.ListType)
	if lt != nil && lt.KeyedBy != nil {
		return d.keyed(lit.First(), elemsKept(old, lit, lt.KeyedBy), elemsFresh(d.a, nw, lt.KeyedBy))
	}
	if len(nw.Elems) != len(old.Elems) {
		return false, nil
	}
	for i, e := range old.Elems {
		if err := d.value(e, nw.Elems[i], syntax.Unparen(lit.Elems[i])); err != nil {
			return true, err
		}
	}
	return true, nil
}

func elemsKept(l *value.List, lit *syntax.ListLit, key *types.Field) []kept {
	out := make([]kept, len(l.Elems))
	for i, e := range l.Elems {
		el := syntax.Unparen(lit.Elems[i])
		out[i] = kept{id: elemID(e, key, i), v: e, pos: i, item: lit.Elems[i], val: el}
	}
	return out
}

func elemsFresh(a *applier, l *value.List, key *types.Field) []fresh {
	out := make([]fresh, len(l.Elems))
	for i, e := range l.Elems {
		out[i] = fresh{id: elemID(e, key, -1-i), v: e, text: func() (string, error) { return a.canonText(e) }}
	}
	return out
}

// elemID is a keyed-list element's identity: its key, else a mark of its own position.
func elemID(e value.Value, key *types.Field, i int) string {
	if k := keyField(e, key); k != nil {
		return k.CanonText()
	}
	return string(positionMark) + strconv.Itoa(i)
}
