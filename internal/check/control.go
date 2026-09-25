package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// ifExpr is `if c { a } else { b }` (TYPES.md §5.1, §6.6).
func (c *checker) ifExpr(env *env, e *syntax.IfExpr, want types.Type) types.Type {
	tf, ff := c.cond(env, e.Cond)
	thenEnv, elseEnv := env.withFacts(tf), env.withFacts(ff)
	var elseX syntax.Expr = e.ElseIf
	if e.Else != nil {
		elseX = e.Else.X
	}
	if want != nil {
		c.expr(thenEnv, e.Then.X, want)
		c.expr(elseEnv, elseX, want)
		return want
	}
	ts := []types.Type{c.synth(thenEnv.join(), e.Then.X), c.synth(elseEnv.join(), elseX)}
	t, _ := c.joinTypes(env, e, []syntax.Expr{e.Then.X, elseX}, ts)
	return t
}

// scrutinee types a match's scrutinee and starts its coverage: an enum, a variant, Bool, a
// Kind or an optional of these (E3604).
func (c *checker) scrutinee(env *env, x syntax.Expr) (*coverage, bool) {
	t := c.synth(env, x)
	cov := c.newCoverage(env, t)
	if t.Kind() == types.Error || c.notDependent(env, x, t, syntax.KwMatch.String()) {
		return cov, false
	}
	if !cov.matchable() {
		c.report(env, diag.E3604.At(env.span(x), t))
		return cov, false
	}
	return cov, true
}

// armEnv is the env of a match arm (TYPES.md §6.6).
func (c *checker) armEnv(env *env, x syntax.Expr, ps []*syntax.Pattern, o *object, hasNone bool) *env {
	ae := env.push()
	key, root, stable := c.pathKey(env, x)
	if stable && hasNone && !hasNoneArm([][]*syntax.Pattern{ps}) {
		ae = ae.withFacts(facts{key: {root: root}})
	}
	if o == nil || o.kind != ObjCase {
		return ae
	}
	if stable {
		ae = ae.withFacts(facts{key: {root: root, kase: o.typ.(*types.CaseType)}})
	}
	if b := ps[0].Binder; b != nil && len(ps) == 1 {
		c.declare(ae, b, c.newLocal(ae, ObjLocal, b, ps[0], o.typ))
	}
	return ae
}

// hasNoneArm reports a `none` pattern among the arms' patterns.
func hasNoneArm(arms [][]*syntax.Pattern) bool {
	for _, ps := range arms {
		for _, p := range ps {
			if p.Keyword == syntax.KwNone {
				return true
			}
		}
	}
	return false
}

// matchExpr is a `match` in value position (TYPES.md §12.6): arms checked against the expected type, else joined.
func (c *checker) matchExpr(env *env, e *syntax.MatchExpr, want types.Type) types.Type {
	cov, ok := c.scrutinee(env, e.Scrutinee)
	var pats [][]*syntax.Pattern
	for _, a := range e.Arms {
		pats = append(pats, a.Patterns)
	}
	hasNone := hasNoneArm(pats)
	var bodies []syntax.Expr
	var ts []types.Type
	for _, a := range e.Arms {
		o := cov.arm(a.Patterns)
		ae := c.armEnv(env, e.Scrutinee, a.Patterns, o, hasNone)
		bodies = append(bodies, a.Body)
		if want != nil {
			ts = append(ts, c.expr(ae, a.Body, want))
		} else {
			ts = append(ts, c.synth(ae.join(), a.Body))
		}
	}
	if ok {
		cov.finish(e)
		c.info.Matches[e] = cov.info
	}
	if want != nil || len(ts) == 0 {
		return cmpType(want)
	}
	t, _ := c.joinTypes(env, e, bodies, ts)
	return t
}
