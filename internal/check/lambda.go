package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// lambda is `x => e` or `(a, b) => e` (TYPES.md §12.4).
func (c *checker) lambda(env *env, e *syntax.LambdaExpr, want types.Type) types.Type {
	if unknownContext(want) {
		return c.lambdaAlone(env, e)
	}
	ft, ok := unwrap(want).(*types.FuncType)
	if !ok {
		c.report(env, diag.E3008.At(env.span(e)))
		return types.ErrorType
	}
	return c.lambdaBody(env, e, ft, nil)
}

// shorthand is `.f` or `.m(args)`, meaning `x => x.f`: one parameter, the chain's receiver.
func (c *checker) shorthand(env *env, e *syntax.ShorthandLambda, want types.Type) types.Type {
	if unknownContext(want) {
		return c.lambdaAlone(env, e)
	}
	ft, ok := unwrap(want).(*types.FuncType)
	if !ok {
		c.report(env, diag.E3008.At(env.span(e)))
		return types.ErrorType
	}
	return c.shorthandBody(env, e, ft, nil)
}

// lambdaAlone checks a lambda or a shorthand whose function type is unknown: its parameters of
// the error type, its body checked against the error type, so only the body's own findings show.
func (c *checker) lambdaAlone(env *env, e syntax.Expr) types.Type {
	switch x := e.(type) {
	case *syntax.LambdaExpr:
		inner := env.push()
		for _, id := range x.Params {
			c.declare(inner, id, c.newLocal(inner, ObjParam, id, id, types.ErrorType))
		}
		c.expr(inner, x.Body, types.ErrorType)
	case *syntax.ShorthandLambda:
		inner := env.with()
		inner.shorthand = types.ErrorType
		c.expr(inner, x.Body, types.ErrorType)
	}
	return types.ErrorType
}

// lambdaWith checks a lambda or a shorthand given to a built-in against fp, whose result may
// still be free in b; it records and returns the lambda's type, nil after a finding.
func (c *checker) lambdaWith(env *env, e syntax.Expr, fp *types.FuncType, b *binding) *types.FuncType {
	var t types.Type
	switch x := e.(type) {
	case *syntax.LambdaExpr:
		t = c.lambdaBody(env, x, fp, b)
	case *syntax.ShorthandLambda:
		t = c.shorthandBody(env, x, fp, b)
	}
	c.info.Types[e] = t
	ft, ok := t.(*types.FuncType)
	if !ok {
		return nil
	}
	return ft
}

// lambdaBody binds the parameters, a pair destructured, and types the body (TYPES.md §6.6, §12.4).
func (c *checker) lambdaBody(env *env, e *syntax.LambdaExpr, fp *types.FuncType, b *binding) types.Type {
	ptypes, ok := lambdaParams(len(e.Params), fp)
	if !ok {
		c.report(env, diag.E3002.At(env.span(e), b.shown(fp), anyFunc(len(e.Params))))
		return types.ErrorType
	}
	inner := env.push()
	inner.bind = nil // a body computes: evaluation reads no argument there
	for i, id := range e.Params {
		o := c.newLocal(inner, ObjParam, id, id, ptypes[i])
		c.declare(inner, id, o)
	}
	return &types.FuncType{Params: fp.Params, Result: c.lambdaResult(inner, e.Body, fp.Result, b)}
}

// lambdaParams are the types a lambda's n parameters take from fp.
func lambdaParams(n int, fp *types.FuncType) ([]types.Type, bool) {
	if n == len(fp.Params) {
		return fp.Params, true
	}
	if pair, ok := onePair(fp); ok && n == pairArity {
		return []types.Type{pair.A, pair.B}, true
	}
	return nil, false
}

func onePair(fp *types.FuncType) (*types.PairType, bool) {
	if len(fp.Params) != 1 {
		return nil, false
	}
	p, ok := fp.Params[0].Base().(*types.PairType)
	return p, ok
}

// anyFunc is a function type of n parameters of any type, for a message.
func anyFunc(n int) *types.FuncType {
	f := &types.FuncType{Result: types.AnyType}
	for range n {
		f.Params = append(f.Params, types.AnyType)
	}
	return f
}

// lambdaResult checks a body against its result type when it is known, else synthesizes it.
func (c *checker) lambdaResult(env *env, body syntax.Expr, result types.Type, b *binding) types.Type {
	if b != nil && b.free(result) {
		return c.synth(env, body)
	}
	c.expr(env, body, result)
	return result
}

// shorthandBody checks `.f…` with its receiver of fp's single parameter type.
func (c *checker) shorthandBody(env *env, e *syntax.ShorthandLambda, fp *types.FuncType, b *binding) types.Type {
	if len(fp.Params) != 1 {
		c.report(env, diag.E3002.At(env.span(e), fp, anyFunc(1)))
		return types.ErrorType
	}
	inner := env.with()
	inner.shorthand, inner.bind = fp.Params[0], nil
	return &types.FuncType{Params: fp.Params, Result: c.lambdaResult(inner, e.Body, fp.Result, b)}
}
