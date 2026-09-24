package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// row is a signature with its index among the rows of its name (Callee.Overload).
type row struct {
	sig      bsig
	overload int
}

// rowsNamed are the rows of a table for one name, in table order.
func rowsNamed(table []bsig, name string) []row {
	var out []row
	for _, s := range table {
		if s.name == name {
			out = append(out, row{sig: s, overload: len(out)})
		}
	}
	return out
}

// builtinCall is one call of a built-in being checked.
type builtinCall struct {
	x    *syntax.CallExpr
	fun  syntax.Expr
	name string
	kind CalleeKind
	b    *binding
	want types.Type
}

// callBuiltin calls a built-in named in scope (STDLIB.md §2, §10).
func (c *checker) callBuiltin(env *env, x *syntax.CallExpr, id *syntax.IdentExpr, want types.Type) types.Type {
	rows := rowsNamed(freeFunctions, id.Name)
	if len(rows) == 0 {
		c.report(env, diag.E3005.At(env.span(id), nameType(c.universe[id.Name])))
		c.argsAlone(env, x)
		return types.ErrorType
	}
	bc := &builtinCall{x: x, fun: id, name: id.Name, kind: CalleeBuiltin, b: newBinding(), want: want}
	if _, isType := builtinTypes[id.Name]; isType {
		bc.kind = CalleeConvert
		return c.conversion(env, bc, rows)
	}
	return c.applyRows(env, bc, rows)
}

// conversion is `Int(x)`, `Float(x)`, `String(x)` (STDLIB.md §2.1).
func (c *checker) conversion(env *env, bc *builtinCall, rows []row) types.Type {
	if len(bc.x.Args) != 1 || bc.x.Args[0].Name != nil {
		return c.applyRows(env, bc, rows)
	}
	arg := bc.x.Args[0].Value
	at := c.synth(env, arg)
	for _, r := range rows {
		p := r.sig.params[0].t
		if tv, ok := p.(*tvar); ok {
			if !satisfies(at, tv.cons) {
				c.report(env, diag.E4503.AtPlain(env.span(arg), at))
			}
			return c.finishBuiltin(bc, r)
		}
		if types.Identical(at, p) || (isIntLiteral(arg) && p.Kind() == types.Int) {
			return c.finishBuiltin(bc, r)
		}
	}
	if at.Kind() != types.Error {
		c.report(env, diag.E3002.At(env.span(arg), bc.b.shown(rows[0].sig.params[0].t), at))
	}
	return types.ErrorType
}

// finishBuiltin records the callee of a built-in call and returns its result.
func (c *checker) finishBuiltin(bc *builtinCall, r row) types.Type {
	c.info.Calls[bc.x] = &Callee{Kind: bc.kind, Builtin: bc.name, Overload: r.overload, TypeArgs: bc.b.typeArgs()}
	ft := &types.FuncType{Result: concrete(bc.b.subst(r.sig.result))}
	for _, p := range r.sig.params {
		ft.Params = append(ft.Params, concrete(bc.b.subst(p.t)))
	}
	if bc.fun != nil {
		c.info.Types[bc.fun] = ft
	}
	return ft.Result
}

// applyRows picks the rows whose parameters the arguments fit (E3004 when none does), then
// applies it; the graph functions choose their `next` form from the function's result type.
func (c *checker) applyRows(env *env, bc *builtinCall, rows []row) types.Type {
	var fit []row
	var maps [][]int
	for _, r := range rows {
		if m, ok := argMap(bc.x, r.sig); ok {
			fit, maps = append(fit, r), append(maps, m)
		}
	}
	switch len(fit) {
	case 0:
		c.arityError(env, bc, rows[len(rows)-1].sig)
		c.argsAlone(env, bc.x)
		return types.ErrorType
	case 1:
		return c.applyRow(env, bc, fit[0], maps[0])
	}
	return c.applyNextForm(env, bc, fit, maps[0])
}

// argMap maps each argument to its parameter, by position then by name; false when an
// argument has no parameter, one is given twice or one is missing.
func argMap(x *syntax.CallExpr, s bsig) ([]int, bool) {
	m := make([]int, len(x.Args))
	given := make([]bool, len(s.params))
	for i, a := range x.Args {
		j := i
		if a.Name != nil {
			j = bparamIndex(s.params, a.Name.Name)
		} else if s.variadic && j >= len(s.params) {
			j = len(s.params) - 1
		}
		if j < 0 || j >= len(s.params) || (given[j] && !s.variadic) {
			return nil, false
		}
		given[j] = true
		m[i] = j
	}
	for _, g := range given {
		if !g {
			return nil, false
		}
	}
	return m, true
}

func bparamIndex(ps []bparam, name string) int {
	for i, p := range ps {
		if p.name == name {
			return i
		}
	}
	return -1
}

// arityError is E3004 for a built-in call no row fits.
func (c *checker) arityError(env *env, bc *builtinCall, s bsig) {
	for _, a := range bc.x.Args {
		if a.Name != nil && bparamIndex(s.params, a.Name.Name) < 0 {
			c.report(env, diag.E3004.AtUnknown(env.span(a.Name), a.Name.Name, bc.name))
			return
		}
	}
	if len(bc.x.Args) > len(s.params) && !s.variadic {
		c.report(env, diag.E3004.AtMany(env.span(bc.x), bc.name))
		return
	}
	missing := ""
	if n := len(bc.x.Args); n < len(s.params) {
		missing = s.params[n].name
	}
	c.report(env, diag.E3004.AtMissing(env.span(bc.x), missing, bc.name))
}

// applyRow checks a call against one row (TYPES.md §12.2).
func (c *checker) applyRow(env *env, bc *builtinCall, r row, m []int) types.Type {
	for _, pass := range []func(syntax.Expr) bool{isPlainArg, isIntLiteral} {
		for i, a := range bc.x.Args {
			if pass(a.Value) {
				c.bindArg(env, bc, a.Value, r.sig.params[m[i]].t)
			}
		}
	}
	for i, a := range bc.x.Args {
		if isLambda(a.Value) {
			c.lambdaArg(env, bc, a.Value, r.sig.params[m[i]].t)
		}
	}
	return c.builtinResult(env, bc, r)
}

// isPlainArg is an argument bound first: neither a lambda nor an integer literal.
func isPlainArg(e syntax.Expr) bool { return !isLambda(e) && !isIntLiteral(e) }

// bindArg checks a non-lambda argument: against its parameter when that is bound, else
// synthesized and unified (E3002 when it does not fit).
func (c *checker) bindArg(env *env, bc *builtinCall, e syntax.Expr, pat types.Type) {
	if pat.Kind() == types.Any && !isTvar(pat) {
		c.synth(env, e)
		return
	}
	if !bc.b.free(pat) && !hasSeq(pat) {
		c.expr(env, e, bc.b.subst(pat))
		return
	}
	t := c.expr(env, e, c.literalHint(e, bc.b, pat))
	before := len(bc.b.order)
	if !bc.b.unify(pat, t) {
		c.argMismatch(env, bc, e, pat, t)
		return
	}
	bc.b.from(before, e)
	if want := bc.b.subst(pat); !bc.b.free(want) && !hasSeq(want) {
		c.accept(env, e, t, want)
	}
}

// argMismatch is E3403 for an optional whose present form fits (TYPES.md §6.5), else E3002; then nothing cascades.
func (c *checker) argMismatch(env *env, bc *builtinCall, e syntax.Expr, pat, t types.Type) {
	want := bc.b.shown(pat)
	if t.Base().Kind() == types.Optional && pat.Kind() != types.Optional {
		c.report(env, diag.E3403.At(env.span(e), want))
	} else {
		c.report(env, diag.E3002.At(env.span(e), want, t))
	}
	bc.b.poison(pat)
}

// literalHint is the type an integer literal takes when its parameter is bound to Float.
func (c *checker) literalHint(e syntax.Expr, b *binding, pat types.Type) types.Type {
	if !isIntLiteral(e) {
		return nil
	}
	if t := b.subst(pat); !b.free(t) && t.Base().Kind() == types.Float {
		return t
	}
	return nil
}

func isTvar(t types.Type) bool {
	_, ok := t.(*tvar)
	return ok
}

// hasSeq reports a Seq(T) pattern, which no value has as its type.
func hasSeq(t types.Type) bool {
	_, ok := t.(*seqOf)
	return ok
}

// lambdaArg checks a lambda (or shorthand) against its parameter's function type, its
// parameters bound; the body's type binds what its result leaves free.
func (c *checker) lambdaArg(env *env, bc *builtinCall, e syntax.Expr, pat types.Type) {
	fp, ok := bc.b.subst(pat).(*types.FuncType)
	if !ok {
		c.report(env, diag.E3002.At(env.span(e), bc.b.shown(pat), anyFunc(lambdaArity(e))))
		c.info.Types[e] = types.ErrorType
		return
	}
	if slices.ContainsFunc(fp.Params, bc.b.free) {
		c.report(env, diag.E3008.At(env.span(e)))
		c.info.Types[e] = types.ErrorType
		bc.b.poison(pat)
		return
	}
	lt := c.lambdaWith(env, e, fp, bc.b)
	if lt == nil {
		bc.b.poison(pat)
		return
	}
	before := len(bc.b.order)
	if !bc.b.unify(pat, lt) {
		c.report(env, diag.E3002.At(env.span(e), bc.b.shown(pat), lt))
		bc.b.poison(pat)
		return
	}
	bc.b.from(before, e)
}

// lambdaArity is the number of parameters of a lambda, one for a shorthand.
func lambdaArity(e syntax.Expr) int {
	if l, ok := e.(*syntax.LambdaExpr); ok {
		return len(l.Params)
	}
	return 1
}

// builtinResult binds what is left from the expected type, checks the constraints and records
// the callee.
func (c *checker) builtinResult(env *env, bc *builtinCall, r row) types.Type {
	if bc.b.free(r.sig.result) && bc.want != nil {
		bc.b.unify(r.sig.result, bc.want)
	}
	if bc.b.free(r.sig.result) {
		c.report(env, diag.E3008.At(env.span(bc.x)))
		return types.ErrorType
	}
	for _, v := range bc.b.order {
		if !satisfies(bc.b.vars[v], v.cons) {
			c.constraintError(env, bc, v)
			return types.ErrorType
		}
	}
	return c.finishBuiltin(bc, r)
}

// constraintError is E3310 for an Ord type parameter, E3002 for any other constraint.
func (c *checker) constraintError(env *env, bc *builtinCall, v *tvar) {
	t := bc.b.vars[v]
	at := syntax.Node(bc.x)
	if src := bc.b.src[v]; src != nil {
		at = src
	}
	if o, isOpt := t.Base().(*types.OptionalType); isOpt && satisfies(o.Elem, v.cons) {
		c.report(env, diag.E3403.At(env.span(at), o.Elem))
		return
	}
	if v.cons == consOrd {
		c.report(env, diag.E3310.At(env.span(at), bc.name, t))
		return
	}
	c.report(env, diag.E3002.At(env.span(at), consExample(v.cons), t))
}

// applyNextForm applies a graph function (STDLIB.md §2.3).
func (c *checker) applyNextForm(env *env, bc *builtinCall, rows []row, m []int) types.Type {
	first := rows[0].sig
	for i, a := range bc.x.Args {
		if !isLambda(a.Value) && first.params[m[i]].name != paramNext {
			c.bindArg(env, bc, a.Value, first.params[m[i]].t)
		}
	}
	for i, a := range bc.x.Args {
		if first.params[m[i]].name != paramNext {
			continue
		}
		r := c.nextForm(env, bc, a.Value, rows)
		return c.builtinResult(env, bc, r)
	}
	return c.builtinResult(env, bc, rows[0])
}

// nextForm types the `next` argument with its result free, then picks the row it fits.
func (c *checker) nextForm(env *env, bc *builtinCall, e syntax.Expr, rows []row) row {
	elem := bc.b.subst(tEq)
	free := &tvar{name: nameR}
	var t types.Type
	if isLambda(e) {
		t = c.lambdaWith(env, e, &types.FuncType{Params: []types.Type{elem}, Result: free}, bc.b)
	} else {
		t = c.synth(env, e)
	}
	if t == nil {
		return rows[0]
	}
	for _, r := range rows {
		pat := r.sig.params[bparamIndex(r.sig.params, paramNext)].t
		if bc.b.unify(pat, t) {
			return r
		}
	}
	if ft, ok := t.(*types.FuncType); ok && types.Assignable(ft.Result, elem) {
		return rows[len(rows)-1]
	}
	c.report(env, diag.E3002.At(env.span(e), bc.b.subst(rows[0].sig.params[1].t), t))
	return rows[0]
}
