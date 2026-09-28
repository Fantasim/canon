package rules

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// columns name fields once each, with widths in range (VIEWMODEL.md §3.3, §3.6).
func (v *view) columns(it syntax.ViewItem) {
	seen := map[string]source.Span{}
	for _, col := range it.(*syntax.ViewColumns).Items {
		if col.Name != nil && !v.again(v.span(col.Name), col.Name.Name, seen) {
			v.fieldOnly(col.Name, diag.E1606.AtColumns)
		}
		// A width beyond int64 cannot be rendered by E1613: check refuses the literal itself.
		if w := col.Width; w != nil && w.Value.IsInt64() && (w.Value.Int64() < minWidth || w.Value.Int64() > maxWidth) {
			v.report(diag.E1613.AtWidth(v.span(w), w.Value.Int64()))
		}
	}
}

// filters name fields once each, of a type that can be filtered, `multi` where it applies (T14).
func (v *view) filters(it syntax.ViewItem) {
	seen := map[string]source.Span{}
	for _, fl := range it.(*syntax.ViewFilters).Items {
		if fl.Name == nil || v.again(v.span(fl.Name), fl.Name.Name, seen) {
			continue
		}
		if f := v.fieldOnly(fl.Name, diag.E1606.AtFilters); f != nil {
			v.filter(fl, f)
		}
	}
}

// filter is E1620 on a type that cannot be filtered, E1621 at a `multi` that does not apply.
func (v *view) filter(fl *syntax.ViewFilter, f *types.Field) {
	k := filterOf(shape.StripOptional(f.Type))
	switch {
	case k == filterNone:
		v.report(diag.E1620.At(v.span(fl.Name), f.Type, f.Name))
	case fl.Multi.Valid() && k != filterChoice && k != filterCase && k != filterContains:
		v.report(diag.E1621.At(v.tok(fl.Multi), f.Type))
	}
}

// fieldOnly is the field a column or filter names: nothing is E1602, a method notField's.
func (v *view) fieldOnly(id *syntax.Ident, notField func(span source.Span, name, typ string) *diag.Builder) *types.Field {
	o := v.c.info.NameUses[id]
	switch {
	case o == nil:
		v.report(diag.E1602.At(v.span(id), v.name, id.Name))
	case o.Kind() == check.ObjField:
		return v.field(id)
	default:
		v.report(notField(v.span(id), id.Name, v.name))
	}
	return nil
}

// filterOf is the filter a type gives, the optional stripped (VIEWMODEL.md T11).
func filterOf(t types.Type) filterKind {
	switch {
	case shape.KindIn(t, types.Enum, types.Ref):
		return filterChoice
	case shape.KindIn(t, types.Variant):
		return filterCase
	case shape.KindIn(t, types.Bool):
		return filterBool
	case shape.KindIn(t, types.Int, types.Float, types.Duration):
		return filterRange
	}
	if e := shape.ElemOf(t); e != nil && shape.KindIn(e, types.Enum, types.Ref) {
		return filterContains
	}
	return filterNone
}

// search types each term: text, a number, an enum, a ref or an asset, an optional of one or a
// list of one; another is E1622 (VIEWMODEL.md G13).
func (v *view) search(it syntax.ViewItem) {
	for _, e := range it.(*syntax.ViewSearch).Items {
		t := v.c.info.Types[e]
		if t == nil || isError(t) {
			continue
		}
		term := shape.StripOptional(t)
		if el := shape.ElemOf(t); el != nil {
			term = shape.StripOptional(el)
		}
		if !shape.KindIn(term, types.String, types.Int, types.Enum, types.VariantKind, types.Ref) {
			v.report(diag.E1622.At(v.span(e), t))
		}
	}
}

// preview is an asset or its optional; another type is E1612 (VIEWMODEL.md G13).
func (v *view) preview(it syntax.ViewItem) {
	x := it.(*syntax.ViewPreview).X
	t := v.c.info.Types[x]
	if t == nil || isError(t) || shape.IsAsset(shape.StripOptional(t)) {
		return
	}
	v.report(diag.E1612.At(v.span(x), t))
}
