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

// resolveUnion is `A | "lit" | …`: a type after the first alternative is E3028, unless in error (TYPES.md §13.2, §1).
func (c *checker) resolveUnion(tc *typeCtx, t *syntax.UnionType) types.Type {
	first := tc.element()
	first.pos &^= posLiteral // a nested union's A is a type again
	of := c.resolveType(first, t.Alts[0])
	u := &types.LitUnionType{Of: of}
	for _, alt := range t.Alts[1:] {
		lit, ok := literalAlt(alt)
		if !ok {
			if typ := c.resolveType(tc.literalAlt(), alt); !holdsError(typ) {
				c.report(tc.env, diag.E3028.At(tc.env.span(alt), typ))
			}
			continue
		}
		c.stringAlt(alt, lit)
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

// literalAlt is a later union alternative's context, a literal in it a String (TYPES.md §13.2).
func (tc *typeCtx) literalAlt() *typeCtx {
	in := tc.element()
	in.pos |= posLiteral
	return in
}

// unparen is t without the parentheses around it.
func unparen(t syntax.Type) syntax.Type {
	for {
		p, ok := t.(*syntax.ParenType)
		if !ok {
			return t
		}
		t = p.Type
	}
}

// literalAlt is a union alternative written as a string literal, parentheses allowed (TYPES.md §13.2).
func literalAlt(alt syntax.Type) (*syntax.LiteralType, bool) {
	lit, ok := unparen(alt).(*syntax.LiteralType)
	return lit, ok
}

// stringAlt records a literal alternative, and each parenthesis around it, as a String.
func (c *checker) stringAlt(alt syntax.Type, lit *syntax.LiteralType) {
	for t := alt; t != syntax.Type(lit); t = t.(*syntax.ParenType).Type {
		c.info.TypeExprs[t] = types.StringType
	}
	c.info.TypeExprs[lit] = types.StringType
	c.info.Types[lit.Value] = types.StringType
}

// checkUnions judges the literal unions written in p's types, its refs now resolved; judging one may resolve more.
func (c *checker) checkUnions(p *pkgState) {
	drainPkgJobs(&c.unions, p, func(j unionJob) *pkgState { return j.env.pkg }, c.unionWire)
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

// stringWire reports a string wire form: String, an enum without @json(codes), Never, a dependent type, a ref keyed by one (WIRE.md §5.9).
func (c *checker) stringWire(t types.Type) bool {
	seen := map[*types.Collection]bool{}
	for {
		switch t.Base().Kind() {
		case types.String, types.Never, types.TypeApp, types.DepUnion, types.Error:
			return true
		case types.Enum:
			return !t.Base().(*types.EnumType).WireCodes
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

// unionOver is t's literal union when it reaches a dependent type, else nil (TYPES.md §13.2).
func unionOver(t types.Type) *types.LitUnionType {
	u, ok := t.Base().(*types.LitUnionType)
	if !ok {
		return nil
	}
	if _, fns, _ := unionChain(u); len(fns) == 0 {
		return nil
	}
	return u
}

// unionValues is E3007 for `==`/`!=` between two values of one literal union, neither a literal nor an `A` (TYPES.md §13.2).
func (c *checker) unionValues(env *env, e *syntax.BinaryExpr, tx, ty types.Type) bool {
	if !c.unionValue(e.X, tx) || !c.unionValue(e.Y, ty) || !types.Identical(optElem(tx), optElem(ty)) {
		return false
	}
	c.report(env, diag.E3007.AtBinary(env.span(e), e.Op.String(), tx, ty))
	return true
}

// unionValue reports an operand typed as a literal union, optional or not, that is neither a
// string literal nor a symbol: a union value, not one of its literals or an `A` written bare.
func (c *checker) unionValue(e syntax.Expr, t types.Type) bool {
	if optElem(t).Base().Kind() != types.LitUnion {
		return false
	}
	switch x := inner(e).(type) {
	case syntax.StrLit:
		return false
	case *syntax.IdentExpr:
		return !c.info.Symbols[x]
	}
	return true
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
