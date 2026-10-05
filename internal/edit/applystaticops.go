package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// node is the named entry's item or declaration, and its file.
func (t *srcTable) node() item { return t.entries[t.pos] }

// write adds change to the named entry's file.
func (t *srcTable) write(ch format.Change) {
	t.x.w.addCanon(t.node().file.Src.Path, t.x.res.root.pkg.Path, []format.Change{ch})
}

// oldText is the text of n in the named entry's file, which an inverse carries back; the root's
// refusal when it is no literal (E19).
func (t *srcTable) oldText(n syntax.Node, ty types.Type, scope *types.Field) (Source, error) {
	text, ok := literalText(t.x, item{n, t.node().file}, ty, scope)
	if !ok {
		return "", t.cause
	}
	return text, nil
}

// literalText is the source text of it, false when it is no literal of ty in API.md 8.2's
// sense: an inverse carrying it could not apply (E19, E22).
func literalText(x *opCtx, it item, ty types.Type, scope *types.Field) (Source, bool) {
	sp := it.file.Span(it.node)
	text := Source(it.file.Src.Content[sp.Start:sp.End])
	ctx := x.a.typer(x.res.root.pkg.Path)
	ctx.marks, ctx.scope, ctx.Outer = nil, scope, wire.Outer{} // a check: no symbol it types is kept
	_, err := ctx.Value(x.a.ctx, text, ty)
	return text, err == nil
}

// typedText is lit typed as a value of ty in scope's wire rules (V1), and that value as a Canon
// literal (M7).
func typedText(x *opCtx, lit Lit, ty types.Type, scope *types.Field) (value.Value, string, error) {
	if lit == nil {
		return nil, "", ErrBadOp
	}
	if err := utf8Text(lit, ty); err != nil {
		return nil, "", err
	}
	x.a.outer = wire.Outer{}
	typer := x.a.typer(x.res.root.pkg.Path)
	typer.scope = scope
	v, err := typer.Value(x.a.ctx, lit, ty)
	if err != nil {
		return nil, "", err
	}
	text, err := x.a.canonText(v)
	return v, text, err
}

// stableHeld reports the table stable and key's id held by the lock or locked by the request (E4).
func (t *srcTable) stableHeld(key string) bool {
	id := Locked{Name: t.x.res.root.lockName(), Key: key}
	return t.tt.Stable && (slices.Contains(t.x.a.locked, id) || t.x.a.snap.a.LockHolds(build.LockID{Name: id.Name, Key: id.Key}))
}

// lockSet records the named entry's id for its lock lines, as a Set or Reset of a stable entry
// the lock does not hold yet does (API.md E20).
func (t *srcTable) lockSet() {
	id := Locked{Name: t.x.res.root.lockName(), Key: t.key}
	if t.tt.Stable && !t.x.a.snap.a.LockHolds(build.LockID{Name: id.Name, Key: id.Key}) {
		t.x.w.locked = append(t.x.w.locked, id)
	}
}

// removeSource removes the entry's item or declaration, as removeCanon does, E4 judged first. Its
// inverse adds it back with its source text, then moves it to its place among the literal's
// entries, then retires it again when it was (API.md E4, E23, N6).
func removeSource(t *srcTable, p Path) error {
	if err := t.entry(p); err != nil {
		return err
	}
	if t.stableHeld(t.key) {
		return ErrStableKey
	}
	x, n := t.x, t.node()
	text, err := t.oldText(braceOf(n.node), t.tt.Elem, nil)
	if err != nil {
		return err
	}
	x.inverse(Operation{Kind: OpAddEntry, Path: x.res.root.qualified(), Key: Key(t.key), Value: text})
	if !t.filed && t.pos < t.litCount-1 {
		x.inverse(Operation{Kind: OpMove, Path: x.res.Canonical, Index: t.pos})
	}
	if retiredItem(n.node) && t.tt.Stable {
		x.inverse(Operation{Kind: OpRetire, Path: x.res.Canonical})
	}
	if _, ok := n.node.(*syntax.EntryDecl); ok && len(n.file.Decls) == 1 && len(n.file.Amends) == 0 {
		return x.removeFile(n.file.Src.Path, packageDir(x.res.root.pkg))
	}
	t.write(format.Change{Kind: format.Remove, Node: n.node})
	return nil
}

// setFieldSource writes a field of the entry: its value replaced, or inserted where W7 places
// it. Its inverse is a Set of the old text, a Reset, or for a required field the entry lacked a
// Set of the whole entry to its old text (API.md E2, E23; DECISIONS 257, 309).
func setFieldSource(t *srcTable, p Path) error {
	i, f, err := t.field(p)
	if err != nil {
		return err
	}
	_, text, err := typedText(t.x, t.x.op.Value, f.Type, f)
	if err != nil {
		return err
	}
	brace, path := braceOf(t.node().node), childPath(t.x.res.Canonical, Seg{Kind: SegField, Name: f.Name})
	inv := Operation{Kind: OpReset, Path: path}
	ch := format.Change{Kind: format.Insert, List: brace.First(), At: w7Position(brace, fieldsOf(t.tt.Elem), i), Text: f.Name + colonSp + text}
	var old Source
	switch fi := fieldItem(brace, f.Name); {
	case fi != nil:
		old, err = t.oldText(fi.Value, f.Type, f)
		inv, ch = Operation{Kind: OpSet, Path: path, Value: old}, format.Change{Kind: format.Replace, Node: fi.Value, Text: text}
	case !hasDefault(f):
		old, err = t.oldText(brace, t.tt.Elem, nil)
		inv = Operation{Kind: OpSet, Path: t.x.res.Canonical, Value: old}
	}
	if err != nil {
		return err
	}
	t.x.inverse(inv)
	t.write(ch)
	t.lockSet()
	return nil
}

// resetSource removes a field that has a default from the entry (API.md E2, E8); its inverse
// sets its old text back.
func resetSource(t *srcTable, p Path) error {
	_, f, err := t.field(p)
	switch {
	case err != nil:
		return err
	case !hasDefault(f):
		return ErrBadOp
	}
	fi := fieldItem(braceOf(t.node().node), f.Name)
	if fi == nil {
		return nil
	}
	old, err := t.oldText(fi.Value, f.Type, f)
	if err != nil {
		return err
	}
	t.x.inverse(Operation{Kind: OpSet, Path: childPath(t.x.res.Canonical, Seg{Kind: SegField, Name: f.Name}), Value: old})
	t.write(format.Change{Kind: format.Remove, Node: fi})
	t.lockSet()
	return nil
}

// setEntrySource writes the whole entry, its key kept (E5); its inverse sets its old text back.
func setEntrySource(t *srcTable, p Path) error {
	if err := t.entry(p); err != nil {
		return err
	}
	brace := braceOf(t.node().node)
	old, err := t.oldText(brace, t.tt.Elem, nil)
	if err != nil {
		return err
	}
	_, text, err := typedText(t.x, t.x.op.Value, t.tt.Elem, nil)
	if err != nil {
		return err
	}
	t.x.inverse(Operation{Kind: OpSet, Path: t.x.res.Canonical, Value: old})
	t.write(format.Change{Kind: format.Replace, Node: brace, Text: text})
	t.lockSet()
	return nil
}

// moveSource moves the entry among its literal's entries, as moveCanon does; an entry declared
// apart, or a table whose entries get files (N1), keeps its order (API.md E9, W5 order).
func moveSource(t *srcTable, p Path) error {
	if err := t.entry(p); err != nil {
		return err
	}
	to := t.x.op.Index
	switch {
	case to < 0:
		return &PathError{Seg: 1, Err: ErrBadPath}
	case to >= len(t.entries):
		return &PathError{Seg: 1, Err: ErrNoPath}
	case to == t.pos:
		return nil
	case t.filed || t.pos >= t.litCount || to >= t.litCount || len(t.lit.Items) != t.litCount:
		return &NotEditableError{Reason: ReasonOrder}
	}
	t.x.inverse(Operation{Kind: OpMove, Path: t.x.res.Canonical, Index: t.pos})
	at := to
	if to > t.pos {
		at = to + 1
	}
	t.write(format.Change{Kind: format.Move, Node: t.node().node, List: t.lit.First(), At: at})
	return nil
}
