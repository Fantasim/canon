package edit

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// removeOp removes a list element, a map entry or a table entry whose id is not locked (API.md
// E4): its item with its comments (M4), or its own file (N6).
func removeOp(x *opCtx) error {
	parent, pos, ok := x.sibling()
	switch {
	case !ok:
		return ErrBadOp
	case x.lockedKey():
		return ErrStableKey
	case x.a.env.EditLayer != "":
		return x.layerRemove(parent, pos)
	case x.pairsAt(len(x.res.Steps) - 1):
		return x.pairsOp(len(x.res.Steps)-1, without(parent, pos)) // slot keys renumbered (WIRE.md 5.14)
	}
	if err := x.undoRemove(parent, pos); err != nil {
		return err
	}
	c, pc := x.j.last(), x.j.holder()
	switch {
	case pc.files && c.mode == ModeJSON:
		return x.removeFile(x.jsonFileOf(), packageDir(x.res.root.pkg))
	case c.state != stTree:
		return x.change(without(parent, pos))
	case c.mode == ModeJSON && x.writtenWhole(parent):
		return x.diffAt(len(x.res.Steps)-1, parent, without(parent, pos))
	case c.mode == ModeJSON:
		return x.removeJSON()
	}
	return x.removeCanon(c, pc)
}

// writtenWhole reports a collection its JSON source writes as one value, not item by item: a
// bits list is one number (WIRE.md 5.3); an operation on an item writes it again whole (M1).
func (x *opCtx) writtenWhole(c value.Value) bool {
	n, _, err := x.jsonAt(c)
	return err == nil && n.Kind != jsonsrc.Array && n.Kind != jsonsrc.Object
}

// moved is l with its element at from moved to position to, counted after its removal.
func moved(l *value.List, from, to int) *value.List {
	elems := deleted(l.Elems, from)
	return &value.List{T: l.T, Elems: slices.Insert(elems, to, l.Elems[from]), P: l.P}
}

// without is collection c without its item at pos.
func without(c value.Value, pos int) value.Value {
	switch x := c.(type) {
	case *value.List:
		return &value.List{T: x.T, Elems: deleted(x.Elems, pos), P: x.P}
	case *value.Map:
		return &value.Map{T: x.T, Keys: deleted(x.Keys, pos), Vals: deleted(x.Vals, pos), P: x.P}
	case *value.Table:
		return &value.Table{T: x.T, Entries: deleted(x.Entries, pos), P: x.P}
	}
	return c
}

func deleted[V any](vs []V, i int) []V {
	out := make([]V, 0, len(vs))
	out = append(out, vs[:i]...)
	return append(out, vs[i+1:]...)
}

// removeCanon removes the item stating the target from the parent's literal, or the entry
// declaration, whose file goes when it declares nothing else (N6).
func (x *opCtx) removeCanon(c, pc cursor) error {
	if d, ok := c.node.(*syntax.EntryDecl); ok {
		if len(c.file.Decls) == 1 && len(c.file.Amends) == 0 {
			return x.removeFile(c.file.Src.Path, packageDir(x.res.root.pkg))
		}
		x.w.addCanon(c.file.Src.Path, x.res.root.pkg.Path, []format.Change{{Kind: format.Remove, Node: d}})
		return nil
	}
	item := itemOf(pc.node, c.node)
	if item == nil {
		return &NotEditableError{Reason: ReasonComputed}
	}
	x.w.addCanon(pc.file.Src.Path, x.res.root.pkg.Path, []format.Change{{Kind: format.Remove, Node: item}})
	return nil
}

// itemOf is the item of literal lit whose value node is n: a list element, a map or field
// item, a table entry.
func itemOf(lit, n syntax.Node) syntax.Node {
	if l, ok := lit.(*syntax.ListLit); ok {
		for _, e := range l.Elems {
			if syntax.Unparen(e) == n {
				return e
			}
		}
		return nil
	}
	b := braceOf(lit)
	if b == nil {
		return nil
	}
	for _, it := range b.Items {
		if itemValue(it) == n {
			return it
		}
	}
	return nil
}

// itemValue is the node a brace item states its value with: a field's or map item's value, a
// table entry itself.
func itemValue(it syntax.BraceItem) syntax.Node {
	switch x := it.(type) {
	case *syntax.MapItem:
		return syntax.Unparen(x.Value)
	case *syntax.FieldItem:
		return syntax.Unparen(x.Value)
	case *syntax.EntryItem:
		return x
	}
	return nil
}

// removeJSON removes the member or element holding the target from its object or array (§14.2).
func (x *opCtx) removeJSON() error {
	n, display, err := x.jsonItem()
	if err != nil {
		return err
	}
	d := &jsonDiff{a: x.a}
	d.remove(n, int(n.Span.Start))
	x.w.addJSON(display, x.res.root.pkg.Path, d.out)
	return nil
}

// jsonFileOf is the JSON source the target is the whole of: a load.dir element's file.
func (x *opCtx) jsonFileOf() string {
	if p := provOf(x.res.Target); p != nil {
		return x.a.snap.display(p.Span.File)
	}
	return ""
}

// removeFile deletes the file at display; stop is the package directory the directories it
// leaves empty are removed below, "" for none (N6).
func (x *opCtx) removeFile(display, stop string) error {
	if display == "" {
		return &NotEditableError{Reason: ReasonComputed}
	}
	if _, err := x.a.state(display); err != nil {
		return err
	}
	x.w.removes = append(x.w.removes, removal{display: display, stop: stop})
	x.w.own(display, x.res.root.pkg.Path)
	return nil
}

// undoRemove is the inverse of a Remove (E23): Insert at the old position, or AddEntry then
// Move back; into a collection its files order, Add or AddEntry alone, placed by N1.
func (x *opCtx) undoRemove(parent value.Value, pos int) error {
	lit, err := x.a.sourceLit(x.res.Target, x.scopeAt(len(x.res.Steps), false)) // read back as an item
	if err != nil {
		return err
	}
	pp := x.parentPath()
	switch p := parent.(type) {
	case *value.List:
		if n, ordered := literalCount(x.j.holder(), parent); ordered && pos < n {
			x.inverse(Operation{Kind: OpInsert, Path: pp, Index: pos, Value: lit}) // the literal orders it (N1)
		} else {
			x.inverse(Operation{Kind: OpAdd, Path: pp, Value: lit})
		}
		return nil
	case *value.Table:
		x.inverse(Operation{Kind: OpAddEntry, Path: pp, Key: entryKeyLit(entryKey(p.Entries[pos])), Value: lit})
	case *value.Map:
		x.inverse(Operation{Kind: OpAddEntry, Path: pp, Key: keyLit(p.Keys[pos]), Value: lit})
	}
	if n, ordered := literalCount(x.j.holder(), parent); ordered && pos < n-1 && x.a.env.EditLayer == "" { // a layer has no Move (W5 layer)
		x.inverse(Operation{Kind: OpMove, Path: x.res.Canonical, Index: pos})
	}
	return nil
}

// literalCount is how many items of parent its holder pc's literal or JSON container orders:
// all, or beside entries declared in the let's own file, which follow them (W2), the
// literal's; false when file paths order them (API.md N1).
func literalCount(pc cursor, parent value.Value) (int, bool) {
	n := siblingCount(parent)
	switch {
	case !pc.files:
		return n, true
	case pc.newFile || pc.mode != ModeCanon:
		return 0, false
	}
	return n - len(pc.entries), true
}

// moveOp moves an element or entry to position i among its siblings, counted after its
// removal; the siblings must share one literal or JSON container (API.md E9).
func moveOp(x *opCtx) error {
	parent, pos, ok := x.sibling()
	if !ok {
		return ErrBadOp
	}
	n := siblingCount(parent)
	switch {
	case x.op.Index < 0:
		return &PathError{Seg: len(x.res.Steps), Err: ErrBadPath}
	case x.op.Index >= n:
		return &PathError{Seg: len(x.res.Steps), Err: ErrNoPath}
	case x.op.Index == pos:
		return nil
	}
	c, pc := x.j.last(), x.j.holder()
	if lit, ordered := literalCount(pc, parent); c.state != stTree || pc.state != stTree || !ordered || pos >= lit || x.op.Index >= lit {
		return &NotEditableError{Reason: ReasonOrder}
	}
	l, isList := parent.(*value.List)
	if isList && x.pairsAt(len(x.res.Steps)-1) {
		return x.pairsOp(len(x.res.Steps)-1, moved(l, pos, x.op.Index)) // slot keys (WIRE.md 5.14)
	}
	x.inverse(Operation{Kind: OpMove, Path: x.movedPath(parent), Index: pos})
	if isList && c.mode == ModeJSON && x.writtenWhole(parent) {
		return x.diffAt(len(x.res.Steps)-1, parent, moved(l, pos, x.op.Index))
	}
	if c.mode == ModeJSON {
		return x.moveJSON(pos, x.op.Index)
	}
	return x.moveCanon(c, pc, pos, x.op.Index)
}

// movedPath is the target's canonical path once moved: a plain list element's index changes.
func (x *opCtx) movedPath(parent value.Value) string {
	if l, ok := parent.(*value.List); ok {
		if lt, isList := l.T.Base().(*types.ListType); isList && lt.KeyedBy == nil {
			return childPath(x.parentPath(), indexSeg(x.op.Index))
		}
	}
	return x.res.Canonical
}

// moveCanon moves the item, with its comments, among the original items: before the item at i,
// or i+1 past its own place (API-03, M4; log-2026-09-29 M4 U1b).
func (x *opCtx) moveCanon(c, pc cursor, from, to int) error {
	item := itemOf(pc.node, c.node)
	list, ok := listToken(pc.node)
	if item == nil || !ok || itemCount(pc.node) != literalCountOf(x, pc) {
		return &NotEditableError{Reason: ReasonOrder}
	}
	at := to
	if to > from {
		at = to + 1
	}
	x.w.addCanon(pc.file.Src.Path, x.res.root.pkg.Path, []format.Change{{Kind: format.Move, Node: item, List: list, At: at}})
	return nil
}

// literalCountOf is literalCount of the target's collection.
func literalCountOf(x *opCtx, pc cursor) int {
	parent, _, _ := x.sibling()
	n, _ := literalCount(pc, parent)
	return n
}

// moveJSON removes the member or element, then inserts it at its new place counted after the
// removal, as Move counts it (FMT-02): the removal applies first, whatever the offsets.
func (x *opCtx) moveJSON(from, to int) error {
	n, display, err := x.jsonItem()
	if err != nil {
		return err
	}
	holder, err := x.a.jsonHolder(display, n)
	if err != nil {
		return err
	}
	key := ""
	if holder.Kind == jsonsrc.Object {
		i := memberOf(holder, n)
		if i < 0 || i != from {
			return errNoMember
		}
		key = holder.Members[i].Key
	}
	x.w.addJSON(display, x.res.root.pkg.Path, []jsonEdit{
		{e: jsonsrc.Edit{Kind: jsonsrc.Remove, Pointer: n.Pointer()}, anchor: 1},
		{e: jsonsrc.Edit{Kind: jsonsrc.Insert, Pointer: holder.Pointer(), Key: key, At: to, Value: detached(n)}, insert: true},
	})
	return nil
}

// jsonHolder is the object or array holding n in the JSON source at display.
func (a *applier) jsonHolder(display string, n *jsonsrc.Node) (*jsonsrc.Node, error) {
	root, err := a.jsonRoot(display)
	if err != nil {
		return nil, err
	}
	ptr := n.Pointer()
	holder := root.Find(ptr[:strings.LastIndex(ptr, pointerSep)])
	if holder == nil {
		return nil, errNoTree
	}
	return holder, nil
}
