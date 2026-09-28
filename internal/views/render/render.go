package render

import (
	"context"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
)

// Evaluator evaluates a view expression for one value (VIEWMODEL.md 3.4): self the value
// shown, magic the values of its magic names; false when the evaluation fails (X7).
type Evaluator interface {
	Eval(ctx context.Context, e syntax.Expr, self value.Value, magic Magic) (value.Value, bool)
}

// Magic are the values of the magic names of a position (VIEWMODEL.md 3.4): `id` of a table or
// define-table entry, `key` of a map value, `index` of a list element; nil where it has none (G12).
type Magic struct {
	ID, Key, Index value.Value
}

// Input is what templates are rendered with.
type Input struct {
	Program *check.Program
	Index   *control.Index
	Texts   *encode.Texts
	Colls   *encode.Colls
	Eval    Evaluator // nil: every rendering fails (X7)
}

// Renderer renders view templates (VIEWMODEL.md 9.2) in the source language (lang "") or in a
// translation's.
type Renderer struct {
	ctx  context.Context
	in   Input
	tr   translations
	keys map[*types.Collection]map[value.Key]*value.Record
	// inTarget is set while a ref's target title renders: a ref inside it renders its key (S8)
	inTarget bool
}

// New renders with in; the translations are read from in.Program's translation files.
func New(ctx context.Context, in Input) *Renderer {
	return &Renderer{ctx: ctx, in: in, tr: readTranslations(in.Program)}
}

// template renders s for self with the magic names m in lang (X4, X5); false when an
// interpolation fails, which renders as nothing (X7).
func (r *Renderer) template(s syntax.StrLit, self value.Value, m Magic, lang string) (string, bool) {
	switch x := s.(type) {
	case *syntax.RawStringLit:
		return x.Value, true
	case *syntax.StringLit:
		var b strings.Builder
		for _, p := range x.Parts {
			if p.Interp == nil {
				b.WriteString(p.Text)
				continue
			}
			text, ok := r.interp(p.Interp, self, m, lang)
			if !ok {
				return "", false
			}
			b.WriteString(text)
		}
		return b.String(), true
	}
	return "", false
}

// Value is e's value for self with m; false when it fails or there is no Evaluator.
func (r *Renderer) Value(e syntax.Expr, self value.Value, m Magic) (value.Value, bool) {
	if r.in.Eval == nil || e == nil {
		return nil, false
	}
	v, ok := r.in.Eval.Eval(r.ctx, e, self, m)
	return v, ok && v != nil
}

// interp is one interpolation's text: its format spec applied to a number (STDLIB.md 9.5), else
// the view text of its value (X4).
func (r *Renderer) interp(in *syntax.Interp, self value.Value, m Magic, lang string) (string, bool) {
	v, ok := r.Value(in.X, self, m)
	if !ok {
		return "", false
	}
	if in.Spec != nil {
		return std.Format(v, std.Spec{Plus: in.Spec.Plus, Comma: in.Spec.Comma, Decimals: in.Spec.Decimals}), true
	}
	return r.text(v, in.X, self, lang), true
}

// text is a value as a view renders it (X4): a ref by its target's title (S8), an enum member by
// its label, `none` by the field's `none` text; any other value in its canonical text.
func (r *Renderer) text(v value.Value, x syntax.Expr, self value.Value, lang string) string {
	switch y := v.(type) {
	case *value.Ref:
		return r.refText(y, lang)
	case *value.Member:
		return r.memberLabel(y, lang)
	case *value.None:
		if t, ok := r.noneLabel(x, self, lang); ok {
			return t
		}
		return noneText
	}
	return v.CanonText()
}
