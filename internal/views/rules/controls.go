package rules

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// control is a `control:` hint: a name (a string or an expression is E1609, log-2026-09-28 check
// follow-ups 2), known, accepting the field's type, bounded for a slider (C40).
func (v *view) control(n named, fi *syntax.FieldItem) {
	id, ok := shape.Unparen(fi.Value).(*syntax.IdentExpr)
	if !ok {
		v.report(diag.E1609.At(v.span(fi.Value), encode.SourceText(v.file, fi.Value)))
		return
	}
	t := shape.StripOptional(n.field.Type)
	known, accepts := control.Accepts(id.Name, t)
	switch {
	case !known:
		v.report(diag.E1609.At(v.span(id), id.Name))
	case !accepts:
		v.report(diag.E1601.At(v.span(id), id.Name, n.field.Type, n.field.Name))
	case id.Name == control.CtlSlider:
		lo, hi := shape.Bounds(t)
		if !lo {
			v.report(diag.E1619.AtLower(v.span(id), n.field.Type))
		} else if !hi {
			v.report(diag.E1619.AtUpper(v.span(id), n.field.Type))
		}
	}
}

// placeholder is plain text, on a field whose resolved control is a text, number or choice
// control (VIEWMODEL.md 3.5): E1613 on any other.
func (v *view) placeholder(n named, fi *syntax.FieldItem) {
	if s, ok := shape.Unparen(fi.Value).(syntax.StrLit); ok {
		v.plain(s)
	}
	if !v.c.controls.TakesPlaceholder(n.field) {
		v.report(diag.E1613.AtProperty(v.span(fi.Name), fi.Name.Name, n.kind))
	}
}
