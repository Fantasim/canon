package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// qualifier is the type or package a receiver names in a qualified form (TYPES.md §4.3).
func (c *checker) qualifier(env *env, x syntax.Expr) *object {
	switch x := x.(type) {
	case *syntax.IdentExpr:
		if env.lookupLocal(x.Name) != nil || env.lookupRecord(x.Name) != nil {
			return nil
		}
		o := c.lookupGlobal(env, x.Name)
		if o == nil || !qualifierKind(o) {
			return nil
		}
		c.info.Uses[x] = o
		c.dependsOn(env, o)
		if o.kind != ObjPackage {
			c.info.Types[x] = c.typeOfName(env, o, nil)
		}
		return o
	case *syntax.SelectorExpr:
		return c.qualifiedType(env, x)
	}
	return nil
}

// qualifierKind reports a package or a type name.
func qualifierKind(o *object) bool {
	return o.kind == ObjPackage || o.kind == ObjTypeName || (o.kind == ObjBuiltin && o.typ != nil)
}

// qualifiedType is `pkg.T` used as a qualifier: a public type of an imported package.
func (c *checker) qualifiedType(env *env, s *syntax.SelectorExpr) *object {
	if s.X == nil || s.Optional {
		return nil
	}
	q := c.qualifier(env, s.X)
	if q == nil || q.kind != ObjPackage {
		return nil
	}
	m, ok := q.target.names[s.Name.Name]
	if !ok || m.local || m.kind != ObjTypeName {
		return nil
	}
	c.info.NameUses[s.Name] = m
	c.dependsOn(env, m)
	c.info.Types[s] = c.typeOfName(env, m, nil)
	return m
}

// qualifiedValue is what a qualified form names, `E.members` included (STDLIB.md §3); it has no Selection.
func (c *checker) qualifiedValue(env *env, s *syntax.SelectorExpr, q *object) types.Type {
	if q.kind == ObjPackage {
		return c.packageMember(env, s, q)
	}
	switch t := c.typeOfName(env, q, nil).(type) {
	case *types.EnumType:
		if s.Name.Name == membersMember {
			c.info.NameUses[s.Name] = c.builtins[membersMember]
			return &types.ListType{Elem: t}
		}
		if m := c.memberObject(t, s.Name.Name); m != nil {
			c.info.NameUses[s.Name] = m
			c.deprecatedUse(env, s.Name, m)
			return t
		}
		c.report(env, diag.E3003.At(env.span(s.Name), t, diag.KindMember, s.Name.Name))
	case *types.VariantType:
		if cs := c.caseObject(t, s.Name.Name); cs != nil {
			c.info.NameUses[s.Name] = cs
			c.deprecatedUse(env, s.Name, cs)
			c.bareCase(env, s, cs)
			return cs.typ
		}
		c.report(env, diag.E3003.At(env.span(s.Name), t, diag.KindCase, s.Name.Name))
	default:
		c.report(env, diag.E2110.AtType(env.span(s.X), q.name))
	}
	return types.ErrorType
}

// packageMember is `pkg.Name` in value position: a public declaration of the package.
func (c *checker) packageMember(env *env, s *syntax.SelectorExpr, q *object) types.Type {
	m, ok := q.target.names[s.Name.Name]
	if !ok || m.local {
		c.report(env, diag.E2004.At(env.span(s.Name), q.target.path, s.Name.Name))
		return types.ErrorType
	}
	c.info.NameUses[s.Name] = m
	c.dependsOn(env, m)
	switch m.kind {
	case ObjConst:
		return c.constType(m)
	case ObjLet:
		return staticView(c.letType(m))
	case ObjFn:
		c.textUse(env, s, m, false)
		return m.typ
	default:
	}
	c.report(env, diag.E2110.AtType(env.span(s), m.name))
	return types.ErrorType
}
