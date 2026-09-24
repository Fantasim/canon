package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// The types of `[]` and `{}` synthesized where a join may still give them one (TYPES.md §6.4).
var (
	emptyList = &types.ListType{Elem: types.NeverType}
	emptyMap  = &types.MapType{Key: types.NeverType, Value: types.NeverType}
)

// listLit is `[a, b, …]` (TYPES.md §5.1).
func (c *checker) listLit(env *env, e *syntax.ListLit, want types.Type) types.Type {
	if l, ok := unwrap(want).(*types.ListType); ok {
		for _, x := range e.Elems {
			c.expr(env, x, l.Elem)
		}
		return l
	}
	if unknownContext(want) {
		for _, x := range e.Elems {
			c.expr(env, x, types.ErrorType)
		}
		return types.ErrorType
	}
	if len(e.Elems) == 0 {
		if env.joining() {
			return emptyList
		}
		c.report(env, diag.E3008.At(env.span(e)))
		return types.ErrorType
	}
	elem, ok := c.joinAll(env, e, e.Elems)
	if !ok {
		return types.ErrorType
	}
	return &types.ListType{Elem: elem}
}

// joinAll synthesizes branches that must agree and joins their types (§6.4).
func (c *checker) joinAll(env *env, at syntax.Node, branches []syntax.Expr) (types.Type, bool) {
	je := env.join()
	ts := make([]types.Type, len(branches))
	for i, b := range branches {
		ts[i] = c.synth(je, b)
	}
	return c.joinTypes(env, at, branches, ts)
}

// joinTypes joins already typed branches; an erroneous branch makes it the error type, silently.
func (c *checker) joinTypes(env *env, at syntax.Node, branches []syntax.Expr, ts []types.Type) (types.Type, bool) {
	if slices.ContainsFunc(ts, c.erroneous) {
		return types.ErrorType, false
	}
	j := ts[0]
	for i := 1; i < len(ts); i++ {
		next, ok := c.join2(j, ts[i], branches[:i], branches[i])
		if !ok {
			c.report(env, diag.E3308.At(env.span(branches[i]), j, ts[i]))
			return types.ErrorType, false
		}
		j = next
	}
	if j == emptyList || j == emptyMap || j.Kind() == types.None {
		if env.joining() {
			return j, true
		}
		c.report(env, diag.E3008.At(env.span(at)))
		return types.ErrorType, false
	}
	for i, b := range branches {
		if ts[i].Kind() != types.Error {
			c.accept(env, b, ts[i], j)
		}
	}
	return j, true
}

// join2 joins two branch types with the literal rows of §6.4.
func (c *checker) join2(a, b types.Type, before []syntax.Expr, next syntax.Expr) (types.Type, bool) {
	switch {
	case a == emptyList && b.Base().Kind() == types.List, a == emptyMap && b.Base().Kind() == types.Map:
		return b, true
	case b == emptyList && a.Base().Kind() == types.List, b == emptyMap && a.Base().Kind() == types.Map:
		return a, true
	case a.Base().Kind() == types.Float && isIntLiteral(next):
		return a, true
	case b.Base().Kind() == types.Float && allIntLiterals(before):
		return b, true
	}
	return types.Join(a, b)
}

func allIntLiterals(es []syntax.Expr) bool {
	for _, e := range es {
		if !isIntLiteral(e) {
			return false
		}
	}
	return true
}

// listComp is `[elem clauses]` (TYPES.md §6.6).
func (c *checker) listComp(env *env, e *syntax.ListComp, want types.Type) types.Type {
	inner := c.clauses(env, e.Clauses)
	if l, ok := unwrap(want).(*types.ListType); ok {
		c.expr(inner, e.Elem, l.Elem)
		return &types.ListType{Elem: l.Elem}
	}
	return &types.ListType{Elem: c.synth(inner, e.Elem)}
}

// clauses checks comprehension clauses in a new scope: `for`, `if` and `let`.
func (c *checker) clauses(env *env, cs []*syntax.CompClause) *env {
	inner := env.push()
	for _, cl := range cs {
		switch cl.Keyword {
		case syntax.KwFor:
			c.iterate(inner, cl.Vars, cl.X, cl)
		case syntax.KwIf:
			tf, _ := c.cond(inner, cl.X)
			inner = inner.withFacts(tf)
		case syntax.KwLet:
			t := c.synth(inner, cl.X)
			if len(cl.Vars) > 0 {
				c.declare(inner, cl.Vars[0], c.newLocal(inner, ObjLocal, cl.Vars[0], cl, t))
			}
		default:
		}
	}
	return inner
}

// iterate types `for x in e` or `for a, b in e` (TYPES.md §12.7) and binds the variables in env's scope.
func (c *checker) iterate(env *env, vars []*syntax.Ident, x syntax.Expr, decl syntax.Node) {
	t := c.synth(env, x)
	ts := c.iterTypes(env, vars, x, t)
	for i, v := range vars {
		c.declare(env, v, c.newLocal(env, ObjLocal, v, decl, ts[i]))
	}
}

// iterTypes are the types of the loop variables over a value of type t: one name over a
// list, keyed list, table or Range; two over a map or a sequence of pairs (E3018 otherwise).
func (c *checker) iterTypes(env *env, vars []*syntax.Ident, x syntax.Expr, t types.Type) []types.Type {
	errs := []types.Type{types.ErrorType, types.ErrorType}
	if t.Kind() == types.Error {
		return errs
	}
	if t.Base().Kind() == types.Optional {
		c.report(env, diag.E3402.At(env.span(x), env.span(x)))
		return errs
	}
	if len(vars) == pairArity {
		return c.pairIter(env, vars, x, t)
	}
	switch b := t.Base().(type) {
	case *types.ListType:
		return []types.Type{b.Elem}
	case *types.TableType:
		return []types.Type{b.Elem}
	}
	if t.Kind() == types.Range {
		return []types.Type{types.IntType}
	}
	if k := t.Base().Kind(); (k == types.Map || k == types.DepMap) && len(vars) == 1 {
		c.report(env, diag.E3018.At(env.span(x), vars[0].Name, "", t))
		return errs
	}
	c.report(env, diag.E3002.At(env.span(x), listT(types.AnyType), t))
	return errs
}

// pairIter is `for a, b in e`: a map's key and value, or a sequence of pairs' components.
func (c *checker) pairIter(env *env, vars []*syntax.Ident, x syntax.Expr, t types.Type) []types.Type {
	switch b := t.Base().(type) {
	case *types.MapType:
		return []types.Type{b.Key, staticView(b.Value)}
	case *types.DepMapType:
		return []types.Type{&types.RefType{Target: b.Coll}, staticView(b.Value)}
	}
	if elem, ok := seqElem(t); ok {
		if p, isPair := elem.Base().(*types.PairType); isPair {
			return []types.Type{p.A, p.B}
		}
	}
	c.report(env, diag.E3018.At(env.span(x), vars[0].Name, vars[1].Name, t))
	return []types.Type{types.ErrorType, types.ErrorType}
}
