package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// block checks a block in a new scope; false when it cannot complete normally (TYPES.md §6.6).
func (c *checker) block(env *env, b *syntax.Block) bool {
	return c.stmtsIn(env.push(), b)
}

// stmt checks one statement: the env after it (its facts) and whether it completes normally.
func (c *checker) stmt(env *env, s syntax.Stmt) (*env, bool) {
	switch s := s.(type) {
	case *syntax.LetStmt:
		c.local(env, s.Name, s.Type, s.Value, s)
	case *syntax.VarStmt:
		c.local(env, s.Name, s.Type, s.Value, s).mutable = true
	case *syntax.AssignStmt:
		return c.assign(env, s), true
	case *syntax.IfStmt:
		return c.ifStmt(env, s)
	case *syntax.ForStmt:
		return c.forStmt(env, s), true
	case *syntax.WhileStmt:
		return c.whileStmt(env, s), true
	case *syntax.MatchStmt:
		return c.matchStmt(env, s)
	case *syntax.BreakStmt, *syntax.ContinueStmt:
		return env, false
	case *syntax.ReturnStmt:
		c.returnStmt(env, s)
		return env, false
	case *syntax.ExpectStmt:
		c.expectStmt(env, s)
	case *syntax.ExprStmt:
		c.exprStmt(env, s)
	case *syntax.BadStmt:
		c.breakObj(env.owner)
	}
	return env, true
}

// local is `let x [: T] = e` or `var x [: T] = e` in a block (TYPES.md §12.7).
func (c *checker) local(env *env, name *syntax.Ident, typ syntax.Type, value syntax.Expr, decl syntax.Stmt) *object {
	var t types.Type
	if typ != nil {
		t = c.resolveType(&typeCtx{env: env, pos: posFn}, typ)
		c.expr(env, value, t)
	} else {
		t = inferred(c.synth(env, value))
	}
	o := c.newLocal(env, ObjLocal, name, decl, t)
	c.declare(env, name, o)
	return o
}

// assign is `x = e`, `x op= e` or `x[i]… = e` (TYPES.md §12.7).
func (c *checker) assign(env *env, s *syntax.AssignStmt) *env {
	root, field := assignRoot(s.Target)
	if field != nil {
		c.report(env, diag.E3307.At(env.span(field.Name), field.Name.Name))
		c.synth(env, s.Target)
		c.synth(env, s.Value)
		return env
	}
	if root == nil {
		c.synth(env, s.Value)
		return env
	}
	o := c.lookup(env, root.Name)
	if o == nil || o.kind != ObjLocal || !o.mutable {
		c.assignError(env, root, o)
		c.synth(env, s.Value)
		return env
	}
	c.info.Uses[root] = o
	target, read := o.typ, c.narrowed(env, root, o.typ)
	c.info.Types[root] = read
	if s.Target != syntax.Expr(root) {
		target = c.synth(env, s.Target)
		read = target
	}
	c.assignValue(env, s, target, read)
	if s.Target == syntax.Expr(root) {
		out := env.with()
		out.facts = env.facts.kill(o.id)
		return out
	}
	return env
}

// assignError is E3017 for a name that is not a var, E2102 for an unknown one.
func (c *checker) assignError(env *env, root *syntax.IdentExpr, o *object) {
	if o == nil {
		if !c.strayIt(env, root, root.Name) {
			c.unknownName(env, root, root.Name)
		}
		return
	}
	c.info.Uses[root] = o
	c.report(env, diag.E3017.At(env.span(root), root.Name))
}

// assignValue checks `x = e` against x's type, `x op= e` with x read under the facts (TYPES.md §7.1).
func (c *checker) assignValue(env *env, s *syntax.AssignStmt, target, read types.Type) {
	if s.Op == syntax.TokAssign {
		c.expr(env, s.Value, target)
		return
	}
	op := compoundOps[s.Op]
	tv := c.operand(env, s.Value, read)
	if tv.Kind() == types.Error || read.Kind() == types.Error {
		return
	}
	for _, side := range []struct {
		x syntax.Expr
		t types.Type
	}{{s.Target, read}, {s.Value, tv}} {
		if k := side.t.Base().Kind(); k == types.Optional || k == types.None {
			c.report(env, diag.E3402.At(env.span(side.x), env.span(side.x)))
			return
		}
	}
	r := arithResult(op, read.Base(), tv.Base())
	if r == nil && op == syntax.TokPlus {
		if l, ok := c.listConcat(read, tv); ok {
			r = l
			c.joined(s.Value, tv, l)
		}
	}
	if r == nil {
		c.report(env, diag.E3007.AtBinary(env.span(s), op.String(), read, tv))
		return
	}
	if !c.assignable(r, target) {
		c.report(env, diag.E3002.At(env.span(s.Value), target, r))
	}
}

// assignRoot is the root name of an assignment target (through index segments), or the first
// field selector found on the way.
func assignRoot(e syntax.Expr) (*syntax.IdentExpr, *syntax.SelectorExpr) {
	for {
		switch x := e.(type) {
		case *syntax.IdentExpr:
			return x, nil
		case *syntax.IndexExpr:
			e = x.X
		case *syntax.SelectorExpr:
			return nil, x
		default:
			return nil, nil
		}
	}
}

// ifStmt is an if statement (TYPES.md §6.6).
func (c *checker) ifStmt(env *env, s *syntax.IfStmt) (*env, bool) {
	tf, ff := c.cond(env, s.Cond)
	thenEnv, elseEnv := env.withFacts(tf), env.withFacts(ff)
	thenDone := c.block(thenEnv, s.Then)
	elseStart, elseDone := elseEnv.facts, true
	switch {
	case s.ElseIf != nil:
		after, done := c.ifStmt(elseEnv, s.ElseIf)
		elseStart, elseDone = after.facts, done
	case s.Else != nil:
		elseDone = c.block(elseEnv, s.Else)
	}
	out := env.with()
	switch {
	case thenDone && elseDone:
		out.facts = thenEnv.facts.intersect(elseStart)
	case thenDone:
		out.facts = thenEnv.facts
	case elseDone:
		out.facts = elseStart
	default:
		return env, false
	}
	out.facts = c.killAssigned(out, out.facts, s)
	return out, true
}

// killAssigned removes the facts of every var the statement assigns.
func (c *checker) killAssigned(env *env, f facts, s syntax.Stmt) facts {
	for _, id := range c.assignedVars(env, &syntax.Block{Stmts: []syntax.Stmt{s}}) {
		f = f.kill(id)
	}
	return f
}

// forStmt is `for x in e` (TYPES.md §12.7).
func (c *checker) forStmt(env *env, s *syntax.ForStmt) *env {
	t := c.synth(env, s.Iter)
	le := env.with()
	le.facts = c.killAssigned(env, env.facts, s)
	be := le.push()
	for i, ty := range c.iterTypes(env, s.Vars, s.Iter, t) {
		if i < len(s.Vars) {
			c.declare(be, s.Vars[i], c.newLocal(be, ObjLocal, s.Vars[i], s, ty))
		}
	}
	c.block(be, s.Body)
	return le
}

// whileStmt is `while c { b }`: b under T(c); after the loop F(c), unless b breaks out of it.
func (c *checker) whileStmt(env *env, s *syntax.WhileStmt) *env {
	le := env.with()
	le.facts = c.killAssigned(env, env.facts, s)
	tf, ff := c.cond(le, s.Cond)
	c.block(le.withFacts(tf), s.Body)
	if breaks(s.Body) {
		return le
	}
	return le.withFacts(ff)
}

// breaks reports a `break` of this loop in its body (not of a loop nested in it).
func breaks(b *syntax.Block) bool {
	found := false
	syntax.Inspect(b, func(n syntax.Node) bool {
		switch n.(type) {
		case *syntax.BreakStmt:
			found = true
		case *syntax.ForStmt, *syntax.WhileStmt, *syntax.LambdaExpr:
			return false
		}
		return !found
	})
	return found
}
