package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// addEntryOp adds an entry to a table or a map: a key that exists is
// ErrKeyExists (E3); a table whose entries live in files gets a new file (N1).
func addEntryOp(x *opCtx) error {
	if x.a.env.EditLayer != "" {
		return x.layerAddEntry()
	}
	switch t := x.res.Target.(type) {
	case *value.Table:
		return x.addTableEntry(t)
	case *value.Map:
		return x.addMapEntry(t)
	}
	return ErrBadOp
}

// entryParts is a new entry: its canonical segment, its key, its value, and for a map entry the
// frame its value's type arguments are read in (TYPES.md 11.5) and its declared type.
type entryParts struct {
	seg Seg
	key value.Value
	v   value.Value
	fr  *depFrame
	t   types.Type
}

// newTableEntry types AddEntry's key, a word (TYPES.md §9.3), and value for table t.
func (x *opCtx) newTableEntry(t *value.Table) (entryParts, error) {
	tt, ok := present(x.targetType()).Base().(*types.TableType)
	if !ok {
		return entryParts{}, ErrBadOp
	}
	kv, err := x.key(types.StringType)
	if err != nil {
		return entryParts{}, err
	}
	key := kv.CanonText()
	switch {
	case !isWord(key):
		return entryParts{}, &ValueError{Expected: types.StringType.String(), Got: describe(x.op.Key), Detail: detailNotWord}
	case entryIndex(t.Entries, key) >= 0:
		return entryParts{}, ErrKeyExists
	}
	v, err := x.typed(x.op.Value, tt.Elem)
	if err != nil {
		return entryParts{}, err
	}
	rec, ok := v.(*value.Record)
	if !ok {
		return entryParts{}, ErrBadOp
	}
	rec.Ident = &value.Identity{Key: value.Key{S: key}}
	return entryParts{seg: entrySeg(rec.Ident.Key), key: kv, v: rec}, nil
}

func (x *opCtx) addTableEntry(t *value.Table) error {
	e, err := x.newTableEntry(t)
	if err != nil {
		return err
	}
	rec := e.v.(*value.Record)
	x.addInverse(e.seg)
	if tt, ok := t.T.Base().(*types.TableType); ok && tt.Stable && len(x.res.Steps) == 0 {
		x.w.locked = append(x.w.locked, Locked{Name: x.res.root.lockName(), Key: rec.Ident.Key.Text()})
	}
	if x.j.last().files {
		return x.newEntryFile(rec, e.key)
	}
	key := e.key.CanonText()
	grown := &value.Table{T: t.T, Entries: append(slices.Clone(t.Entries), rec), P: t.P}
	count := len(t.Entries)
	return x.insertItem(newItem{
		v: rec, grown: grown, key: key, at: count, count: count,
		text: func() (string, error) { return x.a.entryText(key, rec, false) },
	})
}

// newMapEntry types AddEntry's key and value for map m.
func (x *opCtx) newMapEntry(m *value.Map) (entryParts, error) {
	kt, vt, ok := mapTypes(present(x.targetType()))
	if !ok {
		return entryParts{}, ErrBadOp
	}
	k, err := x.key(kt)
	if err != nil {
		return entryParts{}, err
	}
	if slices.ContainsFunc(m.Keys, func(o value.Value) bool { return sameValue(o, k) }) {
		return entryParts{}, ErrKeyExists
	}
	fr := x.frameAt(len(x.j.cur))
	if dm, isDep := present(x.targetType()).Base().(*types.DepMapType); isDep {
		fr = fr.bind(dm.Binder, k) // the new key's binder (log-2026-09-29 M4 B7-r3)
	}
	x.a.outer = fr.outer()
	v, err := x.typed(x.op.Value, vt)
	if err != nil {
		return entryParts{}, err
	}
	return entryParts{seg: keySeg(k, kt), key: k, v: v, fr: &fr, t: vt}, nil
}

func (x *opCtx) addMapEntry(m *value.Map) error {
	e, err := x.newMapEntry(m)
	if err != nil {
		return err
	}
	if e.key, err = x.jsonKey(m, e.key); err != nil {
		return err
	}
	wkey, err := wireKey(e.key)
	if err != nil && x.j.last().mode == ModeJSON {
		return &ValueError{Expected: e.key.Type().String(), Got: e.key.CanonText(), Detail: err.Error()}
	}
	x.addInverse(e.seg)
	grown := &value.Map{T: m.T, Keys: append(slices.Clone(m.Keys), e.key), Vals: append(slices.Clone(m.Vals), e.v), P: m.P}
	count := len(m.Keys)
	return x.insertItem(newItem{
		v: e.v, grown: grown, key: wkey, at: count, count: count, fr: e.fr, t: e.t,
		text: func() (string, error) { return x.a.mapItemText(e.key, e.v) },
	})
}

// jsonKey is k, a new key of m, as a JSON source writes it: a symbol as what it names in its
// record's branch (DEP-02); a key m holds under that name is ErrKeyExists (E3).
func (x *opCtx) jsonKey(m *value.Map, k value.Value) (value.Value, error) {
	if _, isSymbol := k.(*value.Symbol); !isSymbol || x.j.last().mode != ModeJSON {
		return k, nil
	}
	rk, err := x.frameAt(len(x.j.cur)).symbolsIn(k, nil, nil)
	if err != nil {
		return nil, err
	}
	if slices.ContainsFunc(m.Keys, func(o value.Value) bool { return sameValue(o, rk) }) {
		return nil, ErrKeyExists
	}
	return rk, nil
}

// sibling is the collection holding the path's target and the target's position there.
func (x *opCtx) sibling() (value.Value, int, bool) {
	n := len(x.res.Steps)
	if n == 0 {
		return nil, 0, false
	}
	parent := x.res.parent(n - 1)
	switch parent.(type) {
	case *value.List, *value.Map, *value.Table:
		return parent, position(parent, x.res.Target), true
	}
	return nil, 0, false
}

// stableTable reports a target that is an entry of a stable table (API.md E4).
func (x *opCtx) stableTable() bool {
	n := len(x.res.Steps)
	if n == 0 {
		return false
	}
	tt, ok := baseOf(x.res.Steps[n-1].Container).(*types.TableType)
	return ok && tt.Stable
}

// siblingCount is how many elements or entries a collection holds.
func siblingCount(c value.Value) int {
	switch x := c.(type) {
	case *value.List:
		return len(x.Elems)
	case *value.Map:
		return len(x.Keys)
	case *value.Table:
		return len(x.Entries)
	}
	return 0
}

// parentPath is the canonical path of the collection holding the target.
func (x *opCtx) parentPath() string {
	p, _ := Parse(x.res.Canonical)
	p.Segs = p.Segs[:len(p.Segs)-1]
	return p.String()
}
