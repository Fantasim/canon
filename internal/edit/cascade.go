package edit

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// setCaseOp changes a variant's case (API.md E14): the old case's fields the new case has with
// the same type, refinements aside, then the given fields; every other old field is Dropped,
// and a kept field its new refinements refuse is dropped once all operations are applied.
func setCaseOp(x *opCtx) error {
	old, ok := x.res.Target.(*value.Record)
	if !ok {
		return ErrBadOp
	}
	oc, isCase := old.T.Base().(*types.CaseType)
	if !isCase {
		return ErrBadOp
	}
	i := slices.IndexFunc(oc.Variant.Cases, func(c *types.CaseType) bool { return c.Name == x.op.Case })
	if i < 0 {
		return &ValueError{Expected: oc.Variant.String(), Got: call(nameCase, x.op.Case)}
	}
	nc := oc.Variant.Cases[i]
	given := &value.Record{T: nc, Fields: make([]value.Value, len(nc.Fields)), Set: make([]bool, len(nc.Fields))}
	if x.op.Value != nil {
		v, err := x.typed(x.op.Value, nc)
		if err != nil {
			return err
		}
		given, _ = v.(*value.Record)
	}
	nw, err := x.kept(old, nc)
	if err != nil {
		return err
	}
	for j := range given.Fields {
		if given.Set[j] && given.Fields[j] != nil {
			nw.Fields[j], nw.Set[j] = given.Fields[j], true
			x.keptFields = slices.DeleteFunc(x.keptFields, func(name string) bool { return name == nc.Fields[j].Name })
		}
	}
	lit, err := x.a.sourceLit(old)
	if err != nil {
		return err
	}
	x.inverse(Operation{Kind: OpSet, Path: x.res.Canonical, Value: lit})
	x.nw = nw
	return x.change(nw)
}

// kept is the new case with the old case's fields E14 keeps, by static type; the others are
// Dropped. A kept field whose type differs by its refinements is judged at the end (E14).
func (x *opCtx) kept(old *value.Record, nc *types.CaseType) (*value.Record, error) {
	nw := &value.Record{T: nc, Fields: make([]value.Value, len(nc.Fields)), Set: make([]bool, len(nc.Fields)), Ident: old.Ident}
	same := sameShape(old.T, nc)
	for j, f := range fieldsOf(old.T) {
		v := old.Fields[j]
		if v == nil || f.Input != nil {
			continue
		}
		k := fieldIndex(nc.Fields, f.Name)
		if same || k >= 0 && types.Identical(f.Type, nc.Fields[k].Type) {
			// a kept value stays whatever the new case's default: M7 prints it only when they differ
			nw.Fields[k], nw.Set[k] = v, old.Set[j] || !same
			if !same && nc.Fields[k].Type != f.Type {
				x.keptFields = append(x.keptFields, f.Name)
			}
			continue
		}
		if err := x.drop(childPath(x.res.Canonical, Seg{Kind: SegField, Name: f.Name}), v, f); err != nil {
			return nil, err
		}
	}
	return nw, nil
}

// drop reports a value a cascade removed (API.md E14, E15), in its wire form.
func (x *opCtx) drop(path string, v value.Value, f *types.Field) error {
	raw, err := x.a.dropText(v, f)
	if err != nil {
		return err
	}
	x.w.dropped = append(x.w.dropped, Dropped{Path: path, Value: raw})
	return nil
}

// dropText is v's wire form as Dropped reports it; a value without one, its canonical text.
func (a *applier) dropText(v value.Value, f *types.Field) (json.RawMessage, error) {
	raw, err := a.wireText(v, f)
	if err == nil {
		return raw, nil
	}
	if raw, err = json.Marshal(v.CanonText()); err != nil {
		return nil, fmt.Errorf(fmtWrapped, errNoWire, err)
	}
	return raw, nil
}

// touched is a record an operation changed (API.md E15, E16): its path, the fields it changed,
// its fields' declarations, the fields a SetCase kept (E14), and its JSON source's file and
// pointer when one states it.
type touched struct {
	path      string
	fields    []string
	decl      []*types.Field
	kept      []string
	file, ptr string
}

// noteTouched records the records on the path whose field it goes through, and the target
// itself when it is a record whose fields the new value changed or a SetCase kept.
func (x *opCtx) noteTouched() {
	p := Path{Package: x.res.root.pkg.Path, Root: x.res.root.obj.Name()}
	for i, st := range x.res.Steps {
		if rec, isRec := x.res.parent(i).(*value.Record); isRec && st.Seg.Kind == SegField {
			x.w.records = append(x.w.records, x.touchedAt(p.String(), rec, fieldsOf(rec.T), []string{st.Seg.Name}))
		}
		p.Segs = append(p.Segs, st.Seg)
	}
	old, ok := x.res.Target.(*value.Record)
	rec, isRec := x.nw.(*value.Record)
	if !ok || !isRec {
		return
	}
	var changed []string
	for i, f := range fieldsOf(rec.T) {
		if !sameShape(old.T, rec.T) || !sameValue(old.Fields[i], rec.Fields[i]) {
			changed = append(changed, f.Name)
		}
	}
	t := x.touchedAt(x.res.Canonical, old, fieldsOf(rec.T), changed)
	t.kept = x.keptFields
	x.w.records = append(x.w.records, t)
}

// touchedAt is the record at path, stated by rec, with its fields and the ones changed.
func (x *opCtx) touchedAt(path string, rec value.Value, decl []*types.Field, changed []string) touched {
	t := touched{path: path, decl: decl, fields: changed}
	if p := provOf(rec); p != nil && p.Kind == value.ProvJSON {
		t.file, t.ptr = x.a.snap.display(p.Span.File), p.Pointer
	}
	return t
}
