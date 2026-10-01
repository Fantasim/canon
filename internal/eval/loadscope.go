package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// fieldScope is a field given its value by expr, whose scope the loads it reaches decode in (DECISIONS 268).
type fieldScope struct {
	f     *types.Field
	expr  syntax.Expr
	under bool                      // expr gives a value below the field (an amendment's element or map value)
	inner *types.Field              // the scope of a value below the field: its unit and int form, made once
	loads map[*syntax.LoadExpr]bool // the loads reached, true for the field's whole value; nil until asked
}

// newFieldScope is f given its value by expr.
func newFieldScope(f *types.Field, expr syntax.Expr) *fieldScope {
	return &fieldScope{f: f, expr: expr}
}

// of is load e's scope: the field's rules as its whole value, its unit and int below it, else nil (WIRE.md §4.1).
func (s *fieldScope) of(info *check.Info, e *syntax.LoadExpr) *types.Field {
	if s == nil {
		return nil
	}
	if s.loads == nil {
		s.loads = map[*syntax.LoadExpr]bool{}
		w := scopeWalk{info: info, loads: s.loads}
		w.walk(s.expr, !s.under)
	}
	whole, ok := s.loads[e]
	switch {
	case !ok:
		return nil
	case whole:
		return s.f
	case s.inner == nil: // bits rides along inert: a bits field's elements are enums (WIRE.md §4.1), as edit's scopeAt
		s.inner = &types.Field{Unit: s.f.Unit, Enc: s.f.Enc}
	}
	return s.inner
}

// scopeWalk finds the loads a field's expected type reaches through the positions passing it on (TYPES.md §5.1).
type scopeWalk struct {
	info  *check.Info
	loads map[*syntax.LoadExpr]bool
}

// scopeStep follows one kind of node; a kind without one passes no expected type on.
type scopeStep func(w *scopeWalk, e syntax.Expr, whole bool)

// scopeSteps dispatches on the node kind (DECISIONS 26); filled in init, since its steps walk
// subexpressions through it.
var scopeSteps [syntax.NodeKindCount]scopeStep

func init() {
	scopeSteps = [syntax.NodeKindCount]scopeStep{
		syntax.KindLoadExpr: scopeLoad, syntax.KindParenExpr: scopeParen, syntax.KindIfExpr: scopeIf,
		syntax.KindMatchExpr: scopeMatch, syntax.KindBinaryExpr: scopeCoalesce, syntax.KindListLit: scopeList,
		syntax.KindListComp: scopeListComp, syntax.KindBraceLit: scopeMap,
	}
}

// walk follows e, the whole value of the field or a value below it.
func (w *scopeWalk) walk(e syntax.Expr, whole bool) {
	if e == nil {
		return
	}
	if step := scopeSteps[e.Kind()]; step != nil {
		step(w, e, whole)
	}
}

func scopeLoad(w *scopeWalk, e syntax.Expr, whole bool) {
	w.loads[e.(*syntax.LoadExpr)] = whole
}

func scopeParen(w *scopeWalk, e syntax.Expr, whole bool) {
	w.walk(e.(*syntax.ParenExpr).X, whole)
}

func scopeIf(w *scopeWalk, e syntax.Expr, whole bool) {
	for x := e.(*syntax.IfExpr); x != nil; x = x.ElseIf {
		w.body(x.Then, whole)
		w.body(x.Else, whole)
	}
}

// body follows an if branch, nil for none.
func (w *scopeWalk) body(b *syntax.ExprBody, whole bool) {
	if b != nil {
		w.walk(b.X, whole)
	}
}

func scopeMatch(w *scopeWalk, e syntax.Expr, whole bool) {
	for _, arm := range e.(*syntax.MatchExpr).Arms {
		w.walk(arm.Body, whole)
	}
}

// scopeCoalesce is `a ?? b`, whose operands are checked against the whole's type (TYPES.md §5.1).
func scopeCoalesce(w *scopeWalk, e syntax.Expr, whole bool) {
	if x := e.(*syntax.BinaryExpr); x.Op == syntax.TokCoalesce {
		w.walk(x.X, whole)
		w.walk(x.Y, whole)
	}
}

func scopeList(w *scopeWalk, e syntax.Expr, _ bool) {
	for _, el := range e.(*syntax.ListLit).Elems {
		w.walk(el, false)
	}
}

func scopeListComp(w *scopeWalk, e syntax.Expr, _ bool) {
	w.walk(e.(*syntax.ListComp).Elem, false)
}

// scopeMap follows a map literal's or comprehension's values; a record or table literal's fields
// are fields of their own.
func scopeMap(w *scopeWalk, e syntax.Expr, _ bool) {
	lit := e.(*syntax.BraceLit)
	if k := w.info.Literals[lit]; k != check.LitMap && k != check.LitMapComp {
		return
	}
	for _, it := range lit.Items {
		switch x := it.(type) {
		case *syntax.MapItem:
			w.walk(x.Value, false)
		case *syntax.FieldItem:
			w.walk(x.Value, false)
		}
	}
}
