package edit

import "github.com/fantasim/canonlang/internal/value"

// sourceLit is v as the Source literal Undo carries (API.md E23): canonical, single-line (E26).
func (a *applier) sourceLit(v value.Value) (Lit, error) {
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
	lit, err := x.a.sourceLit(x.res.Target)
	if err != nil {
		return err
	}
	x.inverse(Operation{Kind: OpSet, Path: x.res.Canonical, Value: lit})
	return nil
}

// inverse records an operation that undoes part of this one (E22).
func (x *opCtx) inverse(op Operation) {
	x.w.undo = append(x.w.undo, op)
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
