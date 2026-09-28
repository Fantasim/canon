package control

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/encode"
)

// scale is the choice control of n active enum members or variant cases (C4).
func scale(n int) string {
	switch {
	case n <= segmentedMax:
		return CtlSegmented
	case n <= selectMax:
		return CtlSelect
	}
	return ctlSearch
}

// activeMembers counts an enum's members that are not retired (C3).
func activeMembers(e *types.EnumType) int {
	n := 0
	for _, m := range e.Members {
		if !m.Retired {
			n++
		}
	}
	return n
}

// enumControl is a choice by C4 over the enum's active members (C8).
func (r *Resolver) enumControl(_ at, t types.Type) vm.Control {
	e := t.Base().(*types.EnumType)
	return vm.Control{Kind: scale(activeMembers(e)), Source: &vm.Source{Enum: e.String()}}
}

// variantControl is a case selector, a choice by C4 over the active cases, and the current
// case's fields (C9); `inline` for an @json(inline) field (L17).
func (r *Resolver) variantControl(c at, t types.Type) vm.Control {
	v := variantOf(t)
	n := 0
	for _, cs := range v.Cases {
		if !cs.Retired {
			n++
		}
	}
	sel := vm.Control{Kind: scale(n), Source: &vm.Source{Cases: v.String()}}
	return vm.Control{Kind: ctlVariant, Of: v.String(), Selector: &sel, Inline: c.inline}
}

// caseControl is a variant case used as a type: its fields, the case fixed, no selector.
func caseControl(t types.Type) vm.Control {
	c := t.Base().(*types.CaseType)
	return vm.Control{Kind: ctlVariant, Of: c.Variant.String(), Case: c.Name}
}

// variantOf is a variant type, or the variant of a case; nil for another type.
func variantOf(t types.Type) *types.VariantType {
	if cs, ok := t.Base().(*types.CaseType); ok {
		return cs.Variant
	}
	v, _ := t.Base().(*types.VariantType)
	return v
}

// refControl is a choice over the ref's target: a select of the current keys of a collection of
// the enclosing instance (C6); a select for at most 10 active entries, else a search (C5).
func (r *Resolver) refControl(c at, t types.Type) vm.Control {
	coll := t.Base().(*types.RefType).Target
	src := r.source(c, t)
	if coll.Kind == types.CollField {
		return vm.Control{Kind: CtlSelect, Source: src}
	}
	_, active := r.counts(coll)
	kind := ctlSearch
	if active <= selectMax {
		kind = CtlSelect
	}
	return vm.Control{Kind: kind, Source: src}
}

// source is what a choice picks from: an enum's members, a variant's cases or a ref's target
// (VIEWMODEL.md 12.5 `source`).
func (r *Resolver) source(c at, t types.Type) *vm.Source {
	switch x := t.Base().(type) {
	case *types.EnumType:
		return &vm.Source{Enum: x.String()}
	case *types.RefType:
		if s, perInstance := encode.Sibling(c.decl, x.Target); perInstance {
			return siblingSource(s)
		}
		return &vm.Source{Collection: encode.CollectionID(x.Target)}
	}
	if v := variantOf(t); v != nil {
		return &vm.Source{Cases: v.String()}
	}
	return nil
}

// counts are a collection's entries and active entries in this build, none without Env.Counts.
func (r *Resolver) counts(coll *types.Collection) (count, active int) {
	if r.env.Counts == nil {
		return 0, 0
	}
	return r.env.Counts(coll)
}

// siblingSource is a choice over a collection of the enclosing instance; none when the record
// declaring the ref is unknown.
func siblingSource(s *vm.Sibling) *vm.Source {
	if s == nil {
		return nil
	}
	return &vm.Source{Sibling: s}
}
