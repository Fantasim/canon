package control

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// Env is what a control reads beyond types and views.
type Env struct {
	Counts func(*types.Collection) (count, active int) // entries in this build (C3, C5); nil: none
	Fold   func(syntax.Expr) (value.Value, bool)       // a constant of a `where` (C34); nil: none
}

// Resolver resolves the controls of fields, values, elements, keys and map values (C1–C46).
type Resolver struct {
	index *Index
	env   Env
}

// NewResolver resolves controls with the properties of x.
func NewResolver(x *Index, env Env) *Resolver { return &Resolver{index: x, env: env} }

// at is where a control is resolved: the record or case declaring the field (for a ref into an
// enclosing record, C6), the field's @json(bits) (C32) and its Duration wire unit (X12).
type at struct {
	decl   types.Type
	enc    types.Enc
	unit   types.Unit
	inline bool
}

// Field is the control of field f of the record or case decl (C1, C2): its widget, else its
// hint, else a default widget, else the type rules; then its optional wrapper and unit.
func (r *Resolver) Field(decl types.Type, f *types.Field) vm.Control {
	c := at{decl: decl, enc: f.Enc, unit: f.Unit, inline: f.Inline}
	p := r.index.Field(f)
	fallback := r.control(c, f.Type, p.Name(syntax.PropControl), threeState(f))
	withUnit(&fallback, p.Name(syntax.PropUnit))
	w, ok := r.index.widget(f.Type, p)
	if !ok {
		return fallback
	}
	w.Fallback, w.Optional = &fallback, fallback.Optional
	return w
}

// Value is the control of a value of type t outside a field: a top-level value, an element, a key
// or a map value (VIEWMODEL.md 4); decl is the record or case whose field holds it, nil for none.
func (r *Resolver) Value(decl types.Type, t types.Type) vm.Control {
	return r.control(at{decl: decl}, t, "", false)
}

// control is t's control: hint's when valid, else the type rules, with the optional wrapper
// of C35–C39.
func (r *Resolver) control(c at, t types.Type, hintName string, three bool) vm.Control {
	o, optional := t.Base().(*types.OptionalType)
	if !optional {
		return r.chosen(c, t, hintName, false)
	}
	ctl := r.chosen(c, o.Elem, hintName, true)
	if ctl.Kind == ctlNever {
		return ctl // C39
	}
	unset := unsetClear
	if ctl.Kind == CtlSegmented {
		unset = unsetSegment
	}
	ctl.Optional = &vm.ControlOptional{Unset: unset, ThreeState: three}
	return ctl
}

// chosen is a valid hint's control, else the type rules'; a `Bool?` without one is segmented
// over Yes, No and Unset (C35).
func (r *Resolver) chosen(c at, t types.Type, hintName string, optional bool) vm.Control {
	if ctl, ok := r.hint(c, t, hintName); ok {
		return ctl
	}
	if optional && t.Base().Kind() == types.Bool {
		return vm.Control{Kind: CtlSegmented, Source: &vm.Source{Bool: true}}
	}
	return r.typeRule(c, t)
}

// threeState is an optional field whose default is not `none` (C36).
func threeState(f *types.Field) bool {
	if f.Default == nil || f.Type.Base().Kind() != types.Optional {
		return false
	}
	_, none := shape.Unparen(f.Default).(*syntax.NoneLit)
	return !none
}

// withUnit puts unit on the control that shows the number: ctl itself, or its element or
// value control (C45).
func withUnit(ctl *vm.Control, unit string) {
	if unit == "" {
		return
	}
	for _, c := range []*vm.Control{ctl, ctl.Element, ctl.Value} {
		if c != nil && (c.Kind == CtlNumber || c.Kind == CtlSlider) {
			c.Unit = unit
			return
		}
	}
}

// TakesPlaceholder reports that f's control is a text, number or choice control (VIEWMODEL.md
// 3.5): input, textarea, code, number, segmented, select or search.
func (r *Resolver) TakesPlaceholder(f *types.Field) bool {
	switch r.Field(nil, f).Kind {
	case ctlInput, ctlTextarea, ctlCode, CtlNumber, CtlSegmented, CtlSelect, ctlSearch:
		return true
	}
	return false
}

// Cell is ctl in a table cell (C46): switch is a checkbox, segmented and radio a select, and
// the optional wrapper clears.
func Cell(ctl vm.Control) vm.Control {
	switch ctl.Kind {
	case CtlSwitch:
		ctl.Kind = CtlCheckbox
	case CtlSegmented, CtlRadio:
		ctl.Kind = CtlSelect
	}
	if ctl.Optional != nil {
		o := *ctl.Optional
		o.Unset = unsetClear
		ctl.Optional = &o
	}
	return ctl
}
