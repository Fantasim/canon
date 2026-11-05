package edit

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// readableSet is readableValue for a Set's value v, typed as st, at the path's declared type; a
// write into the typed literal stating it keeps its name (keptTyped).
func (x *opCtx) readableSet(st types.Type, v value.Value) error {
	t, dep := x.declaredType()
	if !dep {
		t = st
	}
	return x.readableValue(t, x.frameAt(len(x.res.Steps)), v, dep && x.keptTyped(v))
}

// readableItem is readableValue for v, an Insert's or AddEntry's item of declared type t in frame fr,
// once its container's own text reads as the branch it computes (readableContainer).
func (x *opCtx) readableItem(t types.Type, fr depFrame, v value.Value) error {
	if err := x.readableContainer(); err != nil {
		return err
	}
	return x.readableValue(t, fr, v, false)
}

// readableContainer refuses an item into the path's container, of a dependent type whose computed
// list or map branch its bare `[…]` or `{…}` does not read as (DECISIONS 171, 474), in Canon text.
func (x *opCtx) readableContainer() error {
	t, dep := x.declaredType()
	if !dep || x.j.editable().Mode == ModeJSON {
		return nil
	}
	arm, _, ok := x.frameAt(len(x.res.Steps)).computedIn(t)
	if !ok || computesNever(arm) || reads(t, arm, x.res.Target) {
		return nil
	}
	return x.otherBranch(arm)
}

// readableValue refuses v, of declared type t in fr, Canon text reads as another branch than t
// computes (API.md V1, M7; DECISIONS 171, 475), below it too for a FromJSON (B7): no typed record
// literal states it, or a Source wrote it bare. A JSON decoder reads the computed one.
func (x *opCtx) readableValue(t types.Type, fr depFrame, v value.Value, kept bool) error {
	if x.j.editable().Mode == ModeJSON {
		return nil
	}
	if dependent(present(t)) {
		arm, in, ok := fr.computedIn(t)
		switch {
		case !ok || computesNever(arm):
			return nil // an inactive layer's arm, refused `computed` (W11a); Never takes no such value (V1)
		case !kept && !reads(t, arm, v) && (!statedTyped(v) || bareText(x.op.Value)):
			return x.otherBranch(arm)
		}
		t, fr = arm, in
	}
	if _, fromJSON := x.op.Value.(FromJSON); fromJSON {
		return x.readableIn(v, t, fr)
	}
	return nil
}

// reads reports v, on the arm dependent t computes, read as that arm from Canon text: a scalar,
// or a value whose form's first branch is the arm (DECISIONS 171).
func reads(t, arm types.Type, v value.Value) bool {
	fits := valueForm(v)
	return fits == nil || sameBranch(check.FirstFittingBranch(present(t), fits), arm)
}

// sameBranch reports branches a and b one type (sameType), an optional's taken for its element.
func sameBranch(a, b types.Type) bool {
	return sameType(present(a), present(b))
}

// sameType reports a and b one type: a record by its package and name, a list or map by its
// parts, so a value of another analysis of the project compares (the Undo prints the base's).
func sameType(a, b types.Type) bool {
	switch x := a.Base().(type) {
	case *types.RecordType:
		y, ok := b.Base().(*types.RecordType)
		return ok && x.Pkg == y.Pkg && x.Name == y.Name
	case *types.ListType:
		y, ok := b.Base().(*types.ListType)
		return ok && sameKey(x.KeyedBy, y.KeyedBy) && sameType(x.Elem, y.Elem)
	case *types.MapType:
		y, ok := b.Base().(*types.MapType)
		return ok && sameType(x.Key, y.Key) && sameType(x.Value, y.Value)
	case *types.DepMapType:
		y, ok := b.Base().(*types.DepMapType)
		return ok && x.Coll.Pkg == y.Coll.Pkg && x.Coll.Name == y.Coll.Name && types.Identical(x.Value, y.Value)
	}
	return types.Identical(a, b)
}

// sameKey reports two lists keyed alike: by fields of one name, or neither keyed.
func sameKey(x, y *types.Field) bool {
	if x == nil || y == nil {
		return x == y
	}
	return x.Name == y.Name
}

// otherBranch is the V1 refusal of the operation's value on arm, which Canon text cannot state.
func (x *opCtx) otherBranch(arm types.Type) error {
	return &ValueError{Expected: arm.String(), Got: describe(x.op.Value), Detail: detailOtherBranch}
}

// readableIn is readableAt on each field, element and map value of v, of type t in frame fr.
func (x *opCtx) readableIn(v value.Value, t types.Type, fr depFrame) error {
	switch c := v.(type) {
	case *value.Record:
		in, fields := fr.enter(c, t), fieldsOf(c.T)
		for i, fv := range c.Fields {
			if err := x.readableAt(fv, fieldType(fields, i), in); err != nil {
				return err
			}
		}
	case *value.List:
		for _, e := range c.Elems {
			if err := x.readableAt(e, elemType(present(t)), fr); err != nil {
				return err
			}
		}
	case *value.Map:
		return x.readableVals(c, t, fr)
	}
	return nil
}

// readableVals is readableAt on each value of map m, of type t, a dependent map's binder bound to
// its key (TYPES.md 11.5).
func (x *opCtx) readableVals(m *value.Map, t types.Type, fr depFrame) error {
	_, vt, _ := mapTypes(present(t))
	dm, isDep := baseOf(present(t)).(*types.DepMapType)
	for i, mv := range m.Vals {
		in := fr
		if isDep && i < len(m.Keys) {
			in = fr.bind(dm.Binder, m.Keys[i])
		}
		if err := x.readableAt(mv, vt, in); err != nil {
			return err
		}
	}
	return nil
}

// readableAt refuses v, of declared type t in frame fr, when t is dependent and Canon text reads v
// as another branch than the one fr computes; then the same below v.
func (x *opCtx) readableAt(v value.Value, t types.Type, fr depFrame) error {
	if v == nil || t == nil {
		return nil
	}
	if !dependent(present(t)) {
		return x.readableIn(v, t, fr)
	}
	arm, in, ok := fr.computedIn(t)
	switch {
	case !ok || computesNever(arm):
		return nil
	case !reads(t, arm, v) && !statedTyped(v):
		return x.otherBranch(arm)
	}
	return x.readableIn(v, arm, in)
}

// bareText reports lit a Source the checker reads as written, never as the typed literal M7 prints
// in its place: a brace or list it wrote bare (DECISIONS 171, 475).
func bareText(lit Lit) bool {
	text, isSource := lit.(Source)
	if !isSource {
		return false
	}
	e, _ := parseLiteral(string(text))
	_, typed := syntax.Unparen(e).(*syntax.TypedLit)
	return !typed
}

// fieldType is field i's declared type, nil past the fields.
func fieldType(fields []*types.Field, i int) types.Type {
	if i >= len(fields) {
		return nil
	}
	return fields[i].Type
}

// valueForm is the test of the branches the checker reads v, written as Canon text, against: a
// list's a list branch, a record's, case's or map's a brace branch (DECISIONS 171); nil for a scalar.
func valueForm(v value.Value) func(types.Type) bool {
	switch v.(type) {
	case *value.List:
		return check.IsListBranch
	case *value.Record, *value.Map:
		return check.IsBraceBranch
	}
	return nil
}

// keptTyped reports v written into the typed literal that states the path's value now, of v's own
// type: the write keeps the literal's name (M6), so Canon text reads v's type.
func (x *opCtx) keptTyped(v value.Value) bool {
	last := len(x.j.cur) - 1
	e, isExpr := x.j.cur[last].node.(syntax.Expr)
	if x.statedIndex() != last || !isExpr {
		return false
	}
	if _, typed := syntax.Unparen(e).(*syntax.TypedLit); !typed {
		return false
	}
	old, oldRec := x.valueAt(len(x.res.Steps)).(*value.Record)
	nw, nwRec := v.(*value.Record)
	return oldRec && nwRec && types.Identical(old.T, nw.T)
}
