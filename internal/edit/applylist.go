package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// addOp appends an element to a list (API.md §8.3).
func addOp(x *opCtx) error {
	l, ok := x.res.Target.(*value.List)
	if !ok {
		return ErrBadOp
	}
	return x.addElem(l, len(l.Elems))
}

// insertOp inserts an element at a position 0 to the list's length (API.md §8.3, P4).
func insertOp(x *opCtx) error {
	l, ok := x.res.Target.(*value.List)
	switch {
	case !ok:
		return ErrBadOp
	case x.op.Index < 0:
		return &PathError{Seg: len(x.res.Steps), Err: ErrBadPath}
	case x.op.Index > len(l.Elems):
		return &PathError{Seg: len(x.res.Steps), Err: ErrNoPath}
	}
	return x.addElem(l, x.op.Index)
}

// addElem adds v at position at: a keyed list takes its key from v (E3); a collection whose
// entries live in files gets a new file (N1), any other its literal a new item (M3).
func (x *opCtx) addElem(l *value.List, at int) error {
	lt, ok := present(x.targetType()).Base().(*types.ListType)
	if !ok {
		return ErrBadOp
	}
	v, err := x.typed(x.op.Value, lt.Elem)
	if err != nil {
		return err
	}
	seg := indexSeg(at)
	if lt.KeyedBy != nil {
		key := keyField(v, lt.KeyedBy)
		if key != nil && hasElemKey(l, lt.KeyedBy, key) {
			return ErrKeyExists
		}
		if key != nil {
			seg = keySeg(key, lt.KeyedBy.Type)
		}
		if rec, isRec := v.(*value.Record); isRec && x.j.last().files {
			x.inverse(Operation{Kind: OpRemove, Path: childPath(x.res.Canonical, seg)})
			return x.newEntryFile(rec, key)
		}
	}
	x.inverse(Operation{Kind: OpRemove, Path: childPath(x.res.Canonical, seg)})
	grown := &value.List{T: l.T, Elems: slices.Insert(slices.Clone(l.Elems), at, v), P: l.P}
	return x.insertItem(newItem{v: v, grown: grown, at: at, count: len(l.Elems), text: func() (string, error) { return x.a.canonText(v) }})
}

// hasElemKey reports an element of l whose key is key.
func hasElemKey(l *value.List, kf *types.Field, key value.Value) bool {
	for _, e := range l.Elems {
		if sameValue(keyField(e, kf), key) {
			return true
		}
	}
	return false
}

// newItem is an item an operation adds: its value, the collection with it, its key in a JSON
// object, its position among the count items of the collection, its text as a Canon item.
type newItem struct {
	v, grown  value.Value
	key       string
	at, count int
	text      func() (string, error)
	fr        *depFrame  // the frame its symbols are read in, nil for the collection's (DEP-02)
	t         types.Type // its declared type, nil for its own
}

// insertItem inserts a new item into the literal or JSON container stating the target
// collection (API.md M3); a collection a default or a spread supplies is written whole with
// the item, as W7 and W8 insert a field.
func (x *opCtx) insertItem(it newItem) error {
	c := x.j.last()
	switch {
	case c.files:
		return &NotEditableError{Reason: ReasonOrder}
	case c.state == stAbsent || c.state == stSpread:
		return x.change(it.grown)
	case c.mode == ModeCanon:
		return x.insertCanon(c, it)
	}
	n, display, err := x.jsonAt(x.res.Target)
	if err != nil {
		return err
	}
	fr := x.frameAt(len(x.j.cur))
	if it.fr != nil {
		fr = *it.fr
	}
	v, err := fr.symbolsIn(it.v, nil, it.t)
	if err != nil {
		return err
	}
	node, err := x.a.wireNode(v, itemScope(x.fieldAt(len(x.j.cur)-1)))
	if err != nil {
		return err
	}
	d := &jsonDiff{a: x.a}
	d.insert(n, it.at, it.key, node)
	x.w.addJSON(display, x.res.root.pkg.Path, d.out)
	return nil
}

func (x *opCtx) insertCanon(c cursor, it newItem) error {
	t, err := it.text()
	if err != nil {
		return err
	}
	list, ok := listToken(c.node)
	if !ok || itemCount(c.node) != it.count {
		return &NotEditableError{Reason: ReasonComputed}
	}
	d := &canonDiff{a: x.a}
	d.insert(list, it.at, t)
	x.w.addCanon(c.file.Src.Path, x.res.root.pkg.Path, d.out)
	return nil
}

// listToken is the opening bracket of the list literal n states its items in.
func listToken(n syntax.Node) (syntax.Tok, bool) {
	if l, ok := n.(*syntax.ListLit); ok {
		return l.First(), true
	}
	if lit := braceOf(n); lit != nil {
		return lit.First(), true
	}
	return syntax.NoTok, false
}

// itemCount is how many items the literal or container stating the target holds.
func itemCount(n syntax.Node) int {
	if l, ok := n.(*syntax.ListLit); ok {
		return len(l.Elems)
	}
	if lit := braceOf(n); lit != nil {
		return len(lit.Items)
	}
	return 0
}
