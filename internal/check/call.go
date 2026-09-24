package check

import (
	"strconv"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// call is `f(args)` (TYPES.md §12.2): a function or built-in by name, a method after `.`, or a function value.
func (c *checker) call(env *env, x *syntax.CallExpr, want types.Type) (types.Type, bool) {
	switch f := x.Fun.(type) {
	case *syntax.IdentExpr:
		return c.callName(env, x, f, want), false
	case *syntax.SelectorExpr:
		return c.callMethod(env, x, f, want)
	}
	t, opt := c.recv(env, x.Fun)
	return c.callValue(env, x, t, ""), opt
}

// callName calls a name found by steps 2 to 6: a function, a method of the record body (on
// self), a function value, a built-in or a conversion; anything else is E3005.
func (c *checker) callName(env *env, x *syntax.CallExpr, id *syntax.IdentExpr, want types.Type) types.Type {
	o := c.lookup(env, id.Name)
	if o == nil {
		c.unknownName(env, id, id.Name)
		c.argsAlone(env, x)
		return types.ErrorType
	}
	c.info.Uses[id] = o
	c.dependsOn(env, o)
	switch o.kind {
	case ObjFn, ObjMethod:
		c.info.Types[id] = o.typ
		return c.callUser(env, x, o)
	case ObjLocal, ObjParam:
		t := c.narrowed(env, id, o.typ)
		c.info.Types[id] = t
		return c.callValue(env, x, t, o.name)
	case ObjBuiltin:
		return c.callBuiltin(env, x, id, want)
	default:
	}
	c.report(env, diag.E3005.At(env.span(id), nameType(o)))
	c.argsAlone(env, x)
	return types.ErrorType
}

// nameType is what a name that cannot be called is, for E3005.
func nameType(o *object) types.Type {
	if o.typ != nil {
		return o.typ
	}
	return types.ErrorType
}

// argsAlone types the arguments of a call that failed, so they are still checked.
func (c *checker) argsAlone(env *env, x *syntax.CallExpr) {
	for _, a := range x.Args {
		if !isLambda(a.Value) {
			c.synth(env, a.Value)
		}
	}
}

func isLambda(e syntax.Expr) bool {
	switch e.(type) {
	case *syntax.LambdaExpr, *syntax.ShorthandLambda:
		return true
	default:
		return false
	}
}

// callUser calls a top-level function or a method (TYPES.md §12.2).
func (c *checker) callUser(env *env, x *syntax.CallExpr, o *object) types.Type {
	ft, ok := o.typ.(*types.FuncType)
	if !ok {
		c.argsAlone(env, x)
		return types.ErrorType
	}
	switch {
	case env.what != noConstant:
		c.report(env, diag.E3015.At(env.span(x), env.what))
	case env.fields >= 0:
		c.report(env, diag.E3010.At(env.span(x)))
	}
	kind := CalleeFn
	if o.kind == ObjMethod {
		kind = CalleeMethod
	}
	c.info.Calls[x] = &Callee{Kind: kind, Obj: o}
	c.userArgs(env, x, o.name, c.paramObjects(o), ft.Params)
	return ft.Result
}

// paramObjects are the parameter objects of a function or method.
func (c *checker) paramObjects(o *object) []*object {
	return c.fnParams[o]
}

// userArgs matches and checks the arguments of a call against named parameters.
func (c *checker) userArgs(env *env, x *syntax.CallExpr, fn string, params []*object, ts []types.Type) {
	given := make([]bool, len(ts))
	for i, a := range x.Args {
		j := i
		if a.Name != nil {
			j = paramIndex(params, a.Name.Name)
			if j < 0 {
				c.report(env, diag.E3004.AtUnknown(env.span(a.Name), a.Name.Name, fn))
				c.synth(env, a.Value)
				continue
			}
			c.info.NameUses[a.Name] = params[j]
		}
		if j >= len(ts) {
			c.report(env, diag.E3004.AtMany(env.span(a), fn))
			c.synth(env, a.Value)
			continue
		}
		if given[j] {
			c.report(env, diag.E3004.AtMany(env.span(a), fn))
		}
		given[j] = true
		c.expr(env, a.Value, ts[j])
	}
	for j, ok := range given {
		if !ok && !c.hasDefault(params, j) {
			c.report(env, diag.E3004.AtMissing(env.span(x), paramName(params, j), fn))
		}
	}
}

func paramIndex(params []*object, name string) int {
	for i, p := range params {
		if p.name == name {
			return i
		}
	}
	return -1
}

func paramName(params []*object, j int) string {
	if j < len(params) {
		return params[j].name
	}
	return ""
}

// hasDefault reports a parameter with a default (TYPES.md §12.1).
func (c *checker) hasDefault(params []*object, j int) bool {
	if j >= len(params) {
		return false
	}
	p, ok := params[j].decl.(*syntax.Param)
	return ok && p.Default != nil
}

// callValue calls a function value (a parameter or local of function type, named name):
// positional arguments only; anything else is E3005.
func (c *checker) callValue(env *env, x *syntax.CallExpr, t types.Type, name string) types.Type {
	ft, ok := t.Base().(*types.FuncType)
	if k := t.Base().Kind(); k == types.Optional || k == types.None {
		c.report(env, diag.E3402.At(env.span(x.Fun), env.span(x.Fun)))
		c.argsAlone(env, x)
		return types.ErrorType
	}
	if !ok {
		if t.Kind() != types.Error {
			c.report(env, diag.E3005.At(env.span(x.Fun), t))
		}
		c.argsAlone(env, x)
		return types.ErrorType
	}
	c.info.Calls[x] = &Callee{Kind: CalleeLambda}
	for i, a := range x.Args {
		switch {
		case a.Name != nil:
			c.report(env, diag.E3004.AtUnknown(env.span(a.Name), a.Name.Name, name))
			c.synth(env, a.Value)
		case i >= len(ft.Params):
			c.report(env, diag.E3004.AtMany(env.span(a), name))
			c.synth(env, a.Value)
		default:
			c.expr(env, a.Value, ft.Params[i])
		}
	}
	if n := len(x.Args); n < len(ft.Params) {
		c.report(env, diag.E3004.AtMissing(env.span(x), argPosition(n), name))
	}
	return ft.Result
}

// argPosition names the n-th parameter of a function type, which has no names.
func argPosition(n int) string { return hash + strconv.Itoa(n) }

// callMethod is `x.m(args)`: a function of a package (`pkg.f(…)`), a user method of the
// receiver's record or case, or a built-in method (STDLIB.md); `x?.m(…)` unwraps x.
func (c *checker) callMethod(env *env, x *syntax.CallExpr, s *syntax.SelectorExpr, want types.Type) (types.Type, bool) {
	if q := c.qualifier(env, s.X); q != nil {
		return c.callQualified(env, x, s, q), false
	}
	if isEmptyList(s.X) && s.Name.Name == methodSum {
		return c.sumOfEmpty(env, x, s, want), false
	}
	var t types.Type
	opt := false
	if s.X == nil {
		t = env.shorthand
	} else {
		t, opt = c.recv(env, s.X)
	}
	if t.Kind() == types.Error {
		c.argsAlone(env, x)
		return t, opt
	}
	t, ok := c.optionalRecv(env, s.X, t, s.Optional, optDotText)
	if !ok {
		c.argsAlone(env, x)
		return types.ErrorType, opt || s.Optional
	}
	return c.methodOn(env, x, s, t, want), opt || s.Optional
}

func isEmptyList(e syntax.Expr) bool {
	l, ok := inner(e).(*syntax.ListLit)
	return ok && len(l.Elems) == 0
}

// sumOfEmpty is `[].sum()`: of the expected numeric type, else E3314 (STDLIB.md §4.3).
func (c *checker) sumOfEmpty(env *env, x *syntax.CallExpr, s *syntax.SelectorExpr, want types.Type) types.Type {
	elem := unwrap(want)
	if elem == nil {
		c.report(env, diag.E3314.At(env.span(x)))
		c.info.Types[s.X] = types.ErrorType
		c.argsAlone(env, x)
		return types.ErrorType
	}
	l := &types.ListType{Elem: elem}
	for e := s.X; ; e = e.(*syntax.ParenExpr).X {
		c.info.Types[e] = l
		if _, isParen := e.(*syntax.ParenExpr); !isParen {
			break
		}
	}
	return c.methodOn(env, x, s, l, want)
}

// methodOn resolves m on a receiver of type t: a user method (through a ref), else a built-in.
func (c *checker) methodOn(env *env, x *syntax.CallExpr, s *syntax.SelectorExpr, t, want types.Type) types.Type {
	recv, deref := t, false
	if r, ok := t.Base().(*types.RefType); ok {
		recv, deref = c.coll(r).Elem, true
	}
	if m := c.userMethod(recv, s.Name.Name); m != nil {
		c.info.Selections[s] = &Selection{Kind: SelMethod, Obj: m, Recv: t, Deref: deref}
		c.info.NameUses[s.Name] = m
		c.info.Types[s] = m.typ
		return c.callUser(env, x, m)
	}
	sel := &Selection{Kind: SelMethod, Recv: t, Deref: deref}
	c.info.Selections[s] = sel
	if bo := c.builtins[s.Name.Name]; bo != nil {
		sel.Obj = bo
		c.info.NameUses[s.Name] = bo
	}
	return c.builtinMethod(env, x, s, recv, want)
}

// userMethod is the method name of a record or case, or nil.
func (c *checker) userMethod(t types.Type, name string) *object {
	var body *recordCtx
	switch x := t.Base().(type) {
	case *types.RecordType:
		c.completeRecord(x)
		body = c.bodyOf(x)
	case *types.AppliedRecord:
		c.completeRecord(x.Rec)
		body = c.bodyOf(x.Rec)
	case *types.CaseType:
		c.completeVariant(x.Variant)
		body = c.caseBodies[x]
	}
	if body == nil {
		return nil
	}
	return body.methods[name]
}

// bodyOf is the body context of a record.
func (c *checker) bodyOf(r *types.RecordType) *recordCtx {
	if o := c.typeObjects[r]; o != nil {
		return o.body
	}
	return nil
}

// callQualified is `pkg.f(args)`, or a conversion or type written qualified (E3005).
func (c *checker) callQualified(env *env, x *syntax.CallExpr, s *syntax.SelectorExpr, q *object) types.Type {
	if q.kind != ObjPackage {
		c.report(env, diag.E3005.At(env.span(s), nameType(q)))
		c.argsAlone(env, x)
		return types.ErrorType
	}
	m, ok := q.target.names[s.Name.Name]
	if !ok || m.local {
		c.report(env, diag.E2004.At(env.span(s.Name), q.target.path, s.Name.Name))
		c.argsAlone(env, x)
		return types.ErrorType
	}
	c.info.NameUses[s.Name] = m
	c.dependsOn(env, m)
	if m.kind != ObjFn {
		c.report(env, diag.E3005.At(env.span(s), nameType(m)))
		c.argsAlone(env, x)
		return types.ErrorType
	}
	c.info.Types[s] = m.typ
	return c.callUser(env, x, m)
}
