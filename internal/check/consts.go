package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// constType is a const's type, synthesized from its constant initializer when first needed (TYPES.md §15).
func (c *checker) constType(o *object) types.Type {
	if o.typ != nil {
		return o.typ
	}
	d := o.decl.(*syntax.ConstDecl)
	env := c.declEnv(o)
	if o.state == stateResolving {
		c.constCycle(o)
		return types.ErrorType
	}
	o.state = stateResolving
	c.constStack = append(c.constStack, o)
	t := c.synth(env.constant(diag.KindConstValue), d.Value)
	c.constStack = c.constStack[:len(c.constStack)-1]
	o.state = stateDone
	if o.typ == nil {
		o.typ = t
	}
	return o.typ
}

// constCycle breaks a cycle's consts with no finding: eval reports E4301 (EVALUATION.md §3.2, DECISIONS 172).
func (c *checker) constCycle(o *object) {
	for _, m := range c.constStack[slices.Index(c.constStack, o):] {
		m.typ = types.ErrorType
		c.breakObj(m)
	}
}

// foldConst folds through the Folder; a failure it did not report is E3015 (IMPLEMENTATION-PLAN §4.7).
func (c *checker) foldConst(env *env, e syntax.Expr) (value.Value, bool) {
	before := env.pkg.bag.Summary().Errors
	v, ok := c.fold.Fold(c.ctx, env.owner, e, c.info)
	switch {
	case ok:
		return v, true
	case env.pkg.bag.Summary().Errors == before:
		c.report(env, diag.E3015.At(env.span(e), env.what))
	default:
		c.breakObj(env.owner)
	}
	return nil, false
}

// notConstant is E3015 for a `load`, `self`, an `if` or a `match` in a constant (TYPES.md §15).
func (c *checker) notConstant(env *env, e syntax.Expr) bool {
	switch e.(type) {
	case *syntax.LoadExpr, *syntax.SelfExpr, *syntax.IfExpr, *syntax.MatchExpr:
		c.report(env, diag.E3015.At(env.span(e), env.what))
		return true
	}
	return false
}
