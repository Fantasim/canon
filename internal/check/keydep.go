package check

import (
	"cmp"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// keyDependent names how map e's values depend on its key, in t or e's declared type (TYPES.md §11.5).
func (c *checker) keyDependent(e syntax.Expr, t types.Type) string {
	if name := keyApp(t); name != "" {
		return name
	}
	return keyApp(c.declared(e))
}

// keyApp names the first application in t that reads a dependent map's key, through optionals,
// lists, maps, dependent maps, pairs and literal unions; "" for none.
func keyApp(t types.Type) string {
	if t == nil {
		return ""
	}
	switch x := t.Base().(type) {
	case *types.OptionalType:
		return keyApp(x.Elem)
	case *types.ListType:
		return keyApp(x.Elem)
	case *types.MapType:
		return cmp.Or(keyApp(x.Key), keyApp(x.Value))
	case *types.DepMapType:
		return keyApp(x.Value)
	case *types.PairType:
		return cmp.Or(keyApp(x.A), keyApp(x.B))
	case *types.LitUnionType:
		return keyApp(x.Of)
	case *types.AppliedRecord:
		return readsKey(x.Args, x.Rec.Name)
	case *types.TypeAppType:
		return readsKey(x.Args, x.Fn.Name)
	}
	return ""
}

// readsKey is name when an argument reads a dependent map's key, else "".
func readsKey(args []*types.Arg, name string) string {
	for _, a := range args {
		if a.Source == types.ArgKey {
			return name
		}
	}
	return ""
}

// unionOnKeyDependent is E3804 for union on a map whose values depend on its key, its arguments checked silently (DECISIONS 338).
func (c *checker) unionOnKeyDependent(env *env, x *syntax.CallExpr, s *syntax.SelectorExpr, recv types.Type) bool {
	if s.Name.Name != methodUnion || recv.Base().Kind() != types.Map && recv.Base().Kind() != types.DepMap {
		return false
	}
	name := c.keyDependent(s.X, recv)
	if name == "" {
		return false
	}
	c.report(env, diag.E3804.At(env.span(s.Name), methodUnion, name))
	for _, a := range x.Args {
		c.expr(env, a.Value, types.ErrorType)
	}
	return true
}

// declared is the type e is declared with, before its static view: a name's, a field's, or an
// element or value of one; the any type for another expression.
func (c *checker) declared(e syntax.Expr) types.Type {
	switch x := inner(e).(type) {
	case *syntax.IdentExpr:
		o, _ := c.info.Uses[x].(*object)
		return c.declaredOf(o)
	case *syntax.SelectorExpr:
		if sel := c.info.Selections[x]; sel != nil && sel.Kind == SelField {
			o, _ := sel.Obj.(*object)
			return c.declaredOf(o)
		}
	case *syntax.IndexExpr:
		return elemOrValue(c.declared(x.X))
	}
	return types.AnyType
}

// declaredOf is the declared type of what o names, the any type for none.
func (c *checker) declaredOf(o *object) types.Type {
	switch {
	case o == nil:
	case o.kind == ObjField && o.field != nil:
		return orAny(o.field.Type)
	case o.kind == ObjLet:
		return orAny(c.letType(o))
	case o.kind == ObjLocal || o.kind == ObjParam:
		return orAny(o.typ)
	}
	return types.AnyType
}

// elemOrValue is a list's element or a map's value type, the any type for another.
func elemOrValue(t types.Type) types.Type {
	switch x := t.Base().(type) {
	case *types.ListType:
		return x.Elem
	case *types.MapType:
		return x.Value
	case *types.DepMapType:
		return x.Value
	}
	return types.AnyType
}

// orAny is t, the any type for nil.
func orAny(t types.Type) types.Type {
	if t == nil {
		return types.AnyType
	}
	return t
}
