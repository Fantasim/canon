package edit

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// sourceLit is v as the literal Undo carries (API.md E23): Source, canonical and single-line
// (E26); FromJSON of its wire, in scope's rules that the inverse reads it in, when a JSON source
// states it with a decoded symbol (M4 B7-r, B10-r2).
func (a *applier) sourceLit(v value.Value, scope *types.Field) (Lit, error) {
	raw, ok, err := a.readWire(v, scope)
	switch {
	case err != nil:
		return nil, err
	case ok:
		return FromJSON(raw), nil
	}
	text, err := a.canonText(v)
	if err != nil {
		return nil, err
	}
	return Source(flatText(text)), nil
}

// undoSet is the inverse of a Set or Reset of the path (E23): Reset when the field was left to
// its default, else Set to the old value.
func (x *opCtx) undoSet() error {
	if rec, f, ok := x.field(); ok && !rec.Set[f.Index] && x.j.last().state == stAbsent && hasDefault(f) {
		x.inverse(Operation{Kind: OpReset, Path: x.res.Canonical})
		return nil
	}
	path, v, k := x.res.Canonical, x.res.Target, len(x.res.Steps)
	if x.inWhole(k) {
		k--
		path, v = x.pathAt(k), x.valueAt(k) // an element of a bits list, which keeps no order: the list (WIRE.md 5.3)
	}
	lit, err := x.a.sourceLit(v, x.scopeAt(k, true))
	if err != nil {
		return err
	}
	x.inverse(Operation{Kind: OpSet, Path: path, Value: lit})
	return nil
}

// inverse records an operation that undoes part of this one (E22).
func (x *opCtx) inverse(op Operation) {
	x.w.undo = append(x.w.undo, op)
}

// addInverse is the inverse of adding item seg to the target collection (E23): Remove of the
// item, or Reset of the collection when its default supplied it, which the operation wrote
// (W8), so Undo leaves it absent again (E22; log-2026-09-29 M4 B7-r4).
func (x *opCtx) addInverse(seg Seg) {
	if _, f, ok := x.field(); ok && hasDefault(f) && x.j.last().state == stAbsent {
		x.inverse(Operation{Kind: OpReset, Path: x.res.Canonical})
		return
	}
	x.inverse(Operation{Kind: OpRemove, Path: childPath(x.res.Canonical, seg)})
}

// wholeInverse is the inverse of adding an item to a collection its JSON source writes whole,
// which keeps no position: Set of the collection as it was (E23; WIRE.md 5.3).
func (x *opCtx) wholeInverse() error {
	lit, err := x.a.sourceLit(x.res.Target, x.scopeAt(len(x.res.Steps), true))
	if err != nil {
		return err
	}
	x.w.undo = []Operation{{Kind: OpSet, Path: x.res.Canonical, Value: lit}}
	return nil
}

// keyLit is a key value as an operation's key: an integer as IntKey, any other as the path key
// of its canonical text (API.md E25, P1, P2).
func keyLit(k value.Value) Lit {
	switch x := k.(type) {
	case *value.Int:
		return IntKey(x.V)
	case *value.Ref:
		if x.Key.IsInt {
			return IntKey(x.Key.I)
		}
		return PathKey(x.Key.S)
	case *value.Str:
		return PathKey(x.V)
	}
	return PathKey(k.CanonText())
}

// entryKeyLit is a table entry's key as an operation's key.
func entryKeyLit(k value.Key) Lit {
	if k.IsInt {
		return IntKey(k.I)
	}
	return PathKey(k.S)
}

// childPath is the canonical path of the child seg of the value at path.
func childPath(path string, seg Seg) string {
	p := Path{Segs: []Seg{seg}}
	return path + p.String()
}

// indexSeg is `[n]`.
func indexSeg(n int) Seg {
	return intSeg(int64(n))
}
