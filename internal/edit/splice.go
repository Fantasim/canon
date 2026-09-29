package edit

import (
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// canonDiff finds the smallest set of items whose value changed between a stated value and its
// new value (API.md M1), as changes of one .canon file: a re-printed item, an item inserted
// where W7 or the collection puts it, an item removed (M2-M4).
type canonDiff struct {
	a   *applier
	out []format.Change
}

// value turns n, the node stating old, into a statement of nw: n is a value expression, a
// table entry or an entry declaration, whose value is its brace.
func (d *canonDiff) value(old, nw value.Value, n syntax.Node) error {
	if sameValue(old, nw) {
		return nil
	}
	done, err := d.structural(old, nw, n)
	if done || err != nil {
		return err
	}
	return d.replace(nw, n)
}

// structural compares old and nw item by item when both are the same kind of collection and
// n states it item by item; false when the whole node is re-printed instead (M1).
func (d *canonDiff) structural(old, nw value.Value, n syntax.Node) (bool, error) {
	switch o := old.(type) {
	case *value.Record:
		r, ok := nw.(*value.Record)
		if lit := braceOf(n); ok && lit != nil && sameShape(o.T, r.T) {
			return true, d.record(o, r, lit)
		}
	case *value.Table:
		if t, ok := nw.(*value.Table); ok && braceOf(n) != nil {
			return d.table(o, t, braceOf(n))
		}
	case *value.Map:
		if m, ok := nw.(*value.Map); ok && braceOf(n) != nil {
			return d.mapItems(o, m, braceOf(n))
		}
	case *value.List:
		if l, ok := nw.(*value.List); ok {
			return d.list(o, l, n)
		}
	}
	return false, nil
}

// replace re-prints the whole value n states (M2): an entry's brace for an entry.
func (d *canonDiff) replace(nw value.Value, n syntax.Node) error {
	text, err := d.a.canonText(nw)
	if err != nil {
		return err
	}
	switch x := n.(type) {
	case *syntax.EntryItem:
		n = x.Value
	case *syntax.EntryDecl:
		n = x.Value
	}
	d.out = append(d.out, format.Change{Kind: format.Replace, Node: n, Text: text})
	return nil
}

// record compares field by field (M1): a field left or set to its default is removed (E6,
// M7), a new one inserted in declaration order (W7), beside a spread when one supplies it (W9).
func (d *canonDiff) record(old, nw *value.Record, lit *syntax.BraceLit) error {
	for i, f := range fieldsOf(nw.T) {
		if f.Input != nil {
			continue
		}
		fi := fieldItem(lit, f.Name)
		var err error
		switch {
		case !nw.Set[i] || nw.Fields[i] == nil:
			err = d.leftOut(old, nw, i, lit, fi)
		case fi != nil:
			err = d.present(old, nw, i, lit, fi)
		default:
			err = d.absent(old, nw, i, lit)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// present is a written field the literal holds: kept when unchanged, removed when set to its
// default (E6) unless a spread would then supply it, else compared.
func (d *canonDiff) present(old, nw *value.Record, i int, lit *syntax.BraceLit, fi *syntax.FieldItem) error {
	switch {
	case sameValue(old.Fields[i], nw.Fields[i]):
		return nil
	case !hasSpread(lit) && d.a.isDefault(nw, i):
		d.remove(fi)
		return nil
	}
	return d.value(old.Fields[i], nw.Fields[i], syntax.Unparen(fi.Value))
}

// absent is a written field the literal lacks: inserted unless a spread supplies that value,
// or, without a spread, the value is its default computed from the new record (W7, W9, E6).
func (d *canonDiff) absent(old, nw *value.Record, i int, lit *syntax.BraceLit) error {
	if hasSpread(lit) && sameValue(old.Fields[i], nw.Fields[i]) || !hasSpread(lit) && d.a.isDefault(nw, i) {
		return nil
	}
	return d.insertField(nw, i, nw.Fields[i], lit)
}

// leftOut is a field the new value leaves to its default (API.md V3): its item is removed, or
// under a spread its default is written, since removing it would let the spread supply it.
func (d *canonDiff) leftOut(old, nw *value.Record, i int, lit *syntax.BraceLit, fi *syntax.FieldItem) error {
	if !hasSpread(lit) {
		if fi != nil {
			d.remove(fi)
		}
		return nil
	}
	dv, ok := d.a.defaultOf(nw, i)
	switch {
	case !ok || sameValue(old.Fields[i], dv):
		return nil
	case fi != nil:
		return d.value(old.Fields[i], dv, syntax.Unparen(fi.Value))
	}
	return d.insertField(nw, i, dv, lit)
}

// insertField inserts `name: v` after the nearest present field declared before it, after
// any spread (W7, W9).
func (d *canonDiff) insertField(rec *value.Record, i int, v value.Value, lit *syntax.BraceLit) error {
	text, err := d.a.canonText(v)
	if err != nil {
		return err
	}
	fields := fieldsOf(rec.T)
	d.insert(lit.First(), w7Position(lit, fields, i), fields[i].Name+colonSp+text)
	return nil
}

// w7Position is where field i goes in lit: after the present field declared nearest before
// it, first if none, and never before a spread (API.md W7, W9).
func w7Position(lit *syntax.BraceLit, fields []*types.Field, i int) int {
	at, nearest := 0, -1
	for k, it := range lit.Items {
		switch x := it.(type) {
		case *syntax.SpreadItem:
			at = max(at, k+1)
		case *syntax.FieldItem:
			if j := fieldIndex(fields, x.Name.Name); j >= 0 && j < i && j > nearest {
				nearest = j
				at = max(at, k+1)
			}
		}
	}
	return at
}

func (d *canonDiff) remove(n syntax.Node) {
	d.out = append(d.out, format.Change{Kind: format.Remove, Node: n})
}

func (d *canonDiff) insert(list syntax.Tok, at int, text string) {
	d.out = append(d.out, format.Change{Kind: format.Insert, List: list, At: at, Text: text})
}
