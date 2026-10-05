package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// addSource adds an entry, as addTableEntry does: its own file placed by the op's literal when
// the table's entries get files (N1, N2), else at the end of its literal. Its inverse is a
// Remove, or in a stable table a Retire of its new id (API.md E3, E23).
func addSource(t *srcTable, _ Path) error {
	x := t.x
	kv, err := x.key(types.StringType)
	if err != nil {
		return err
	}
	key := kv.CanonText()
	switch {
	case !isWord(key):
		return &ValueError{Expected: types.StringType.String(), Got: describe(x.op.Key), Detail: detailNotWord}
	case slices.ContainsFunc(t.entries, func(it item) bool { return statedKey(it.node) == key }):
		return ErrKeyExists
	}
	v, _, err := typedText(x, x.op.Value, t.tt.Elem, nil)
	if err != nil {
		return err
	}
	rec, isRec := v.(*value.Record)
	if !isRec {
		return ErrBadOp
	}
	rec.Ident = &value.Identity{Key: value.Key{S: key}}
	path := childPath(x.res.root.qualified(), entrySeg(rec.Ident.Key))
	if t.tt.Stable {
		x.inverse(Operation{Kind: OpRetire, Path: path}) // its locked id is never removed (API.md E4, E23)
		x.w.locked = append(x.w.locked, Locked{Name: x.res.root.lockName(), Key: key})
		x.a.held[path] = true
	} else {
		x.inverse(Operation{Kind: OpRemove, Path: path})
	}
	if t.filed {
		return x.newEntryFile(rec, kv)
	}
	text, err := x.a.entryText(key, rec, false)
	if err != nil {
		return err
	}
	x.w.addCanon(x.res.root.obj.File().Src.Path, x.res.root.pkg.Path,
		[]format.Change{{Kind: format.Insert, List: t.lit.First(), At: len(t.lit.Items), Text: text}})
	return nil
}

// retireSource retires an entry of a stable table: `retired ` before its key (API.md N7). It has
// no inverse (E4); an entry already retired stays as it is.
func retireSource(t *srcTable, p Path) error {
	if err := t.entry(p); err != nil {
		return err
	}
	switch {
	case !t.tt.Stable:
		return ErrBadOp
	case retiredItem(t.node().node):
		return nil
	}
	t.x.w.locked = append(t.x.w.locked, Locked{Name: t.x.res.root.lockName(), Key: t.key})
	t.write(format.Change{Kind: format.Retire, Node: t.node().node})
	return nil
}

// setRootSource writes the whole of a root with no value that is no table: its initializer
// replaced by the op's value; its inverse sets the old text back, which must be a literal
// (API.md E19, E23).
func setRootSource(x *opCtx, cause error) error {
	let := x.res.root.obj.Decl().(*syntax.LetDecl)
	f, ty := x.res.root.obj.File(), x.res.root.obj.Type()
	old, ok := literalText(x, item{let.Value, f}, ty, nil)
	if !ok {
		return cause
	}
	_, text, err := typedText(x, x.op.Value, ty, nil)
	if err != nil {
		return err
	}
	x.inverse(Operation{Kind: OpSet, Path: x.res.Canonical, Value: old})
	x.w.addCanon(f.Src.Path, x.res.root.pkg.Path, []format.Change{{Kind: format.Replace, Node: let.Value, Text: text}})
	return nil
}
