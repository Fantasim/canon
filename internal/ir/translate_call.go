package ir

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// builtinSpec is a built-in function or conversion of the portable subset: its IR function, its
// arity (the least when variadic) and the kinds every argument may have.
type builtinSpec struct {
	fn       Builtin
	arity    int
	variadic bool
	kinds    map[types.Kind]bool
}

// call is a precomputed method of self (a read), an export fn of the package, a built-in, `Float(i)` or `Int(f)` (CONFORMANCE.md §2.2); any other call is refused.
func (t *translator) call(e syntax.Expr) PExpr {
	x := e.(*syntax.CallExpr)
	c := t.s.info.Calls[x]
	if c == nil {
		t.broken = true
		return nil
	}
	if n, isRead := t.selfRead(x, ctxValue); isRead {
		return n
	}
	_, byName := x.Fun.(*syntax.IdentExpr)
	switch {
	case c.Kind == check.CalleeFn:
		return t.callFn(x, c.Obj)
	case c.Kind == check.CalleeBuiltin && byName:
		return t.builtin(x, portableBuiltins[c.Builtin])
	case c.Kind == check.CalleeConvert && byName:
		return t.builtin(x, portableConversions[c.Builtin])
	}
	operands := argValues(x)
	if sel, ok := x.Fun.(*syntax.SelectorExpr); ok {
		operands = append([]syntax.Expr{sel.X}, operands...)
	}
	t.refuseScan(x, operands...)
	return nil
}

// callFn calls a translated or lookup export fn of the package, arguments in parameter order; CONFORMANCE.md §2.2 lists no precomputed package fn, and a lookup whose result is optional or composite holds no value of the subset (meta/decisions/log-2026-09-24.md "gen/go translated fns").
func (t *translator) callFn(x *syntax.CallExpr, obj check.Object) PExpr {
	site := t.s.fnByObj[obj]
	if site == nil || site.pkg != t.site.pkg || site.recv != nil || site.fn.Kind == FnPrecomputed || !inOrder(x, site.decl) ||
		site.fn.Kind == FnLookup && !resultKinds[site.fn.Result.Kind] {
		t.refuseScan(x, argValues(x)...)
		return nil
	}
	args := t.args(x)
	if args == nil {
		return nil
	}
	return &CallFn{T: t.typ(x), Fn: site.fn, Args: args}
}

// inOrder reports arguments given for every parameter, in parameter order: the translation
// evaluates them in that order, as the evaluator evaluates them in written order.
func inOrder(x *syntax.CallExpr, d *syntax.FnDecl) bool {
	if len(x.Args) != len(d.Params) {
		return false
	}
	for i, a := range x.Args {
		if a.Name != nil && (d.Params[i].Name == nil || a.Name.Name != d.Params[i].Name.Name) {
			return false
		}
	}
	return true
}

// args translates a call's arguments; nil when one is missing.
func (t *translator) args(x *syntax.CallExpr) []PExpr {
	out := make([]PExpr, len(x.Args))
	for i, a := range x.Args {
		out[i] = t.expr(a.Value)
	}
	if !present(out...) {
		return nil
	}
	return out
}

// builtin calls the built-in or conversion spec describes, positional arguments of its kinds (CONFORMANCE.md §3); a zero spec is refused.
func (t *translator) builtin(x *syntax.CallExpr, spec builtinSpec) PExpr {
	if !t.builtinFits(x, spec) {
		t.refuseScan(x, argValues(x)...)
		return nil
	}
	args := t.args(x)
	if args == nil {
		return nil
	}
	return &Call{T: t.computed(t.typ(x)), Fn: spec.fn, Args: args}
}

func (t *translator) builtinFits(x *syntax.CallExpr, spec builtinSpec) bool {
	n := len(x.Args)
	if spec.kinds == nil || n < spec.arity || (!spec.variadic && n != spec.arity) {
		return false
	}
	var first types.Kind
	for i, a := range x.Args {
		k := t.kind(a.Value)
		if a.Name != nil || !spec.kinds[k] || (i > 0 && k != first) {
			return false
		}
		first = k
	}
	return true
}

// argValues are a call's argument expressions, in written order.
func argValues(x *syntax.CallExpr) []syntax.Expr {
	out := make([]syntax.Expr, len(x.Args))
	for i, a := range x.Args {
		out[i] = a.Value
	}
	return out
}
