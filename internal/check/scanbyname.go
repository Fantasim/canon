package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// scanMethods are the methods whose scan of `E.members` by name warns (W3002, TYPES.md §4.3).
var scanMethods = []string{methodFirst, methodAny, methodFilter}

// scanByName is W3002 at `E.members.m(p)`, p's body or an `and` operand of it `m.name == x`, x free of m (TYPES.md §4.3).
func (c *checker) scanByName(env *env, x *syntax.CallExpr, s *syntax.SelectorExpr) {
	if !slices.Contains(scanMethods, s.Name.Name) || len(x.Args) != 1 {
		return
	}
	enum := c.membersOf(s.X)
	lam, ok := inner(x.Args[0].Value).(*syntax.LambdaExpr)
	if enum == nil || !ok || len(lam.Params) != 1 {
		return
	}
	param := c.info.Defs[lam.Params[0]]
	if param == nil {
		return
	}
	for _, op := range conjuncts(lam.Body) {
		if other := c.nameCompared(op, param); other != nil {
			c.warn(env, diag.W3002.At(env.span(x), env.written(enum), env.span(other)))
			return
		}
	}
}

// membersOf is E in `E.members`, E naming an enum; nil for any other receiver, `F(a…).members` included.
func (c *checker) membersOf(x syntax.Expr) syntax.Expr {
	m, ok := inner(x).(*syntax.SelectorExpr)
	if !ok || m.X == nil || m.Name.Name != membersMember || c.info.NameUses[m.Name] != Object(c.builtins[membersMember]) {
		return nil
	}
	if _, isCall := inner(m.X).(*syntax.CallExpr); isCall {
		return nil
	}
	return m.X
}

// conjuncts are e's operands across a chain of `and`, parentheses removed; e alone otherwise.
func conjuncts(e syntax.Expr) []syntax.Expr {
	b, ok := inner(e).(*syntax.BinaryExpr)
	if !ok || b.Op != syntax.KwAnd {
		return []syntax.Expr{inner(e)}
	}
	return append(conjuncts(b.X), conjuncts(b.Y)...)
}

// nameCompared is x in `m.name == x` or `x == m.name`, m the parameter, parentheses removed; else nil.
func (c *checker) nameCompared(e syntax.Expr, param Object) syntax.Expr {
	b, ok := e.(*syntax.BinaryExpr)
	if !ok || b.Op != syntax.TokEq {
		return nil
	}
	switch {
	case c.readsName(b.X, param) && c.lookupKey(b.Y, param):
		return inner(b.Y)
	case c.readsName(b.Y, param) && c.lookupKey(b.X, param):
		return inner(b.X)
	}
	return nil
}

// lookupKey reports x fit for `E.named(x)`: a String, not optional, not naming the parameter.
func (c *checker) lookupKey(x syntax.Expr, param Object) bool {
	t := c.info.Types[inner(x)]
	if t == nil || t.Kind() == types.Error || c.mentions(x, param) {
		return false
	}
	_, fits := c.convert(t, types.StringType)
	return fits
}

// readsName reports `m.name`, the built-in member read on the parameter m.
func (c *checker) readsName(e syntax.Expr, param Object) bool {
	s, ok := inner(e).(*syntax.SelectorExpr)
	if !ok || s.Optional || s.Name.Name != nameMember {
		return false
	}
	id, isIdent := inner(s.X).(*syntax.IdentExpr)
	sel := c.info.Selections[s]
	return isIdent && c.info.Uses[id] == param && sel != nil && sel.Kind == SelBuiltinMember
}

// mentions reports an expression naming the parameter anywhere.
func (c *checker) mentions(e syntax.Expr, param Object) bool {
	found := false
	syntax.Inspect(e, func(n syntax.Node) bool {
		if id, ok := n.(*syntax.IdentExpr); ok && c.info.Uses[id] == param {
			found = true
		}
		return !found
	})
	return found
}
