package edit

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// sourceLit is v as the literal Undo carries (API.md E23): Source, single-line (E26), the tokens
// at writes for it as written; FromJSON of its wire, in scope's rules that the inverse reads it
// in, when a JSON source states it with a decoded symbol (M4 B7-r, B10-r2).
func (a *applier) sourceLit(v value.Value, scope *types.Field, at stated) (Lit, error) {
	return a.notedLit(v, scope, a.noteSpellings(v, at))
}

// notedLit is sourceLit with the tokens notes spells as written.
func (a *applier) notedLit(v value.Value, scope *types.Field, notes spellings) (Lit, error) {
	raw, ok, err := a.readWire(v, scope)
	switch {
	case err != nil:
		return nil, err
	case ok:
		return FromJSON(raw), nil
	}
	p := &canonPrinter{a: a, notes: notes}
	p.value(v)
	if p.err != nil {
		return nil, p.err
	}
	return Source(flatText(p.b.String())), nil
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
	scope, notes := x.scopeAt(k, true), x.a.noteSpellings(v, x.statedAt(k))
	lit, err := x.a.notedLit(v, scope, notes)
	if err != nil {
		return err
	}
	if x.lockHeld(v, k) {
		lit = heldValue{old: v, scope: scope, lit: lit, notes: notes} // the edit's lock facts are kept (E23)
	}
	x.inverse(Operation{Kind: OpSet, Path: path, Value: lit})
	return nil
}

// statedAt is the .canon literal stating the value at cursor k, zero when none does: only tokens
// written there are carried as written (DECISIONS 337).
func (x *opCtx) statedAt(k int) stated {
	if k >= len(x.j.cur) {
		return stated{}
	}
	return statedBy(x.j.cur[k])
}

// lockHeld reports v, at step k of the path, a root stable table or one of its entries, whose
// write back keeps the edit's lock facts (API.md E23).
func (x *opCtx) lockHeld(v value.Value, k int) bool {
	switch v.(type) {
	case *value.Table:
		return k == 0 && stableTable(v.Type())
	case *value.Record:
		return k == 1 && x.stableTable()
	}
	return false
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
	k := len(x.res.Steps)
	lit, err := x.a.sourceLit(x.res.Target, x.scopeAt(k, true), x.statedAt(k))
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
