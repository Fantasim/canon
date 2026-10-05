package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// typeFuncCallee is the type function x's callee names, `pkg.F` included, recorded; else nil (TYPES.md §4.3).
func (c *checker) typeFuncCallee(env *env, x *syntax.CallExpr) *types.TypeFunc {
	var o *object
	switch f := x.Fun.(type) {
	case *syntax.IdentExpr:
		o = c.lookup(env, f.Name)
		if c.typeFuncOf(o) == nil {
			return nil
		}
		c.info.Uses[f] = o
	case *syntax.SelectorExpr:
		o = c.qualifiedTypeFunc(env, f)
		if o == nil {
			return nil
		}
		c.info.NameUses[f.Name] = o
	default:
		return nil
	}
	c.dependsOn(env, o)
	c.info.Types[x.Fun] = o.typ
	return c.typeFuncs[o]
}

// qualifiedTypeFunc is the public type function `pkg.F` names, else nil.
func (c *checker) qualifiedTypeFunc(env *env, f *syntax.SelectorExpr) *object {
	if f.X == nil || f.Optional {
		return nil
	}
	q := c.qualifier(env, f.X)
	if q == nil || q.kind != ObjPackage {
		return nil
	}
	m, ok := q.target.names[f.Name.Name]
	if !ok || m.local || c.typeFuncOf(m) == nil {
		return nil
	}
	return m
}

// typeFuncOf is the type function o declares, resolved, or nil.
func (c *checker) typeFuncOf(o *object) *types.TypeFunc {
	if o == nil || o.kind != ObjTypeName {
		return nil
	}
	c.resolveTypeName(o)
	return c.typeFuncs[o]
}

// typeFuncReflection is `F(a…).members` or `.typeName`, E3804 for another member or branch (TYPES.md §4.3).
func (c *checker) typeFuncReflection(env *env, s *syntax.SelectorExpr, x *syntax.CallExpr, fn *types.TypeFunc) types.Type {
	dep := &types.DepUnionType{Fn: fn}
	c.info.Types[x] = dep // typed F(*), with no Callee: it is not a call
	argsOK := c.valueTypeArgs(env, x, fn)
	if c.info.Types[x.Fun].Kind() == types.Error || erroredBranch(fn) {
		return types.ErrorType
	}
	name := s.Name.Name
	if _, enums := enumBranches(fn); !enums || name != membersMember && name != typeNameMember {
		c.report(env, diag.E3804.At(env.span(s.Name), dot+name, fn.Name))
		return types.ErrorType
	}
	reflected := c.enumReflection(s, staticView(dep))
	if !argsOK {
		return types.ErrorType
	}
	return reflected
}

// valueTypeArgs is E3806 for F(a…)'s arguments, ordinary expressions, an optional refused (TYPES.md §4.3).
func (c *checker) valueTypeArgs(env *env, x *syntax.CallExpr, fn *types.TypeFunc) bool {
	if len(x.Args) != len(fn.Params) || namedArg(x.Args) {
		c.wrongArity(env, x, fn.Name, fn.Params)
		c.typeArgsAlone(env, x, fn.Params)
		return false
	}
	ok := true
	for i, a := range x.Args {
		p := fn.Params[i].Type
		t := c.typedAgainst(env, a.Value, p)
		if _, fits := c.convert(t, p); !fits {
			c.wrongArity(env, a.Value, fn.Name, fn.Params)
			ok = false
			continue
		}
		c.accept(env, a.Value, t, p)
	}
	return ok
}

// typeArgsAlone types F(a…)'s arguments in error against the parameters, for their contextual names.
//
// An argument past the last parameter is checked alone.
func (c *checker) typeArgsAlone(env *env, x *syntax.CallExpr, params []*types.Param) {
	for i, a := range x.Args {
		if i < len(params) && !isLambda(a.Value) {
			c.typedAgainst(env, a.Value, params[i].Type)
			continue
		}
		c.argAlone(env, a.Value)
	}
}

// namedArg reports an argument written with a name: a type function's are positional (TYPES.md §11.1).
func namedArg(args []*syntax.Arg) bool {
	for _, a := range args {
		if a.Name != nil {
			return true
		}
	}
	return false
}

// typedAgainst types e against want, its contextual names resolved there, with no finding for a misfit.
//
// The caller reports the misfit its own way.
func (c *checker) typedAgainst(env *env, e syntax.Expr, want types.Type) types.Type {
	t := c.exprNode(env, inner(e), want)
	if t == nil {
		t = types.ErrorType
	}
	for x := e; ; x = x.(*syntax.ParenExpr).X { // a paren has its operand's type (TYPES.md §5.1)
		c.info.Types[x] = t
		if _, isParen := x.(*syntax.ParenExpr); !isParen {
			return t
		}
	}
}

// enumBranches are fn's enums, and whether every branch is one, no Never, optional or application (TYPES.md §4.3).
func enumBranches(fn *types.TypeFunc) ([]*types.EnumType, bool) {
	bs := types.Branches(fn)
	out := make([]*types.EnumType, 0, len(bs))
	for _, b := range bs {
		e, ok := b.Base().(*types.EnumType)
		if !ok {
			return nil, false
		}
		out = append(out, e)
	}
	return out, len(out) > 0
}

// erroredBranch reports a type function with a branch in error, or none: its declaration has the finding.
func erroredBranch(fn *types.TypeFunc) bool {
	bs := types.Branches(fn)
	for _, b := range bs {
		if b == nil || b.Kind() == types.Error {
			return true
		}
	}
	return len(bs) == 0
}

// depEnumMember is an enum value member of a dependent value whose branches are enums (TYPES.md §11.4).
func (c *checker) depEnumMember(fn *types.TypeFunc, name string) (*Selection, types.Type) {
	enums, ok := enumBranches(fn)
	if !ok {
		return nil, nil
	}
	return c.enumValueMember(name, allCodes(enums))
}

// allCodes reports enums that all have `@codes`.
func allCodes(enums []*types.EnumType) bool {
	for _, e := range enums {
		if e.Codes == nil {
			return false
		}
	}
	return true
}

// depCodeless reports `.code` on enum branches not all with `@codes`: E3003, not E3804 (TYPES.md §11.4).
func depCodeless(t types.Type, name string) bool {
	d, ok := t.Base().(*types.DepUnionType)
	if !ok || name != codeMember {
		return false
	}
	_, enums := enumBranches(d.Fn)
	return enums
}
