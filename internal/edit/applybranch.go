package edit

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// readableSet refuses a Set's value v, typed as st, bound for Canon text that reads another branch
// than its dependent field computes (API.md V1; TYPES.md 11.4; DECISIONS 171), below it too for a
// FromJSON (B7); a JSON decoder reads the computed one (WIRE.md 5.9). Until EDIT-TYPEDSOURCE.
func (x *opCtx) readableSet(st types.Type, v value.Value) error {
	if x.j.editable().Mode == ModeJSON {
		return nil
	}
	fr := x.frameAt(len(x.res.Steps))
	if t, dep := x.declaredType(); dep {
		arm, in, ok := fr.computedIn(t)
		switch {
		case !ok || computesNever(arm):
			return nil // an inactive layer's arm, refused `computed` (W11a); Never takes no such value (V1)
		case !x.keptTyped(v) && !reads(t, arm, v):
			return x.otherBranch(arm)
		}
		st, fr = arm, in
	}
	if _, fromJSON := x.op.Value.(FromJSON); fromJSON {
		return x.readableIn(v, st, fr)
	}
	return nil
}

// reads reports v, on the arm dependent t computes, read as that arm from Canon text: a scalar,
// or a value whose form's first branch is the arm (DECISIONS 171).
func reads(t, arm types.Type, v value.Value) bool {
	fits := valueForm(v)
	return fits == nil || types.Identical(present(check.FirstFittingBranch(present(t), fits)), present(arm))
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
	case !reads(t, arm, v):
		return x.otherBranch(arm)
	}
	return x.readableIn(v, arm, in)
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
