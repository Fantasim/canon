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

// foldConst folds through the Folder; a failure it did not report is E3015 (DECISIONS 150, 263).
func (c *checker) foldConst(env *env, e syntax.Expr) (value.Value, bool) {
	before := env.pkg.bag.ErrorCount()
	v, ok := c.fold.Fold(c.ctx, env.owner, e, c.info)
	switch {
	case ok:
		return v, true
	case env.pkg.bag.ErrorCount() == before:
		c.report(env, diag.E3015.AtNotConstant(env.span(e), env.what))
	default:
		c.breakObj(env.owner)
	}
	return nil, false
}

// notConstant is E3015 for a `load`, `self`, an `if` or a `match` in a constant (TYPES.md §15).
func (c *checker) notConstant(env *env, e syntax.Expr) bool {
	switch e.(type) {
	case *syntax.LoadExpr, *syntax.SelfExpr, *syntax.IfExpr, *syntax.MatchExpr:
		c.report(env, diag.E3015.AtNotConstant(env.span(e), env.what))
		return true
	}
	return false
}

// readsBroken reports e naming a declaration broken now, before propagateBroken (TYPES.md §1).
func (c *checker) readsBroken(e syntax.Expr) bool {
	seen := map[*object]bool{}
	found := false
	syntax.Inspect(e, func(n syntax.Node) bool {
		var o Object
		switch x := n.(type) {
		case *syntax.IdentExpr:
			o = c.info.Uses[x]
		case *syntax.Ident:
			o = c.info.NameUses[x]
		}
		if obj, isObj := o.(*object); isObj && c.brokenNow(obj, seen) {
			found = true
		}
		return !found
	})
	return found
}

// brokenNow reports o broken, or naming a broken declaration through its dependencies.
func (c *checker) brokenNow(o *object, seen map[*object]bool) bool {
	if c.info.Broken[o] {
		return true
	}
	if seen[o] {
		return false
	}
	seen[o] = true
	return slices.ContainsFunc(c.deps[o], func(d *object) bool { return c.brokenNow(d, seen) })
}
