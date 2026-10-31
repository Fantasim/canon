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
	v, ok, silent := c.foldReported(env, e)
	switch {
	case !silent:
	case c.readsFailed(e):
		c.breakFolded(env.owner) // its const's fold failed with a finding, which this reader does not repeat (TYPES.md §1)
	default:
		c.report(env, diag.E3015.AtNotConstant(env.span(e), env.what))
	}
	return v, ok
}

// foldReported folds e, its findings in the owner's bag; a failure with one breaks the owner, silent is one without (DECISIONS 150).
func (c *checker) foldReported(env *env, e syntax.Expr) (v value.Value, ok, silent bool) {
	before := env.pkg.bag.ErrorCount()
	v, ok = c.fold.Fold(c.ctx, env.owner, e, c.info)
	if ok {
		return v, true, false
	}
	c.noteSpent(env.pkg)
	if env.pkg.bag.ErrorCount() == before {
		return nil, false, true
	}
	c.breakFolded(env.owner)
	return nil, false, false
}

// breakFolded breaks the owner of a fold that failed with a finding (DECISIONS 150); a session
// notes the break as the fold's when the fold made it.
func (c *checker) breakFolded(o *object) {
	if c.journal != nil && o != nil && !c.info.Broken[o] {
		c.journal.foldBroke(o)
	}
	c.breakObj(o)
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
	return c.readsWhere(e, c.isBroken)
}

func (c *checker) isBroken(o *object) bool {
	return c.info.Broken[o]
}

// readsFailed reports e naming a const whose declaration holds an error now, directly or through
// its dependencies: an earlier fold of it failed with a finding (DECISIONS 150).
func (c *checker) readsFailed(e syntax.Expr) bool {
	return c.readsWhere(e, c.holdsErrorNow)
}

// readsWhere reports e naming a declaration hit holds, or one naming it through its dependencies.
func (c *checker) readsWhere(e syntax.Expr, hit func(*object) bool) bool {
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
		if obj, isObj := o.(*object); isObj && c.hitNow(obj, seen, hit) {
			found = true
		}
		return !found
	})
	return found
}

// hitNow reports hit holding for o or for a declaration o names through its dependencies.
func (c *checker) hitNow(o *object, seen map[*object]bool, hit func(*object) bool) bool {
	if hit(o) {
		return true
	}
	if seen[o] {
		return false
	}
	seen[o] = true
	return slices.ContainsFunc(c.deps[o], func(d *object) bool { return c.hitNow(d, seen, hit) })
}

// holdsErrorNow reports a const whose declaration holds an error of its package's bag.
func (c *checker) holdsErrorNow(o *object) bool {
	p := c.pkgs[o.pkg]
	if o.kind != ObjConst || o.file == nil || p == nil {
		return false
	}
	at := o.file.Span(o.decl)
	return slices.ContainsFunc(p.bag.Findings(), func(f diag.Finding) bool {
		return f.Severity == diag.Error && f.Span.File == at.File && at.Contains(f.Span.Start)
	})
}
