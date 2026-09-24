package check

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// matchStmt is a `match` statement (TYPES.md §12.6).
func (c *checker) matchStmt(env *env, s *syntax.MatchStmt) (*env, bool) {
	cov, ok := c.scrutinee(env, s.Scrutinee)
	var pats [][]*syntax.Pattern
	for _, a := range s.Arms {
		pats = append(pats, a.Patterns)
	}
	hasNone := hasNoneArm(pats)
	completes := len(s.Arms) == 0
	for _, a := range s.Arms {
		o := cov.arm(a.Patterns)
		ae := c.armEnv(env, s.Scrutinee, a.Patterns, o, hasNone)
		if a.Block != nil {
			done := c.block(ae, a.Block)
			completes = completes || done
			continue
		}
		c.exprStmt(ae, &syntax.ExprStmt{Bounds: a.Bounds, X: a.X})
		completes = true
	}
	if ok {
		cov.finish(s)
		c.info.Matches[s] = cov.info
	}
	out := env.with()
	out.facts = c.killAssigned(env, env.facts, s)
	return out, completes
}

// returnStmt is `return e` in a function: e checked against the result type; a bare return
// is E3019.
func (c *checker) returnStmt(env *env, s *syntax.ReturnStmt) {
	if env.fn == nil {
		if s.Value != nil {
			c.synth(env, s.Value)
		}
		return
	}
	if s.Value == nil {
		c.report(env, diag.E3019.At(env.span(s), env.fn.name, env.fn.result))
		return
	}
	c.expr(env, s.Value, env.fn.result)
}

// expectStmt is `expect c` (c ⇐ Bool) or `expect v passes|fails|warns …` (EVALUATION.md §10.3).
func (c *checker) expectStmt(env *env, s *syntax.ExpectStmt) {
	if s.Outcome == nil {
		c.expr(env, s.X, types.BoolType)
		return
	}
	c.synth(env, s.X)
	if m, ok := s.Message.(syntax.StrLit); ok {
		c.info.Types[m] = types.StringType
	}
}

// exprStmt is an expression statement: a call to a user function, fail or warn; anything
// else has no effect (E3020).
func (c *checker) exprStmt(env *env, s *syntax.ExprStmt) {
	c.synth(env, s.X)
	call, ok := s.X.(*syntax.CallExpr)
	if ok {
		callee := c.info.Calls[call]
		switch {
		case callee == nil:
			return
		case callee.Kind == CalleeFn || callee.Kind == CalleeMethod || callee.Kind == CalleeLambda:
			return
		case callee.Kind == CalleeBuiltin && (callee.Builtin == fnFail || callee.Builtin == fnWarn):
			return
		}
	}
	if c.info.Types[s.X].Kind() != types.Error {
		c.report(env, diag.E3020.At(env.span(s.X)))
	}
}

// assignedVars are the ids of the vars of env that the statements of b assign anywhere,
// sorted; a name declared again inside b hides the outer var from there on.
func (c *checker) assignedVars(env *env, b *syntax.Block) []int {
	names := map[string]bool{}
	scanAssigned(b.Stmts, map[string]bool{}, names)
	var ids []int
	for name := range names { //canon:unordered sorted below
		if o := env.lookupLocal(name); o != nil && o.mutable {
			ids = append(ids, o.id)
		}
	}
	slices.Sort(ids)
	return ids
}

// scanAssigned adds to out the names assigned in stmts that are not declared before them in
// the statements' own blocks.
func scanAssigned(stmts []syntax.Stmt, declared, out map[string]bool) {
	local := maps.Clone(declared)
	for _, s := range stmts {
		scanStmt(s, local, out)
	}
}

// scanIf scans the branches of an if statement.
func scanIf(s *syntax.IfStmt, declared, out map[string]bool) {
	scanAssigned(s.Then.Stmts, declared, out)
	if s.ElseIf != nil {
		scanIf(s.ElseIf, declared, out)
	}
	if s.Else != nil {
		scanAssigned(s.Else.Stmts, declared, out)
	}
}

func scanStmt(s syntax.Stmt, declared, out map[string]bool) {
	switch s := s.(type) {
	case *syntax.LetStmt:
		declared[s.Name.Name] = true
	case *syntax.VarStmt:
		declared[s.Name.Name] = true
	case *syntax.AssignStmt:
		if root, _ := assignRoot(s.Target); root != nil && !declared[root.Name] && root == s.Target {
			out[root.Name] = true
		}
	case *syntax.IfStmt:
		scanIf(s, declared, out)
	case *syntax.ForStmt:
		inner := maps.Clone(declared)
		for _, v := range s.Vars {
			inner[v.Name] = true
		}
		scanAssigned(s.Body.Stmts, inner, out)
	case *syntax.WhileStmt:
		scanAssigned(s.Body.Stmts, declared, out)
	case *syntax.MatchStmt:
		for _, a := range s.Arms {
			if a.Block != nil {
				scanAssigned(a.Block.Stmts, declared, out)
			}
		}
	}
}
