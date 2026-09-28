package rules

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// hints are the built-in controls and what each accepts, the optional stripped (VIEWMODEL.md §4.5).
var hints map[string]func(types.Type) bool

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
	hints = map[string]func(types.Type) bool{
		ctlSwitch: isBool, ctlCheckbox: isBool,
		ctlSegmented: isChoice, ctlRadio: isChoice, ctlSelect: isChoice, ctlSearch: isChoice,
		ctlCheckboxes: isChoices, ctlChips: isChoices,
		ctlInput: isText, ctlTextarea: isText, ctlCode: isText,
		ctlNumber: isNumber, ctlStepper: isNumber, ctlSlider: isNumber,
		ctlColor: isColor,
		ctlText:  func(types.Type) bool { return true },
	}
}

// control is a `control:` hint: known, accepting the field's type, bounded for a slider (C40).
func (v *view) control(n named, fi *syntax.FieldItem) {
	id, ok := fi.Value.(*syntax.IdentExpr)
	if !ok {
		return
	}
	accepts, known := hints[id.Name]
	t := shape.StripOptional(n.field.Type)
	switch {
	case !known:
		v.report(diag.E1609.At(v.span(id), id.Name))
	case !accepts(t):
		v.report(diag.E1601.At(v.span(id), id.Name, n.field.Type, n.field.Name))
	case id.Name == ctlSlider:
		lo, hi := shape.Bounds(t)
		if !lo {
			v.report(diag.E1619.AtLower(v.span(id), n.field.Type))
		} else if !hi {
			v.report(diag.E1619.AtUpper(v.span(id), n.field.Type))
		}
	}
}
