package check

import (
	"maps"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// typeArgRoot is what a type argument may start from (TYPES.md §11.1).
type typeArgRoot struct {
	param  *types.Param
	field  *types.Field
	binder string
	coll   *types.Collection
	obj    *object
}

// resolveTypeFunc is `type F(p: P, …) = type` (TYPES.md §11.2).
func (c *checker) resolveTypeFunc(o *object, td *syntax.TypeDecl) types.Type {
	if o.state == stateResolving && c.records == c.funcDepth[o] {
		return c.typeFuncCycle(o, td)
	}
	if o.state != stateNone {
		return o.typ
	}
	o.state = stateResolving
	c.funcDepth[o] = c.records
	fn := &types.TypeFunc{Pkg: o.pkg, Name: o.name, Doc: docText(td.Doc), Decl: td}
	c.typeFuncs[o] = fn
	o.typ = &types.DepUnionType{Fn: fn}
	env := c.declEnv(o)
	tc := &typeCtx{env: env, scope: map[string]typeArgRoot{}}
	fn.Params = c.typeParams(tc, td.Params)
	tc.fnBody = true
	if m, ok := td.Type.(*syntax.MatchType); ok {
		c.typeMatch(tc, fn, m)
	} else {
		fn.Body = c.resolveType(tc, td.Type)
	}
	if o.state == stateDone {
		fn.Body, fn.Arms, fn.Scrutinee = types.ErrorType, nil, nil
		return o.typ
	}
	o.state = stateDone
	return o.typ
}

// typeFuncCycle is E3021 for a type function reaching itself with no record on the way (TYPES.md §13.1).
func (c *checker) typeFuncCycle(o *object, td *syntax.TypeDecl) types.Type {
	diag.E3021.At(o.file.Span(td.Name), o.name).Report(c.pkgs[o.pkg].bag)
	c.breakObj(o)
	o.state = stateDone
	o.typ = types.ErrorType
	return o.typ
}

// typeMatch is a type-level match: its scrutinee a path of enum or Bool type rooted at a
// parameter (E3803); arms of members, `true`/`false` or `_`, exhaustive (E3601, E3602).
func (c *checker) typeMatch(tc *typeCtx, fn *types.TypeFunc, m *syntax.MatchType) {
	env := tc.env
	c.info.TypeExprs[m] = &types.DepUnionType{Fn: fn}
	sc := *tc
	sc.scrut = true
	arg, t, ok := c.typeArgPath(&sc, m.Scrutinee)
	if !ok || arg.Source != types.ArgParam || !matchable(t) {
		if ok {
			c.report(env, diag.E3803.AtScrutinee(env.span(m.Scrutinee)))
		}
		return
	}
	fn.Scrutinee = &types.Scrutinee{Param: arg.Param, Path: arg.Path, Type: t}
	cov := c.newCoverage(env, t)
	for _, arm := range m.Arms {
		ta := &types.TypeArm{Result: c.resolveType(tc, arm.Type)}
		for _, p := range arm.Patterns {
			if p.Keyword == syntax.TokUnderscore {
				ta.Wildcard = true
			}
			idx, _, _ := cov.pattern(p)
			if !ta.Wildcard {
				ta.Members = append(ta.Members, idx...)
			}
		}
		fn.Arms = append(fn.Arms, ta)
	}
	cov.finish(m)
}

// matchable reports an enum or Bool scrutinee of a type-level match.
func matchable(t types.Type) bool {
	k := t.Base().Kind()
	return k == types.Enum || k == types.Bool
}

// typeArgPath resolves a type argument or scrutinee: a stable path from a root of tc, through
// record fields (a ref dereferences). A later field is E3805, anything else E3803.
func (c *checker) typeArgPath(tc *typeCtx, e syntax.Expr) (*types.Arg, types.Type, bool) {
	env := tc.env
	root, segs := pathParts(e)
	if root == nil {
		c.notTypePath(tc, e)
		return nil, nil, false
	}
	r, ok := tc.scope[root.Name]
	if !ok {
		if tc.later[root.Name] {
			c.report(env, diag.E3805.At(env.span(root), root.Name))
		} else {
			c.notTypePath(tc, e)
		}
		return nil, nil, false
	}
	arg, t := rootArg(r)
	c.info.Uses[root] = r.obj
	c.info.Types[root] = t
	for _, s := range segs {
		if t.Base().Kind() == types.Optional {
			c.notTypePath(tc, e)
			return nil, nil, false
		}
		f := c.fieldOf(env, t, s.Name)
		if f == nil {
			return nil, nil, false
		}
		c.info.Selections[s] = &Selection{Kind: SelField, Obj: c.fieldObjects[f], Recv: t, Deref: t.Base().Kind() == types.Ref}
		c.info.NameUses[s.Name] = c.fieldObjects[f]
		arg.Path = append(arg.Path, f)
		t = c.fieldType(f)
		c.info.Types[s] = t
	}
	return arg, t, true
}

// notTypePath is E3803 for a type argument or a scrutinee not rooted at a parameter (TYPES.md §11.1, §11.2).
func (c *checker) notTypePath(tc *typeCtx, e syntax.Expr) {
	if _, bad := e.(*syntax.BadExpr); bad {
		c.breakObj(tc.env.owner)
		return
	}
	if tc.scrut {
		c.report(tc.env, diag.E3803.AtScrutinee(tc.env.span(e)))
		return
	}
	c.report(tc.env, diag.E3803.AtArgument(tc.env.span(e)))
}

// pathParts splits `a.b.c` into its root and its selectors, root first; nil root otherwise.
func pathParts(e syntax.Expr) (*syntax.IdentExpr, []*syntax.SelectorExpr) {
	var segs []*syntax.SelectorExpr
	for {
		switch x := e.(type) {
		case *syntax.IdentExpr:
			for i, j := 0, len(segs)-1; i < j; i, j = i+1, j-1 {
				segs[i], segs[j] = segs[j], segs[i]
			}
			return x, segs
		case *syntax.SelectorExpr:
			if x.Optional || x.X == nil {
				return nil, nil
			}
			segs = append(segs, x)
			e = x.X
		default:
			return nil, nil
		}
	}
}

// rootArg is the argument a root starts, with the root's type.
func rootArg(r typeArgRoot) (*types.Arg, types.Type) {
	switch {
	case r.param != nil:
		return &types.Arg{Source: types.ArgParam, Param: r.param}, r.param.Type
	case r.field != nil:
		return &types.Arg{Source: types.ArgField, Path: []*types.Field{r.field}}, r.field.Type
	}
	return &types.Arg{Source: types.ArgKey, Binder: r.binder}, &types.RefType{Target: r.coll}
}

// fieldOf is the field name of a record (or of a ref's entry), E3003 when there is none.
func (c *checker) fieldOf(env *env, t types.Type, name *syntax.Ident) *types.Field {
	b := t.Base()
	if r, ok := b.(*types.RefType); ok {
		b = c.coll(r).Elem.Base()
	}
	rec, ok := b.(*types.RecordType)
	if ok {
		c.completeRecord(rec)
		if f := fieldNamed(rec.Fields, name.Name); f != nil {
			return f
		}
	}
	c.report(env, diag.E3003.At(env.span(name), t, diag.KindField, name.Name))
	return nil
}

// applyTypeFunc is `F(args)` (TYPES.md §11.1): a dependent type.
func (c *checker) applyTypeFunc(tc *typeCtx, t *syntax.NamedType, fn *types.TypeFunc) types.Type {
	args, ok := c.typeArgs(tc, t, fn.Name, fn.Params)
	if !ok {
		return types.ErrorType
	}
	return &types.TypeAppType{Fn: fn, Args: args}
}

// applyRecord is `R(args)`, a parameterized record applied (TYPES.md §11.1, TYP-18).
func (c *checker) applyRecord(tc *typeCtx, t *syntax.NamedType, rec *types.RecordType) types.Type {
	c.completeRecord(rec)
	args, ok := c.typeArgs(tc, t, rec.Name, rec.Params)
	if !ok {
		return types.ErrorType
	}
	return &types.AppliedRecord{Rec: rec, Args: args}
}

// typeArgs checks the arity (none written is 0 given) and the type of each argument against its
// parameter (E3806).
func (c *checker) typeArgs(tc *typeCtx, t *syntax.NamedType, name string, params []*types.Param) ([]*types.Arg, bool) {
	env := tc.env
	if t.Args == nil || len(t.Args.Args) != len(params) {
		c.wrongArity(env, arityNode(t), name, params)
		return nil, false
	}
	var out []*types.Arg
	ok := true
	for i, a := range t.Args.Args {
		arg, at, found := c.typeArgPath(tc, a)
		if !found {
			ok = false
			continue
		}
		c.info.Types[a] = at
		if !c.argFits(at, params[i].Type) {
			c.wrongArity(env, a, name, params)
			ok = false
		}
		out = append(out, arg)
	}
	return out, ok
}

// argFits reports an argument fit for its parameter, a ref dereferenced, never an optional (TYPES.md §11.2).
func (c *checker) argFits(at, param types.Type) bool {
	if at.Base().Kind() == types.Optional {
		return false
	}
	return types.Assignable(c.deref(at), param) || types.Assignable(at, param)
}

// wrongArity is E3806 at at: name takes exactly its declared parameters, each of its type (TYPES.md §11.1).
func (c *checker) wrongArity(env *env, at syntax.Node, name string, params []*types.Param) {
	var want []diag.TypeArg
	for _, p := range params {
		want = append(want, p.Type)
	}
	c.report(env, diag.E3806.At(env.span(at), name, int64(len(params)), want))
}

// arityNode is where a wrong arity is reported: the arguments, or the bare name (TYPES.md §11.1).
func arityNode(t *syntax.NamedType) syntax.Node {
	if t.Args == nil {
		return t.Name
	}
	return t.Args
}

// deref is the entry type of a ref, or t itself.
func (c *checker) deref(t types.Type) types.Type {
	if r, ok := t.Base().(*types.RefType); ok {
		return c.coll(r).Elem
	}
	return t
}

// resolveDepMap is `{k in c: T(k)}` (TYPES.md §11.5).
func (c *checker) resolveDepMap(tc *typeCtx, t *syntax.DepMapType) types.Type {
	env := tc.env
	coll := c.domain(env, t.Domain)
	if coll == nil {
		return types.ErrorType
	}
	in := tc.element()
	in.scope = map[string]typeArgRoot{}
	maps.Copy(in.scope, tc.scope)
	bo := c.newObject(ObjLocal, t.Var.Name, env.pkg, t, env.file)
	bo.typ = &types.RefType{Target: coll}
	c.info.Defs[t.Var] = bo
	in.scope[t.Var.Name] = typeArgRoot{binder: t.Var.Name, coll: coll, obj: bo}
	d := &types.DepMapType{Binder: t.Var.Name, Coll: coll, Value: c.resolveType(in, t.Value)}
	if t.Args == nil {
		return d
	}
	return c.refine(tc, d, t.Args)
}

// domain is a dependent map's collection, named as `ref c` names one (TYPES.md §11.5).
func (c *checker) domain(env *env, e syntax.Expr) *types.Collection {
	root, segs := pathParts(e)
	if root == nil {
		c.report(env, diag.E3504.At(env.span(e), ""))
		return nil
	}
	o := c.lookupGlobal(env, root.Name)
	if o == nil {
		c.unknownName(env, root, root.Name)
		return nil
	}
	c.info.Uses[root] = o
	if o.kind == ObjPackage && len(segs) > 0 {
		m, ok := o.target.names[segs[0].Name.Name]
		if !ok || m.local {
			c.report(env, diag.E2004.At(env.span(segs[0].Name), o.target.path, segs[0].Name.Name))
			return nil
		}
		c.info.NameUses[segs[0].Name] = m
		o, segs = m, segs[1:]
	}
	if o.kind != ObjLet {
		c.report(env, diag.E3504.At(env.span(e), env.literalText(e).String()))
		return nil
	}
	c.dependsOn(env, o)
	q := &syntax.QualifiedName{Bounds: syntax.Bounds{From: e.First(), To: e.Last()}, Parts: []*syntax.Ident{{Name: o.name}}}
	path := make([]*syntax.Ident, 0, len(segs))
	for _, s := range segs {
		path = append(path, s.Name)
	}
	q.Parts = append(q.Parts, path...)
	coll := c.letCollection(env, o, path, q)
	c.domainTypes(e, o, segs)
	return coll
}

// domainTypes records the types along a domain's path: the let's, then each field's.
func (c *checker) domainTypes(e syntax.Expr, let *object, segs []*syntax.SelectorExpr) {
	t := c.letType(let)
	c.info.Types[e] = t
	if len(segs) > 0 {
		c.info.Types[segs[0].X] = t
	}
	for _, s := range segs {
		f, ok := c.info.NameUses[s.Name].(*object)
		if !ok || f.kind != ObjField {
			return
		}
		c.info.Selections[s] = &Selection{Kind: SelField, Obj: f, Recv: t}
		t = c.fieldType(f.field)
		c.info.Types[s] = t
	}
}
