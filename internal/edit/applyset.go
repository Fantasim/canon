package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// setOp replaces the value at the path: the key kept (E5), a default removing the
// field (E6, E7), an absent field inserted or materialized (W7-W9), the rest by M1.
func setOp(x *opCtx) error {
	if x.a.env.EditLayer != "" {
		return x.layerSet()
	}
	v, err := x.typed(x.op.Value, x.targetType())
	if err != nil {
		return err
	}
	if err := x.slotsFit(len(x.res.Steps), v); err != nil {
		return err
	}
	if v, err = x.keepKey(v); err != nil {
		return err
	}
	if err := x.undoSet(); err != nil {
		return err
	}
	x.lockStable()
	x.nw = v
	if x.setsDefault(v) {
		return x.change(nil)
	}
	return x.change(v)
}

// resetOp removes a field that has a default from its literal or object (API.md §8.3, E2, E8).
func resetOp(x *opCtx) error {
	if x.a.env.EditLayer != "" {
		return x.layerReset() // removes the layer's line, whatever the field's default (U4b-r)
	}
	rec, f, ok := x.field()
	if !ok || !hasDefault(f) {
		return ErrBadOp
	}
	if !rec.Set[f.Index] {
		return nil // E8
	}
	if err := x.undoSet(); err != nil {
		return err
	}
	x.lockStable()
	return x.change(nil)
}

// lockStable records the entry of a root stable table whose @stable field the operation sets, one
// the lock does not hold yet, for its lock lines (API.md E20; log-2026-09-29 M4 U5b-r2, U5b-r3).
func (x *opCtx) lockStable() {
	rec, f, ok := x.field()
	if !ok || !f.Stable || len(x.res.Steps) != stableDepth || rec.Ident == nil {
		return
	}
	if tt, ok := baseOf(x.res.Steps[0].Container).(*types.TableType); !ok || !tt.Stable {
		return
	}
	id := Locked{Name: x.res.root.lockName(), Key: rec.Ident.Key.Text()}
	if !x.a.snap.a.LockHolds(build.LockID{Name: id.Name, Key: id.Key}) {
		x.w.locked = append(x.w.locked, id)
	}
}

// field is the record the path's last step reads a field of, and that field.
func (x *opCtx) field() (*value.Record, *types.Field, bool) {
	n := len(x.res.Steps)
	if n == 0 || x.res.Steps[n-1].Seg.Kind != SegField {
		return nil, nil, false
	}
	rec, ok := x.res.parent(n - 1).(*value.Record)
	if !ok {
		return nil, nil, false
	}
	fields := fieldsOf(rec.T)
	i := fieldIndex(fields, x.res.Steps[n-1].Seg.Name)
	if i < 0 {
		return nil, nil, false
	}
	return rec, fields[i], true
}

// targetType is the type a value at the path must have: the declared one, or for a type a value
// computes the arm its record's fields select, else the one verification found (U4a G2).
func (x *opCtx) targetType() types.Type {
	t, err := x.a.snap.Type(x.res.Resolved)
	if err != nil || t == nil {
		return x.res.Target.Type()
	}
	if !dependent(present(t)) {
		return t
	}
	rec, f, isField := x.field()
	if isField {
		t = f.Type
	}
	if _, fromJSON := x.op.Value.(FromJSON); fromJSON {
		return t // the decoder computes the arm, a symbol on Never as reading the file (log-2026-09-29 M4 B7-r4)
	}
	if isField {
		ct, computed := concreteType(f.Type, rec)
		switch {
		case computed && present(ct).Base().Kind() == types.Never:
			return t // a name given a Never arm is refused where its symbol is resolved (V1)
		case computed:
			return ct // the arm the record's fields select now (log-2026-09-29 M4 U4b-r)
		}
	}
	if x.res.Target == nil {
		return t
	}
	concrete := present(x.res.Target.Type())
	switch {
	case dependent(concrete) || concrete.Kind() == types.None:
		return t
	case isOptional(t):
		return &types.OptionalType{Elem: concrete}
	}
	return concrete
}

// typed is lit as a value of t (API.md V1): a *ValueError when it does not fit.
func (x *opCtx) typed(lit Lit, t types.Type) (value.Value, error) {
	if lit == nil {
		return nil, ErrBadOp
	}
	if err := utf8Text(lit, t); err != nil {
		return nil, err
	}
	ty := x.a.typer(x.res.root.pkg.Path)
	ty.scope, ty.json = x.wireScope(), x.j.editable().Mode == ModeJSON
	return ty.Value(x.a.ctx, lit, t)
}

// wireScope is the field whose wire rules the operation's value takes (WIRE.md 4.1): the path's
// field for a Set of it (or a SetCase); for an element or entry below it, or added to it, its
// unit and encoding.
func (x *opCtx) wireScope() *types.Field {
	return x.scopeAt(len(x.res.Steps), x.op.Kind == OpSet || x.op.Kind == OpSetCase)
}

// scopeAt is the field whose wire rules the value at cursor k takes: its own field's, none marker
// included, when whole; else, as an element or entry below the nearest field, that field's unit
// and encoding (WIRE.md 4.1). The Undo carries values in the scope its inverse reads them in.
func (x *opCtx) scopeAt(k int, whole bool) *types.Field {
	for i := k; i > 0; i-- {
		f := x.fieldAt(i)
		switch {
		case f == nil:
			continue
		case i == k && whole:
			return fieldRules(f)
		}
		return &types.Field{Unit: f.Unit, Enc: f.Enc}
	}
	return nil
}

// fieldRules are f's wire rules for its whole value: its unit, encoding and none marker.
func fieldRules(f *types.Field) *types.Field {
	return &types.Field{Unit: f.Unit, Enc: f.Enc, NoneWire: f.NoneWire}
}

// unitOf is the wire unit scope gives Durations outside any record, ms for no field.
func unitOf(scope *types.Field) types.Unit {
	if scope == nil {
		return types.UnitMs
	}
	return scope.Unit
}

// keepKey is v with the key of the keyed-list element it replaces (API.md E5): a key written
// differently is not editable; a key left out keeps the element's.
func (x *opCtx) keepKey(v value.Value) (value.Value, error) {
	n := len(x.res.Steps)
	if n == 0 {
		return v, nil
	}
	lt, ok := baseOf(x.res.Steps[n-1].Container).(*types.ListType)
	rec, isRec := v.(*value.Record)
	if !ok || lt.KeyedBy == nil || !isRec {
		return v, nil
	}
	cur := keyField(x.res.Target, lt.KeyedBy)
	i := lt.KeyedBy.Index
	switch {
	case i >= len(rec.Fields):
		return v, nil
	case rec.Fields[i] == nil:
		rec.Fields[i], rec.Set[i] = cur, true
	case cur != nil && !sameValue(cur, rec.Fields[i]):
		return nil, &NotEditableError{Reason: ReasonKey}
	}
	return rec, nil
}

// setsDefault reports a Set of a field to its default (API.md E6, E7), which removes the field;
// in a JSON source, none with a @json(none:) marker is written as the marker instead (E7).
func (x *opCtx) setsDefault(v value.Value) bool {
	rec, f, ok := x.field()
	if !ok || !hasDefault(f) {
		return false
	}
	nw := withField(rec, f.Index, v)
	if !x.a.isDefault(nw, f.Index) {
		return false
	}
	_, none := v.(*value.None)
	return !none || f.NoneWire == nil || x.j.holder().mode != ModeJSON
}

// change sets the path's value to v, nil for the field's default, through the nearest value
// the source states: absent fields on the way are materialized (W8), the rest by M1.
func (x *opCtx) change(v value.Value) error {
	k := x.statedIndex()
	switch {
	case v == nil:
		k = x.statedBelow(len(x.res.Steps) - 1) // a field left to its default goes from its literal
	case x.wiredInParent(k):
		k = x.statedBelow(k - 1)
	}
	k = x.writtenAt(k)
	old := x.valueAt(k)
	nw := withChange(old, x.res.Steps[k:], v)
	return x.diffAt(k, old, nw)
}

// wiredInParent reports a value at cursor k that a JSON source writes as members of its parent
// object (an inline variant, parallel keys: WIR-07, DECISIONS 21), not as one member.
func (x *opCtx) wiredInParent(k int) bool {
	f := x.fieldAt(k)
	return k > 0 && x.j.cur[k].mode == ModeJSON && f != nil && (f.Inline || f.Pairs != nil)
}

// statedIndex is the last cursor of the walk still in the source tree: the target itself,
// or the literal an absent or spread-supplied field is inserted into (W3).
func (x *opCtx) statedIndex() int {
	return x.statedBelow(len(x.j.cur) - 1)
}

// statedBelow is the last cursor from k down still in the source tree.
func (x *opCtx) statedBelow(k int) int {
	k = min(k, len(x.j.cur)-1)
	for k > 0 && x.j.cur[k].state != stTree {
		k--
	}
	return k
}

// valueAt is the value at cursor k: the root's for 0, else step k-1's.
func (x *opCtx) valueAt(k int) value.Value {
	if k == 0 {
		return x.res.val
	}
	return x.res.Steps[k-1].Value
}

// withChange is a copy of v with the value at steps replaced by nv, nil leaving a field to its
// default; every record on the way marks the field it goes through as written (W8).
func withChange(v value.Value, steps []Step, nv value.Value) value.Value {
	if len(steps) == 0 {
		return nv
	}
	child := withChange(stepValue(v, steps[0]), steps[1:], nv)
	switch x := v.(type) {
	case *value.Record:
		i := fieldIndex(fieldsOf(x.T), steps[0].Seg.Name)
		return withField(x, i, child)
	case *value.List:
		out := &value.List{T: x.T, Elems: slices.Clone(x.Elems), P: x.P}
		out.Elems[position(x, steps[0].Value)] = child
		return out
	case *value.Map:
		out := &value.Map{T: x.T, Keys: x.Keys, Vals: slices.Clone(x.Vals), P: x.P}
		out.Vals[position(x, steps[0].Value)] = child
		return out
	case *value.Table:
		out := &value.Table{T: x.T, Entries: slices.Clone(x.Entries), P: x.P}
		rec, _ := child.(*value.Record)
		out.Entries[position(x, steps[0].Value)] = rec
		return out
	}
	return v
}

// stepValue is the value step st read in v.
func stepValue(v value.Value, st Step) value.Value {
	if rec, ok := v.(*value.Record); ok {
		if i := fieldIndex(fieldsOf(rec.T), st.Seg.Name); i >= 0 {
			return rec.Fields[i]
		}
	}
	return st.Value
}

// withField is a copy of rec with field i set to v, written; nil leaves it to its default.
func withField(rec *value.Record, i int, v value.Value) *value.Record {
	out := &value.Record{T: rec.T, Fields: slices.Clone(rec.Fields), Set: slices.Clone(rec.Set), Ident: rec.Ident, P: rec.P}
	if i < 0 || i >= len(out.Fields) {
		return out
	}
	out.Fields[i], out.Set[i] = v, v != nil
	return out
}
