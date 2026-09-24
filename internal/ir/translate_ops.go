package ir

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// unarySpec is a unary operator of the portable subset and the kinds of its operand.
type unarySpec struct {
	op    Op
	kinds map[types.Kind]bool
}

// present reports that every node is there: a missing one was refused or broken, already noted.
func present(ns ...PExpr) bool {
	for _, n := range ns {
		if n == nil {
			return false
		}
	}
	return true
}

// unary is `-x` on a number or duration, `not x` on a Bool (CONFORMANCE.md §2.2).
func (t *translator) unary(e syntax.Expr) PExpr {
	x := e.(*syntax.UnaryExpr)
	spec, ok := unaryOps[x.Op]
	if !ok || !spec.kinds[t.kind(x.X)] {
		t.refuseScan(x, x.X)
		return nil
	}
	operand := t.expr(x.X)
	if !present(operand) {
		return nil
	}
	return &Unary{T: t.computed(t.typ(x)), Op: spec.op, X: operand}
}

// binary is arithmetic, a comparison, `and`, `or` or `??` on the kinds CONFORMANCE.md §2.2 allows; any other refuses the whole expression.
func (t *translator) binary(e syntax.Expr) PExpr {
	x := e.(*syntax.BinaryExpr)
	if x.Op == syntax.TokCoalesce {
		return t.coalesce(x)
	}
	op, ok := binaryOps[x.Op]
	if !ok || !t.operandsFit(op, x.X, x.Y) {
		t.refuseScan(x, x.X, x.Y)
		return nil
	}
	a, b := t.expr(x.X), t.expr(x.Y)
	if !present(a, b) {
		return nil
	}
	ty := t.typ(x)
	if arithmetic(op) {
		ty = t.computed(ty)
	}
	return &Binary{T: ty, Op: op, X: a, Y: b}
}

// operandsFit reports operands op accepts: numbers and durations for arithmetic, Bools for `and`
// and `or`, scalars for `==` and `!=`, numbers, durations and ordered enums for an ordering.
func (t *translator) operandsFit(op Op, x, y syntax.Expr) bool {
	kx, ky := t.kind(x), t.kind(y)
	switch {
	case arithmetic(op):
		return numericKinds[kx] && numericKinds[ky]
	case op == OpAnd || op == OpOr:
		return kx == types.Bool && ky == types.Bool
	case op == OpEq || op == OpNe:
		return scalarKinds[kx] && scalarKinds[ky]
	case numericKinds[kx] && numericKinds[ky]:
		return true
	}
	return kx == types.Enum && ky == types.Enum && t.ordered(x) && t.ordered(y)
}

// arithmetic reports `+ - * / %`, the operators before OpNeg.
func arithmetic(op Op) bool { return op < OpNeg }

// ordered reports an expression of an `ordered` enum type.
func (t *translator) ordered(e syntax.Expr) bool {
	ty := t.checked(e)
	if ty == nil {
		return false
	}
	en, ok := ty.Base().(*types.EnumType)
	return ok && en.Ordered
}

// coalesce is `x ?? y` on an optional path of self (CONFORMANCE.md §2.2); an optional parameter there is E9003's.
func (t *translator) coalesce(x *syntax.BinaryExpr) PExpr {
	left := unparen(x.X)
	if id, ok := left.(*syntax.IdentExpr); ok {
		if o := t.s.info.Uses[id]; o != nil && o.Kind() == check.ObjParam && o.Type().Base().Kind() == types.Optional {
			t.broken = true
			return nil
		}
	}
	read, isRead := t.selfRead(left, ctxCoalesce)
	if !isRead {
		t.refuseScan(x, x.X, x.Y)
		return nil
	}
	fallback := t.expr(x.Y)
	if !present(read, fallback) {
		return nil
	}
	return &Coalesce{T: t.typ(x), X: read, Y: fallback}
}

// isExpr is `x is c` on a variant-typed path of self: the pure function receives its case kind (CONFORMANCE.md §2.2).
func (t *translator) isExpr(e syntax.Expr) PExpr {
	x := e.(*syntax.IsExpr)
	c := t.caseOf(x)
	if c == nil {
		t.refuseScan(x, x.X)
		return nil
	}
	read, isRead := t.selfRead(unparen(x.X), ctxIs)
	if !isRead {
		t.refuseScan(x, x.X)
		return nil
	}
	if !present(read) {
		return nil
	}
	return &IsCase{T: t.s.ref(types.BoolType), X: read, Case: c.Index}
}

// caseOf is the variant case an `is` names, nil for anything else.
func (t *translator) caseOf(x *syntax.IsExpr) *types.CaseType {
	if x.Target == nil || len(x.Target.Parts) == 0 {
		return nil
	}
	o := t.s.info.NameUses[x.Target.Parts[len(x.Target.Parts)-1]]
	if o == nil || o.Kind() != check.ObjCase || o.Type() == nil {
		return nil
	}
	c, _ := o.Type().Base().(*types.CaseType)
	return c
}

// stringLit is a constant string, or a template of Strings, integers and enums without format spec, a Float or Duration there E9005 (CONFORMANCE.md §2.2).
func (t *translator) stringLit(e syntax.Expr) PExpr {
	x := e.(*syntax.StringLit)
	if !hasInterp(x) {
		return t.literal(x)
	}
	out := &Template{T: t.typ(x)}
	text, ok := "", true
	for _, p := range x.Parts {
		if p.Interp == nil {
			text += p.Text
			continue
		}
		n := t.interpolated(p.Interp)
		ok = ok && n != nil
		out.Parts = append(out.Parts, TemplatePart{Text: text, X: n})
		text = ""
	}
	if text != "" {
		out.Parts = append(out.Parts, TemplatePart{Text: text})
	}
	if !ok {
		return nil
	}
	return out
}

func hasInterp(x *syntax.StringLit) bool {
	for _, p := range x.Parts {
		if p.Interp != nil {
			return true
		}
	}
	return false
}

// interpolated is one `{x}` of a template; a format spec is refused, its value still judged.
func (t *translator) interpolated(in *syntax.Interp) PExpr {
	ty := t.checked(in.X)
	switch {
	case ty == nil:
		return nil
	case in.Spec != nil:
		t.refuseScan(in, in.X)
		if untemplated[ty.Base().Kind()] {
			t.report(t.file.Span(in.X), func(sp source.Span) *diag.Builder { return diag.E9005.At(sp, sp, ty) })
		}
		return nil
	case untemplated[ty.Base().Kind()]:
		t.report(t.file.Span(in.X), func(sp source.Span) *diag.Builder { return diag.E9005.At(sp, sp, ty) })
		return nil
	case !templateKinds[ty.Base().Kind()]:
		t.refuse(in.X)
		return nil
	}
	return t.expr(in.X)
}
