package control

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// typeRules are the rows C7–C30 by the kind of T, aliases expanded, the outer optional removed.
var typeRules map[types.Kind]func(*Resolver, at, types.Type) vm.Control

func init() {
	fixed := func(kind string) func(*Resolver, at, types.Type) vm.Control {
		return func(*Resolver, at, types.Type) vm.Control { return vm.Control{Kind: kind} }
	}
	typeRules = map[types.Kind]func(*Resolver, at, types.Type) vm.Control{
		types.Bool:     fixed(CtlSwitch),
		types.Int:      func(_ *Resolver, _ at, t types.Type) vm.Control { return numberControl(t) },
		types.Float:    func(_ *Resolver, _ at, t types.Type) vm.Control { return numberControl(t) },
		types.Duration: func(_ *Resolver, c at, t types.Type) vm.Control { return durationControl(c, t) },
		types.String:   func(r *Resolver, _ at, t types.Type) vm.Control { return r.stringControl(t) },
		types.Enum:     (*Resolver).enumControl,
		types.Variant:  (*Resolver).variantControl,
		types.Case:     func(_ *Resolver, _ at, t types.Type) vm.Control { return caseControl(t) },
		types.Ref:      (*Resolver).refControl,
		types.LitUnion: (*Resolver).unionControl,
		types.TypeApp:  (*Resolver).dependentControl,
		types.Never:    fixed(ctlNever),
		types.Record:   func(_ *Resolver, _ at, t types.Type) vm.Control { return recordControl(t) },
		types.List:     (*Resolver).listControl,
		types.Table:    tableControl,
		types.Map:      (*Resolver).mapControl,
		types.DepMap:   (*Resolver).depMapControl,
	}
}

// typeRule is t's control by the rows of §4.3; a kind without a row is read-only text.
func (r *Resolver) typeRule(c at, t types.Type) vm.Control {
	if rule, ok := typeRules[t.Base().Kind()]; ok {
		return rule(r, c, t)
	}
	return vm.Control{Kind: ctlText}
}

// numberControl is a number with the type's bounds, and a stepper for an integer range of both
// bounds and at most 20 values (C12).
func numberControl(t types.Type) vm.Control {
	ctl := withBounds(vm.Control{Kind: CtlNumber}, t)
	if t.Base().Kind() == types.Int {
		r := encode.IntRange(t)
		width := r.Hi - r.Lo // negative only past int64, where there is no stepper
		ctl.Stepper = r.HasLo && r.HasHi && width >= 0 && width < stepperMax
	}
	return ctl
}

// withBounds adds the type's bounds as the type expression writes them.
func withBounds(ctl vm.Control, t types.Type) vm.Control {
	b := encode.NumberBounds(t)
	ctl.Min, ctl.Max, ctl.MinExclusive, ctl.MaxExclusive = b.Min, b.Max, b.MinExclusive, b.MaxExclusive
	return ctl
}

// durationControl is a duration offering the units from the wire unit up to the upper bound,
// smallest first (C13, X12).
func durationControl(c at, t types.Type) vm.Control {
	ctl := withBounds(vm.Control{Kind: ctlDuration}, t)
	r := encode.IntRange(t)
	for u := c.unit; u <= types.UnitD; u++ {
		if u == c.unit || !r.HasHi || u.Millis() <= r.Hi {
			ctl.Units = append(ctl.Units, u.String())
		}
	}
	return ctl
}

// stringControl is an input with the string's lengths and pattern (C11), or a file picker for
// an asset (C14).
func (r *Resolver) stringControl(t types.Type) vm.Control {
	if a := shape.LayersOf(t).Asset; a != nil {
		return vm.Control{Kind: ctlFile, Root: r.env.Assets.Root(a), Ext: a.Exts}
	}
	minLen, maxLen := encode.Lengths(t)
	return vm.Control{Kind: ctlInput, MinLen: minLen, MaxLen: maxLen, Pattern: encode.Pattern(t)}
}

// recordControl is an inline section for at most 6 fields, else a collapsible card, every
// declared field counted (C18, C19, C31).
func recordControl(t types.Type) vm.Control {
	kind := CtlCard
	if len(encode.FieldsOf(t)) <= sectionMax {
		kind = CtlSection
	}
	return vm.Control{Kind: kind, Of: encode.Name(t)}
}

// unionControl is T's control with the literals pinned (C15, D9).
func (r *Resolver) unionControl(c at, t types.Type) vm.Control {
	u := t.Base().(*types.LitUnionType)
	ctl := r.control(c, u.Of, "", false)
	ctl.Pinned = u.Literals
	return ctl
}
