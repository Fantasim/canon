package control

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// hint is a built-in `control:` hint: the types it accepts, the optional stripped, and the
// control it resolves to (VIEWMODEL.md 4.5).
type hint struct {
	accepts func(types.Type) bool
	resolve func(r *Resolver, c at, t types.Type, name string) vm.Control
}

// hints are the closed list of §4.5; any other name is E1609.
var hints map[string]hint

func init() {
	isBool := func(t types.Type) bool { return shape.KindIn(t, types.Bool) }
	isChoice := func(t types.Type) bool { return shape.KindIn(t, types.Enum, types.Ref, types.Variant) }
	isChoices := func(t types.Type) bool {
		e := shape.ElemOf(t)
		return e != nil && shape.KindIn(e, types.Enum, types.Ref)
	}
	isText := func(t types.Type) bool { return shape.KindIn(t, types.String) }
	isNumber := func(t types.Type) bool { return shape.KindIn(t, types.Int, types.Float, types.Duration) }
	isColor := func(t types.Type) bool {
		return isText(t) && shape.HasPattern(t, colorPattern) || t.Base() == types.Type(types.UInt32Type)
	}
	plain := func(_ *Resolver, _ at, _ types.Type, name string) vm.Control { return vm.Control{Kind: name} }
	hints = map[string]hint{
		CtlSwitch: {isBool, plain}, CtlCheckbox: {isBool, plain},
		CtlSegmented: {isChoice, (*Resolver).choiceHint}, CtlRadio: {isChoice, (*Resolver).choiceHint},
		CtlSelect: {isChoice, (*Resolver).choiceHint}, ctlSearch: {isChoice, (*Resolver).choiceHint},
		CtlCheckboxes: {isChoices, (*Resolver).choicesHint}, CtlChips: {isChoices, (*Resolver).choicesHint},
		ctlInput: {isText, textHint}, ctlTextarea: {isText, textHint}, ctlCode: {isText, textHint},
		CtlNumber: {isNumber, numberHint}, hintStepper: {isNumber, numberHint},
		CtlSlider: {isNumber, sliderHint},
		ctlColor:  {isColor, plain},
		ctlText:   {func(types.Type) bool { return true }, plain},
	}
}

// Accepts reports whether name is a built-in hint, and whether it accepts t, the optional
// stripped (VIEWMODEL.md C40); a slider also needs both bounds (E1619, shape.Bounds).
func Accepts(name string, t types.Type) (known, ok bool) {
	h, known := hints[name]
	return known, known && h.accepts(t)
}

// unbounded is a slider on a number without both bounds (E1619).
func unbounded(name string, t types.Type) bool {
	lo, hi := shape.Bounds(t)
	return name == CtlSlider && (!lo || !hi)
}

// hint is the control a valid hint gives t (C1 step 2); false for none, an unknown name, or a
// hint the type does not take (E1601, E1619 are the checker's).
func (r *Resolver) hint(c at, t types.Type, name string) (vm.Control, bool) {
	h, ok := hints[name]
	if !ok || !h.accepts(t) || unbounded(name, t) {
		return vm.Control{}, false
	}
	return h.resolve(r, c, t, name), true
}

// choiceHint is a choice kind over an enum, a ref's target or a variant's case selector.
func (r *Resolver) choiceHint(c at, t types.Type, name string) vm.Control {
	ctl := r.typeRule(c, t)
	if ctl.Kind == ctlVariant {
		ctl.Selector.Kind = name
		return ctl
	}
	ctl.Kind = name
	return ctl
}

// choicesHint is checkboxes or chips over a list of enum members or refs.
func (r *Resolver) choicesHint(c at, t types.Type, name string) vm.Control {
	return vm.Control{Kind: name, Source: r.source(c, shape.ElemOf(t))}
}

// textHint is input, textarea or code with the string's lengths and pattern (C11).
func textHint(r *Resolver, _ at, t types.Type, name string) vm.Control {
	ctl := r.stringControl(t)
	ctl.Kind = name
	return ctl
}

// numberHint is number, or duration for a Duration; `stepper` adds stepper to a number.
func numberHint(r *Resolver, c at, t types.Type, name string) vm.Control {
	if t.Base().Kind() == types.Duration {
		return durationControl(c, t)
	}
	ctl := numberControl(t)
	ctl.Stepper = name == hintStepper
	return ctl
}

// sliderHint is a slider over the number's bounds.
func sliderHint(_ *Resolver, _ at, t types.Type, _ string) vm.Control {
	return withBounds(vm.Control{Kind: CtlSlider}, t)
}
