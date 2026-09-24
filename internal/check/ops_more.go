package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// coalesce is `a ?? b` (TYPES.md §5.1, §6.5).
func (c *checker) coalesce(env *env, e *syntax.BinaryExpr, want types.Type) types.Type {
	var ta types.Type
	if want != nil {
		ta = c.expr(env, e.X, optionalOf(want))
	} else {
		ta = c.expr(env.join(), e.X, nil)
	}
	if k := ta.Base().Kind(); k != types.Optional && k != types.None && k != types.Error {
		c.warn(env, diag.W3401.At(env.span(e.X), env.span(e.X), e.Op.String()))
	}
	if want != nil {
		elem := unwrapOptional(want)
		if tb := c.expr(env, e.Y, want); tb.Base().Kind() == types.Optional {
			return optionalOf(elem)
		}
		return elem
	}
	if ta.Kind() == types.None {
		return c.synth(env, e.Y)
	}
	elem := optElem(ta)
	tb := c.fallback(env, e.Y, elem)
	if tb.Base().Kind() == types.Optional {
		return optionalOf(elem)
	}
	return elem
}

// fallback types the right side of `??` against T, or T? when it is itself optional.
func (c *checker) fallback(env *env, e syntax.Expr, t types.Type) types.Type {
	if c.contextDependent(env, e) || t.Kind() == types.Error {
		return c.expr(env, e, t)
	}
	tb := c.synth(env, e)
	if tb.Base().Kind() == types.Optional {
		c.accept(env, e, tb, optionalOf(t))
	} else {
		c.accept(env, e, tb, t)
	}
	return tb
}

// optionalOf is T? for T, T? itself for an optional.
func optionalOf(t types.Type) types.Type {
	if k := t.Base().Kind(); k == types.Optional || k == types.Error {
		return t
	}
	return &types.OptionalType{Elem: t}
}

// unwrapOptional is T for T? (through aliases and refinements), else t.
func unwrapOptional(t types.Type) types.Type {
	if o, ok := t.Base().(*types.OptionalType); ok {
		return o.Elem
	}
	return t
}

// inExpr is `x in xs` (TYPES.md §5.1, STDLIB.md §5).
func (c *checker) inExpr(env *env, e *syntax.BinaryExpr) types.Type {
	ts := c.synth(env, e.Y)
	switch x := ts.Base().(type) {
	case *types.OptionalType:
		c.report(env, diag.E3402.At(env.span(e.Y), env.span(e.Y)))
	case *types.ListType:
		if x.KeyedBy != nil {
			c.memberOrKey(env, e.X, e.Y, x.Elem, x.KeyedBy.Type)
		} else {
			c.expr(env, e.X, optionalOf(x.Elem))
		}
	case *types.TableType:
		c.memberOrKey(env, e.X, e.Y, x.Elem, types.StringType)
	case *types.MapType:
		c.expr(env, e.X, optionalOf(x.Key))
	case *types.DepMapType:
		c.expr(env, e.X, optionalOf(&types.RefType{Target: x.Coll}))
	default:
		if ts.Kind() == types.Range {
			c.expr(env, e.X, optionalOf(types.IntType))
			break
		}
		tx := c.synth(env, e.X)
		if ts.Kind() != types.Error {
			c.report(env, diag.E3007.AtBinary(env.span(e), e.Op.String(), tx, ts))
		}
	}
	return types.BoolType
}

// memberOrKey types an element (T or ref T) or a key of a keyed collection (STDLIB.md §5).
func (c *checker) memberOrKey(env *env, x, recv syntax.Expr, elem, key types.Type) {
	if c.bareKey(env, x, recv, key) {
		return
	}
	t := c.synth(env, x)
	b := optElem(t)
	if c.assignable(b, elem) || c.assignable(b, key) || t.Kind() == types.Error {
		return
	}
	if r, isRef := b.Base().(*types.RefType); isRef && types.Identical(c.coll(r).Elem, elem) {
		return
	}
	c.report(env, diag.E3002.At(env.span(x), elem, t))
}

// isExpr is `x is c` (TYPES.md §8.3): x a variant or an optional one, c one of its cases; E3605 otherwise.
func (c *checker) isExpr(env *env, e *syntax.IsExpr) types.Type {
	t := c.synth(env, e.X)
	if t.Kind() == types.Error {
		return types.BoolType
	}
	v := variantOf(optElem(t))
	if v == nil {
		c.report(env, diag.E3605.AtNotVariant(env.span(e.X), t))
		return types.BoolType
	}
	c.caseTarget(env, e.Target, v)
	return types.BoolType
}

// caseTarget resolves `c` or `V.c` against a variant: E3605 when it is not one of its cases.
func (c *checker) caseTarget(env *env, q *syntax.QualifiedName, v *types.VariantType) *object {
	last := q.Parts[len(q.Parts)-1]
	if len(q.Parts) > 1 {
		o := c.typeName(env, &syntax.QualifiedName{Bounds: q.Bounds, Parts: q.Parts[:len(q.Parts)-1]})
		if o == nil {
			return nil
		}
		if t := c.typeOfName(env, o, nil); t == nil || variantOf(t) != v {
			c.report(env, diag.E3605.AtCase(env.span(q), qualified(q), v))
			return nil
		}
	}
	co := c.caseObject(v, last.Name)
	if co == nil {
		c.report(env, diag.E3605.AtCase(env.span(last), last.Name, v))
		return nil
	}
	c.info.NameUses[last] = co
	return co
}

// rangeExpr is `a..b` or `a..=b` as a value (TYPES.md §13.3): Int bounds and a start, E3025 otherwise.
func (c *checker) rangeExpr(env *env, e *syntax.RangeExpr) types.Type {
	ok := e.Lo != nil
	for _, b := range []syntax.Expr{e.Lo, e.Hi} {
		if b == nil {
			continue
		}
		if t := c.synth(env, b); t.Kind() != types.Error && t.Base().Kind() != types.Int {
			ok = false
		}
	}
	if !ok {
		c.report(env, diag.E3025.At(env.span(e)))
		return types.ErrorType
	}
	return types.RangeType
}
