package views_test

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/render"
)

// standIn evaluates the view expressions the tests render, in place of the evaluator, which has
// no entry for view expressions yet: a magic name, or a field of the value shown (`f`, `self.f`). Any other
// expression fails (VIEWMODEL.md X7).
type standIn struct{ info *check.Info }

func (s standIn) Eval(_ context.Context, e syntax.Expr, self value.Value, m render.Magic) (value.Value, bool) {
	if sel, ok := e.(*syntax.SelectorExpr); ok && sel.Name != nil {
		if _, isSelf := sel.X.(*syntax.SelfExpr); isSelf {
			return fieldOf(self, sel.Name.Name)
		}
	}
	id, ok := e.(*syntax.IdentExpr)
	if !ok {
		return nil, false
	}
	o := s.info.Uses[id]
	switch {
	case o == nil:
		return nil, false
	case o.Kind() == check.ObjField:
		return fieldOf(self, id.Name)
	case o.Kind() == check.ObjLocal:
		v := map[string]value.Value{"id": m.ID, "key": m.Key, "index": m.Index}[id.Name]
		return v, v != nil
	}
	return nil, false
}

// noneElse is standIn, with `none` for every expression it does not read (VIEWMODEL.md X4).
type noneElse struct{ standIn }

func (s noneElse) Eval(ctx context.Context, e syntax.Expr, self value.Value, m render.Magic) (value.Value, bool) {
	if v, ok := s.standIn.Eval(ctx, e, self, m); ok {
		return v, true
	}
	return &value.None{}, true
}

// fieldOf is the field name of the record self.
func fieldOf(self value.Value, name string) (value.Value, bool) {
	r, ok := self.(*value.Record)
	if !ok {
		return nil, false
	}
	for i, f := range encode.FieldsOf(r.T) {
		if f.Name == name && i < len(r.Fields) && r.Fields[i] != nil {
			return r.Fields[i], true
		}
	}
	return nil, false
}
