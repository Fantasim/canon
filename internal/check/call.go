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
	if params, ok := c.valueParams(o); ok {
		c.typeArgsAlone(env, x, params)
		return types.ErrorType
	}
	c.argsAlone(env, x)
	return types.ErrorType
}

// valueParams are the value parameters of a type function or a parameterized record o names.
func (c *checker) valueParams(o *object) ([]*types.Param, bool) {
	if fn := c.typeFuncOf(o); fn != nil {
		return fn.Params, true
	}
	if o.kind != ObjTypeName {
		return nil, false
	}
	rec, ok := c.resolveTypeName(o).(*types.RecordType)
	if !ok || rec.Decl == nil || len(rec.Decl.Params) == 0 {
		return nil, false
	}
	c.completeRecord(rec)
	return rec.Params, true
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
		c.argAlone(env, a.Value)
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

// callUser calls a function or method, never from a constant or a default (TYPES.md §12.2, §15).
func (c *checker) callUser(env *env, x *syntax.CallExpr, o *object) types.Type {
	switch {
	case env.what != noConstant:
		c.report(env, diag.E3015.AtNotConstant(env.span(x), env.what))
	case env.fields >= 0:
		c.report(env, diag.E3010.At(env.span(x)))
	}
	ft, ok := o.typ.(*types.FuncType)
	if !ok {
		c.argsAlone(env, x)
		return types.ErrorType
	}
	kind := CalleeFn
	if o.kind == ObjMethod {
		kind = CalleeMethod
	}
	c.info.Calls[x] = &Callee{Kind: kind, Obj: o}
	c.textUse(env, x.Fun, o, true)
	c.userArgs(env, x, o.name, c.paramObjects(o), ft.Params)
	return staticView(ft.Result)
}

// paramObjects are the parameter objects of a function or method.
func (c *checker) paramObjects(o *object) []*object {
	return c.fnParams[o]
}

// userArgs checks a call's arguments against its parameters; what E1121 refused adds no E3004 (TYPES.md §12.2).
func (c *checker) userArgs(env *env, x *syntax.CallExpr, fn string, params []*object, ts []types.Type) {
	given := make([]bool, len(ts))
	refused, misplaced := refusedArgs(x.Args)
	for i, a := range x.Args {
		if refused[i] && a.Name == nil {
			c.argAlone(env, a.Value)
			continue
		}
		j := c.argIndex(env, a, i, fn, params)
		switch {
		case j < 0:
			c.synth(env, a.Value)
		case j >= len(ts):
			c.report(env, diag.E3004.AtMany(env.span(a), fn))
			c.synth(env, a.Value)
		default:
			if given[j] && !refused[i] {
				c.report(env, diag.E3004.AtMany(env.span(a), fn))
			}
			given[j] = true
			c.expr(env, a.Value, ts[j])
		}
	}
	for j, ok := range given {
		if !ok && !misplaced && !c.hasDefault(params, j) {
			c.report(env, diag.E3004.AtMissing(env.span(x), paramName(params, j), fn))
		}
	}
}

// argIndex is the parameter an argument names, else its position; -1 after E3004 for an
// unknown name.
func (c *checker) argIndex(env *env, a *syntax.Arg, i int, fn string, params []*object) int {
	if a.Name == nil {
		return i
	}
	j := paramIndex(params, a.Name.Name)
	if j < 0 {
		c.report(env, diag.E3004.AtUnknown(env.span(a.Name), a.Name.Name, fn))
		return j
	}
	c.info.NameUses[a.Name] = params[j]
	return j
}

// misplacedArgs checks alone the arguments of a built-in call holding a positional argument
// after a named one (E1121): which parameter each fills is unknown, so no row is judged.
func (c *checker) misplacedArgs(env *env, x *syntax.CallExpr) bool {
	_, misplaced := refusedArgs(x.Args)
	if misplaced {
		c.argsAlone(env, x)
	}
	return misplaced
}

// argAlone checks an argument no parameter types: synthesized, a lambda alone (lambdaAlone).
func (c *checker) argAlone(env *env, e syntax.Expr) {
	if isLambda(e) {
		c.info.Types[e] = c.lambdaAlone(env, e)
		return
	}
	c.synth(env, e)
}

// refusedArgs marks the arguments E1121 refuses; misplaced: one is positional after a named one (GRAMMAR.md §6.7).
func refusedArgs(args []*syntax.Arg) (refused []bool, misplaced bool) {
	refused = make([]bool, len(args))
	names := map[string]bool{}
	named := false
	for i, a := range args {
		if a.Name == nil {
			refused[i], misplaced = named, misplaced || named
			continue
		}
		named = true
		refused[i] = names[a.Name.Name]
		names[a.Name.Name] = true
	}
	return refused, misplaced
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
	return staticView(ft.Result)
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
		c.dependsOn(env, m) // DECISIONS 209: a broken method breaks its callers
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
		if cb := c.caseBodies[x]; cb != nil && cb.methods[name] != nil {
			return cb.methods[name]
		}
		body = c.variantBody[x.Variant]
	case *types.VariantType:
		c.completeVariant(x)
		body = c.variantBody[x]
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
