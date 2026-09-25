package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// unionJob is a literal union's alternative whose wire form is judged once its refs resolve.
type unionJob struct {
	env *env
	at  syntax.Node
	of  types.Type
}

// resolveUnion is `A | "lit" | …` (TYPES.md §13.2).
func (c *checker) resolveUnion(tc *typeCtx, t *syntax.UnionType) types.Type {
	of := c.resolveType(tc.element(), t.Alts[0])
	u := &types.LitUnionType{Of: of}
	for _, alt := range t.Alts[1:] {
		lit, ok := alt.(*syntax.LiteralType)
		if !ok {
			c.resolveType(tc.element(), alt)
			c.report(tc.env, diag.E3002.At(tc.env.span(alt), types.StringType, c.info.TypeExprs[alt]))
			continue
		}
		c.info.TypeExprs[alt] = types.StringType
		c.info.Types[lit.Value] = types.StringType
		u.Literals = append(u.Literals, constText(lit.Value))
	}
	j := unionJob{env: tc.env, at: t.Alts[0], of: of}
	if c.bodies {
		c.unionWire(j)
	} else {
		c.unions = append(c.unions, j)
	}
	return u
}

// checkUnions judges the literal unions written in p's types, its refs now resolved; judging one may resolve more.
func (c *checker) checkUnions(p *pkgState) {
	for {
		var mine, rest []unionJob
		for _, j := range c.unions {
			if j.env.pkg == p {
				mine = append(mine, j)
			} else {
				rest = append(rest, j)
			}
		}
		if len(mine) == 0 {
			return
		}
		c.unions = rest
		for _, j := range mine {
			c.unionWire(j)
		}
	}
}

// unionWire is E3002 for a union's alternative, or a branch of it, without a string wire form.
func (c *checker) unionWire(j unionJob) {
	if !c.stringWire(j.of) {
		c.report(j.env, diag.E3002.At(j.env.span(j.at), types.StringType, j.of))
		return
	}
	if fn := depFunc(j.of); fn != nil {
		if bad, ok := c.stringBranches(fn); !ok {
			c.report(j.env, diag.E3002.At(j.env.span(j.at), types.StringType, bad))
		}
	}
}

// stringWire reports a string wire form: String, an enum, Never, a dependent type, or a ref keyed by one (WIRE.md §5.9).
func (c *checker) stringWire(t types.Type) bool {
	seen := map[*types.Collection]bool{}
	for {
		switch t.Base().Kind() {
		case types.String, types.Enum, types.Never, types.TypeApp, types.DepUnion, types.Error:
			return true
		case types.Ref:
		default:
			return false
		}
		coll := c.coll(t.Base().(*types.RefType))
		if coll.KeyedBy == nil {
			return true
		}
		if seen[coll] {
			return false
		}
		seen[coll] = true
		t = c.fieldType(coll.KeyedBy)
	}
}

// stringBranches reports every branch of fn having a string wire form (TYPES.md §13.2), else the first that has none.
func (c *checker) stringBranches(fn *types.TypeFunc) (types.Type, bool) {
	bad := firstBranch(fn, func(b types.Type) bool {
		return b.Base().Kind() != types.LitUnion && !c.stringWire(b)
	})
	return bad, bad == nil
}

// unionOver is the literal union a value compares as when it is one over a dependent type, or
// an application of a type function whose body is a literal union; nil otherwise.
func unionOver(t types.Type) *types.LitUnionType {
	switch x := t.Base().(type) {
	case *types.LitUnionType:
		if depFunc(x.Of) != nil {
			return x
		}
	case *types.DepUnionType:
		return types.UnionBody(x.Fn)
	}
	return nil
}

// unionEquality is `==`/`!=` with such a union: a literal of it, itself or its alternative, else E3002 or E3804 (TYPES.md §13.2).
func (c *checker) unionEquality(env *env, e *syntax.BinaryExpr, a, b types.Type) bool {
	u, ut, other, ox := unionOver(a), a, b, e.Y
	if u == nil {
		u, ut, other, ox = unionOver(b), b, a, e.X
	}
	if u == nil || types.Identical(a, b) {
		return u != nil
	}
	_, isStr := inner(ox).(syntax.StrLit)
	switch {
	case isStr && c.unionLiteral(ox, u):
	case !isStr && c.unionMember(u, other):
	case !isStr && depFunc(u.Of) != nil:
		c.report(env, diag.E3804.At(env.span(e), e.Op.String(), unionName(ut, u)))
	default:
		c.report(env, diag.E3002.At(env.span(ox), ut, other))
	}
	return true
}

// unionMember reports a value of a union's alternative: the same dependent value, or one
// assignable to a plain alternative.
func (c *checker) unionMember(u *types.LitUnionType, t types.Type) bool {
	_, fns, last := unionChain(u)
	if len(fns) > 0 {
		return t.Base().Kind() == types.DepUnion && slices.Contains(fns, depFunc(t))
	}
	return c.assignable(t, last)
}

// unionName is the type function E3804 names for a union value: its own, or its alternative's.
func unionName(t types.Type, u *types.LitUnionType) string {
	if n := depName(t); n != "" {
		return n
	}
	return depName(u.Of)
}
