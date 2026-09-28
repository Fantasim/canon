package table

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// filters are the element view's `filters` (VIEWMODEL.md 7.3), each resolved to its kind and
// control (T11), `none` for an optional field (T12); a field that cannot be filtered (E1620) has
// none, and `multi` holds only for a choice, case or contains filter (E1621).
func (t *Tables) filters(named namer, v control.View, variant *types.VariantType) []vm.Filter {
	declared, ok := v.Item(syntax.KindViewFilters).(*syntax.ViewFilters)
	if !ok {
		return nil
	}
	var out []vm.Filter
	for _, fl := range declared.Items {
		if fl.Name == nil {
			continue
		}
		keys := named(fl.Name.Name)
		if len(keys) == 0 && variant != nil && t.index.CaseKind(fl.Name) {
			out = append(out, t.caseFilter(variant, fl.Multi.Valid()))
		}
		for _, k := range keys {
			if f, ok := t.filter(k, fl.Multi.Valid()); ok {
				out = append(out, f)
			}
		}
	}
	return out
}

// caseFilter is the filter on the case of a table of variants (T6a, T11): `$case`, a choice over
// its cases, named `kind` by the variant view.
func (t *Tables) caseFilter(v *types.VariantType, multi bool) vm.Filter {
	ctl, n, _ := t.res.Choice(nil, v)
	return vm.Filter{Field: encode.KeyCase, Kind: filterCase, Multi: multi, Control: choices(ctl, n, multi)}
}

// filter is the filter on the keyed field k (T11, T12).
func (t *Tables) filter(k encode.Keyed, multi bool) (vm.Filter, bool) {
	ft := shape.StripOptional(k.Field.Type)
	f := vm.Filter{Field: k.Key, None: k.Field.Type.Base().Kind() == types.Optional}
	switch {
	case shape.KindIn(ft, types.Enum, types.Ref):
		f.Kind = filterChoice
	case shape.KindIn(ft, types.Variant):
		f.Kind = filterCase
	case shape.KindIn(ft, types.Bool):
		f.Kind, f.Control = filterBool, vm.Control{Kind: control.CtlSegmented, Source: &vm.Source{Bool: true}}
		return f, true
	case shape.KindIn(ft, types.Int, types.Float, types.Duration):
		el := t.res.Value(k.Decl, ft)
		f.Kind, f.Control = filterRange, vm.Control{Kind: control.CtlRange, Element: &el}
		return f, true
	case shape.ElemOf(ft) != nil && shape.KindIn(shape.ElemOf(ft), types.Enum, types.Ref):
		f.Kind, ft = filterContains, shape.ElemOf(ft)
	default:
		return vm.Filter{}, false
	}
	ctl, n, _ := t.res.Choice(k.Decl, ft)
	f.Multi, f.Control = multi, choices(ctl, n, multi)
	return f, true
}

// choices is a single choice by C4/C5, or with `multi` checkboxes for at most 6 choices, else
// chips (T11).
func choices(ctl vm.Control, n int, multi bool) vm.Control {
	if !multi {
		return ctl
	}
	kind := control.CtlChips
	if n <= checkboxesMax {
		kind = control.CtlCheckboxes
	}
	return vm.Control{Kind: kind, Source: ctl.Source}
}
