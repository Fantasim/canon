package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// item is a node stating one child of a source tree, and the file holding it.
type item struct {
	node syntax.Node
	file *syntax.File
}

// items are the children a literal states, in source order: list elements, field and map
// values, table entries (W1); a spread states none of its own.
func items(n syntax.Node, f *syntax.File) []item {
	if l, ok := n.(*syntax.ListLit); ok {
		out := make([]item, len(l.Elems))
		for i, e := range l.Elems {
			out[i] = item{syntax.Unparen(e), f}
		}
		return out
	}
	lit := braceOf(n)
	if lit == nil {
		return nil
	}
	out := make([]item, 0, len(lit.Items))
	for _, it := range lit.Items {
		switch x := it.(type) {
		case *syntax.FieldItem:
			out = append(out, item{syntax.Unparen(x.Value), f})
		case *syntax.MapItem:
			out = append(out, item{syntax.Unparen(x.Value), f})
		case *syntax.EntryItem:
			out = append(out, item{x, f})
		}
	}
	return out
}

// braceOf is the brace literal a node states a record, table or map with.
func braceOf(n syntax.Node) *syntax.BraceLit {
	switch x := n.(type) {
	case *syntax.BraceLit:
		return x
	case *syntax.TypedLit:
		return x.Lit
	case *syntax.EntryItem:
		return x.Value
	case *syntax.EntryDecl:
		return x.Value
	}
	return nil
}

// fieldItem is the item giving field name in a record literal, nil when the field is absent.
func fieldItem(n syntax.Node, name string) *syntax.FieldItem {
	lit := braceOf(n)
	if lit == nil {
		return nil
	}
	for _, it := range lit.Items {
		if fi, ok := it.(*syntax.FieldItem); ok && fi.Name.Name == name {
			return fi
		}
	}
	return nil
}

// entryItem is the entry key of a table literal, nil when the literal has none.
func entryItem(n syntax.Node, key string) *syntax.EntryItem {
	lit := braceOf(n)
	if lit == nil {
		return nil
	}
	for _, it := range lit.Items {
		if ei, ok := it.(*syntax.EntryItem); ok && ei.Key.Name == key {
			return ei
		}
	}
	return nil
}

// hasSpread reports a record literal with a `...e` item: it supplies every field not written (W3).
func hasSpread(n syntax.Node) bool {
	lit := braceOf(n)
	if lit == nil {
		return false
	}
	for _, it := range lit.Items {
		if _, ok := it.(*syntax.SpreadItem); ok {
			return true
		}
	}
	return false
}

// entryDecls are the `entry` declarations of a root's collection, in the order evaluation
// appends them (W2).
func (s *Snapshot) entryDecls(r rootRef) []item {
	return slices.Clip(s.decls().entries[r.obj])
}

// filesLet reports a let with `@files`: its entries live in files, ordered by their paths, even
// before it has any.
func filesLet(n syntax.Node) bool {
	d, ok := n.(*syntax.LetDecl)
	if !ok {
		return false
	}
	for _, a := range d.Annotations {
		if a.Name != nil && a.Name.Name == syntax.AnnFiles {
			return true
		}
	}
	return false
}

// position is the index of v among the elements or map values of parent, -1 when it is none.
func position(parent, v value.Value) int {
	var elems []value.Value
	switch p := parent.(type) {
	case *value.List:
		elems = p.Elems
	case *value.Map:
		elems = p.Vals
	case *value.Table:
		for i, e := range p.Entries {
			if value.Value(e) == v {
				return i
			}
		}
	}
	for i, e := range elems {
		if e == v {
			return i
		}
	}
	return -1
}

// provOf is v's provenance, nil for no value.
func provOf(v value.Value) *value.Prov {
	if v == nil {
		return nil
	}
	return v.Prov()
}
