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
			c.element(env, e.X, e.Y, x.Elem, x.KeyedBy.Type)
		} else {
			c.expr(env, e.X, optionalOf(x.Elem))
		}
	case *types.TableType:
		c.element(env, e.X, e.Y, x.Elem, types.StringType)
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

// element types the element `in`, contains or indexOf takes; a key is E3026 (STDLIB.md §5, DECISIONS 317).
func (c *checker) element(env *env, x, recv syntax.Expr, elem, key types.Type) {
	if c.unscopedKey(env, x, recv, key) {
		c.keyNotElement(env, x, recv)
		return
	}
	t := c.synth(env, x)
	b := optElem(t)
	switch {
	case t.Kind() == types.Error || c.assignable(b, elem) || c.refInto(b, elem):
	case c.assignable(b, key):
		c.keyNotElement(env, x, recv)
	default:
		c.report(env, diag.E3002.At(env.span(x), elem, t))
	}
}

// keyNotElement is E3026, its implicit form for a method on an implicit receiver (`.contains(k)`).
func (c *checker) keyNotElement(env *env, x, recv syntax.Expr) {
	if recv == nil {
		c.report(env, diag.E3026.AtImplicit(env.span(x), env.span(x)))
		return
	}
	c.report(env, diag.E3026.AtWritten(env.span(x), env.span(x), env.span(recv)))
}

// unscopedKey records a bare name no scope has as a key, never E2102: the caller reports E3026 (TYPES.md §4.1).
func (c *checker) unscopedKey(env *env, x, recv syntax.Expr, key types.Type) bool {
	id, ok := x.(*syntax.IdentExpr)
	if !ok || c.lookup(env, id.Name) != nil {
		return false
	}
	c.info.Types[x] = key
	if o, _ := c.inExpected(unwrapUnion(key), id.Name); o != nil {
		c.info.Uses[id] = o
	} else if coll := c.receiverColl(recv); coll != nil {
		c.info.Keys[x] = coll
	}
	return true
}

// refInto reports a ref whose collection holds elements of type elem (STDLIB.md §5).
func (c *checker) refInto(t, elem types.Type) bool {
	r, isRef := t.Base().(*types.RefType)
	return isRef && types.Identical(c.coll(r).Elem, elem)
}

// isExpr is `x is c` (TYPES.md §8.3): x a variant or an optional one, c one of its cases; E3605 otherwise.
func (c *checker) isExpr(env *env, e *syntax.IsExpr) types.Type {
	t := c.synth(env, e.X)
	if t.Kind() == types.Error || c.notDependent(env, e.X, t, syntax.KwIs.String()) {
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
