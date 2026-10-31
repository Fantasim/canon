package check

import (
	"cmp"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// keyDependent names how map e's values depend on its key, in t or e's declared type (TYPES.md §11.5).
func (c *checker) keyDependent(e syntax.Expr, t types.Type) string {
	if name := c.keyApp(t); name != "" {
		return name
	}
	return c.keyApp(c.declared(e))
}

// keyApp names the first application in t reading a dependent map's key outside a kept one (TYPES.md §11.5); "" for none.
func (c *checker) keyApp(t types.Type) string {
	if t == nil {
		return ""
	}
	switch x := t.Base().(type) {
	case *types.OptionalType:
		return c.keyApp(x.Elem)
	case *types.ListType:
		return c.keyApp(x.Elem)
	case *types.MapType:
		return cmp.Or(c.keyApp(x.Key), c.keyApp(x.Value))
	case *types.DepMapType:
		if c.keptDepMap(x) {
			return ""
		}
		return c.keyApp(x.Value)
	case *types.PairType:
		return cmp.Or(c.keyApp(x.A), c.keyApp(x.B))
	case *types.LitUnionType:
		return c.keyApp(x.Of)
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

// keptUnion is union on a kept dependent map: b is checked against, and the call returns, its type (STDLIB.md §6).
func (c *checker) keptUnion(env *env, x *syntax.CallExpr, s *syntax.SelectorExpr, recv types.Type) (types.Type, bool) {
	d, isDep := recv.Base().(*types.DepMapType)
	if s.Name.Name != methodUnion || !isDep || !c.keptDepMap(d) {
		return nil, false
	}
	if c.misplacedArgs(env, x) {
		return types.ErrorType, true
	}
	b := newBinding(c.unbroken)
	b.bind(tK, &types.RefType{Target: d.Coll})
	b.bind(tV, c.staticView(d.Value))
	bc := &builtinCall{x: x, fun: s, name: methodUnion, kind: CalleeBuiltin, b: b}
	r := rowsNamed(mapMethods, methodUnion)[0]
	if _, fits := argMap(x, r.sig); !fits {
		c.arityError(env, bc, r.sig)
		c.argsAlone(env, x)
		return types.ErrorType, true
	}
	c.expr(env, x.Args[0].Value, recv)
	c.info.Calls[x] = &Callee{Kind: CalleeBuiltin, Builtin: methodUnion, Overload: r.overload, TypeArgs: b.typeArgs()}
	c.info.Types[s] = &types.FuncType{Params: []types.Type{recv}, Result: recv}
	return recv, true
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
