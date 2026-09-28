package control

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// inner is the context of an element, a key or a value of what c resolves: its declaring record
// and Duration unit stay, its field's @json(bits) and inline do not.
func inner(c at) at { return at{decl: c.decl, unit: c.unit} }

// scalar is Bool, an integer type, a Float, a String (assets included), a Duration, an enum, a
// ref or a literal union (VIEWMODEL.md 4.3).
func scalar(t types.Type) bool {
	return shape.KindIn(t, types.Bool, types.Int, types.Float, types.String, types.Duration,
		types.Enum, types.Ref, types.LitUnion)
}

// recordLike is a record, a variant or a variant case used as a type (C26, C29).
func recordLike(t types.Type) bool { return shape.KindIn(t, types.Record, types.Variant, types.Case) }

// caseName is the fixed case of a variant case used as a type, "" for another type.
func caseName(t types.Type) string {
	if c, ok := t.Base().(*types.CaseType); ok {
		return c.Name
	}
	return ""
}

// listControl is a list's control by its element (C20–C27).
func (r *Resolver) listControl(c at, t types.Type) vm.Control {
	l := t.Base().(*types.ListType)
	switch e := l.Elem; {
	case shape.KindIn(e, types.Enum):
		return enumsControl(t, e.Base().(*types.EnumType), c.enc)
	case shape.KindIn(e, types.Ref):
		return vm.Control{Kind: ctlChips, Source: r.source(c, e)} // C22
	case scalar(e):
		return r.scalarsControl(c, t, e)
	case recordLike(e):
		return vm.Control{Kind: CtlTable, Of: encode.Name(e), Case: caseName(e)} // C26; T1's other members are §7's
	}
	el := r.control(inner(c), l.Elem, "", false)
	return vm.Control{Kind: ctlList, Element: &el} // C27
}

// enumsControl is checkboxes for a set of at most 6 active members, else chips (C20, C21).
func enumsControl(t types.Type, e *types.EnumType, enc types.Enc) vm.Control {
	kind := ctlChips
	if IsSet(t, enc) && activeMembers(e) <= checkboxesMax {
		kind = ctlCheckboxes
	}
	return vm.Control{Kind: kind, Source: &vm.Source{Enum: e.String()}}
}

// scalarsControl is a list of scalars: a range for the range refinement (C23), positional
// inputs for a length bound of at most 6 (C24), else tags (C25).
func (r *Resolver) scalarsControl(c at, t, e types.Type) vm.Control {
	el := r.control(inner(c), e, "", false)
	if strict, ok := rangeForm(t); ok {
		return vm.Control{Kind: ctlRange, Element: &el, Strict: strict}
	}
	lo, hi := encode.Lengths(t)
	if hi == nil || *hi < 1 || *hi > positionalMax {
		return vm.Control{Kind: ctlTags, Element: &el}
	}
	ctl := vm.Control{Kind: ctlPositional, Element: &el, Count: *hi}
	if lo != nil && *lo > 0 {
		ctl.MinCount = lo
	}
	return ctl
}

// tableControl is a table's rows (C26).
func tableControl(_ *Resolver, _ at, t types.Type) vm.Control {
	return vm.Control{Kind: CtlTable, Of: encode.Name(t.Base().(*types.TableType).Elem)}
}

// mapControl is one input per member for an enum key and a scalar value (C28), cards for
// record or variant values (C29), else a key/value table (C30).
func (r *Resolver) mapControl(c at, t types.Type) vm.Control {
	m := t.Base().(*types.MapType)
	val := r.control(inner(c), m.Value, "", false)
	if e, ok := m.Key.Base().(*types.EnumType); ok && scalar(m.Value) {
		return vm.Control{Kind: ctlEnumRow, Source: &vm.Source{Enum: e.String()}, Value: &val}
	}
	key := r.control(inner(c), m.Key, "", false)
	if recordLike(m.Value) {
		return vm.Control{Kind: ctlCards, Of: encode.Name(m.Value), KeyControl: &key, Value: &val}
	}
	return vm.Control{Kind: ctlMap, KeyControl: &key, Value: &val}
}

// depMapControl is cards whose keys are fixed once added, over the key domain (D10); `of` is
// the value's record or variant, or its `match` function, and absent otherwise.
func (r *Resolver) depMapControl(c at, t types.Type) vm.Control {
	d := t.Base().(*types.DepMapType)
	keyType := &types.RefType{Target: d.Coll}
	key := r.control(inner(c), keyType, "", false)
	val := r.control(inner(c), d.Value, "", false)
	return vm.Control{
		Kind: ctlCards, Of: cardsOf(d.Value), Source: r.source(inner(c), keyType),
		KeyControl: &key, Value: &val, KeyFixed: true,
	}
}

// cardsOf is what a dependent map's cards show: a record or a variant, or a `match` function
// (a function without one expanded, J11); "" for any other value.
func cardsOf(v types.Type) string {
	app, ok := v.Base().(*types.TypeAppType)
	switch {
	case ok && Matches(app.Fn):
		return encode.FuncName(app.Fn)
	case ok && app.Fn.Body != nil:
		return cardsOf(shape.Expand(app))
	case recordLike(v):
		return encode.Name(v)
	}
	return ""
}
