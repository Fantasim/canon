package edit

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// childNode is the node a literal states for step st: a field, a table entry, a map value, or a
// list element (by index, or by key field in a keyed list).
func childNode(n syntax.Node, st Step) (syntax.Node, bool) {
	switch st.Seg.Kind {
	case SegField:
		if fi := fieldItem(n, st.Seg.Name); fi != nil {
			return unparen(fi.Value), true
		}
		if ei := entryItem(n, st.Seg.Name); ei != nil {
			return ei, true
		}
	case SegKey:
		if l, ok := n.(*syntax.ListLit); ok {
			return listChild(l, st)
		}
		return mapChild(n, st.Seg)
	case SegPos:
	}
	return nil, false
}

// listChild is a list literal's element for step st.
func listChild(l *syntax.ListLit, st Step) (syntax.Node, bool) {
	lt, _ := baseOf(st.Container).(*types.ListType)
	if lt == nil || lt.KeyedBy == nil {
		i := st.Seg.Key.Int
		if st.Seg.Key.Kind != KeyInt || i < 0 || i >= int64(len(l.Elems)) {
			return nil, false
		}
		return unparen(l.Elems[i]), true
	}
	for _, e := range l.Elems { // a Canon literal writes an enum key as its member name, never its wire value
		el := unparen(e)
		if fi := fieldItem(el, lt.KeyedBy.Name); fi != nil && keyExprIs(unparen(fi.Value), st.Seg) {
			return el, true
		}
	}
	return nil, false
}

// mapChild is a map literal's value for the canonical key s.
func mapChild(n syntax.Node, s Seg) (syntax.Node, bool) {
	lit := braceOf(n)
	if lit == nil {
		return nil, false
	}
	for _, it := range lit.Items {
		switch x := it.(type) {
		case *syntax.MapItem:
			if keyExprIs(unparen(x.Key), s) {
				return unparen(x.Value), true
			}
		case *syntax.FieldItem:
			if nameIs(s, x.Name.Name) {
				return unparen(x.Value), true
			}
		}
	}
	return nil, false
}
