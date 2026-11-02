package progen_test

import "github.com/fantasim/canonlang/internal/syntax"

// testedOperands are the path operands of tg's tests against none and `is`: what a condition narrows (TYPES.md §6.6).
func testedOperands(tg target) []syntax.Expr {
	var out []syntax.Expr
	add := func(e syntax.Expr) {
		if e = syntax.Unparen(e); rootIdent(e) != nil {
			out = append(out, e)
		}
	}
	for _, b := range nodes[*syntax.BinaryExpr](tg) {
		_, xNone := syntax.Unparen(b.X).(*syntax.NoneLit)
		_, yNone := syntax.Unparen(b.Y).(*syntax.NoneLit)
		if (b.Op == syntax.TokEq || b.Op == syntax.TokNe) && (xNone || yNone) {
			add(b.X)
			add(b.Y)
		}
	}
	for _, is := range nodes[*syntax.IsExpr](tg) {
		add(is.X)
	}
	return out
}

// rootIdent is the root of a path `a.b.c` (or `a?.b`), nil for anything else.
func rootIdent(e syntax.Expr) *syntax.IdentExpr {
	for {
		switch x := e.(type) {
		case *syntax.IdentExpr:
			return x
		case *syntax.SelectorExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// testedPaths are the selectors of tg's tested paths, whose renaming drops a fact (TYPES.md §6.6).
func testedPaths(tg target) map[*syntax.SelectorExpr]bool {
	out := map[*syntax.SelectorExpr]bool{}
	for _, e := range testedOperands(tg) {
		syntax.Inspect(e, func(n syntax.Node) bool {
			if s, ok := n.(*syntax.SelectorExpr); ok {
				out[s] = true
			}
			return true
		})
	}
	return out
}

// testedRoots are the roots of tg's tested paths with a field: `x!.f` is no stable path (TYPES.md §6.6).
func testedRoots(tg target) map[string]bool {
	out := map[string]bool{}
	for _, e := range testedOperands(tg) {
		if _, isPath := e.(*syntax.SelectorExpr); isPath {
			out[rootIdent(e).Name] = true
		}
	}
	return out
}

// dependentReceivers tells a receiver that may be a dependent value, whose members are TYPES.md §4.3, §11.4's.
func dependentReceivers(tg target) func(syntax.Expr) bool {
	fns, fields := typeFunctions(tg), dependentFields(tg)
	direct := func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.CallExpr:
			id, ok := x.Fun.(*syntax.IdentExpr)
			return ok && fns[id.Name]
		case *syntax.SelectorExpr:
			return fields[x.Name.Name]
		}
		return false
	}
	bound := dependentLocals(tg, func(e syntax.Expr) bool { return e != nil && holds(e, direct) })
	return func(x syntax.Expr) bool {
		x = syntax.Unparen(x)
		if id, ok := x.(*syntax.IdentExpr); ok {
			return bound[id.Name]
		}
		return direct(x)
	}
}

// dependentLocals are the names tg binds over a source dep accepts: a loop or comprehension
// variable, or a parameter of a lambda passed to a method of it.
func dependentLocals(tg target, dep func(syntax.Expr) bool) map[string]bool {
	out := map[string]bool{}
	bind := func(src syntax.Expr, names []*syntax.Ident) {
		if dep(src) {
			for _, n := range names {
				out[n.Name] = true
			}
		}
	}
	for _, f := range nodes[*syntax.ForStmt](tg) {
		bind(f.Iter, f.Vars)
	}
	for _, c := range nodes[*syntax.CompClause](tg) {
		bind(c.X, c.Vars)
	}
	for _, call := range nodes[*syntax.CallExpr](tg) {
		recv, ok := call.Fun.(*syntax.SelectorExpr)
		if !ok {
			continue
		}
		for _, a := range call.Args {
			if l, isLambda := a.Value.(*syntax.LambdaExpr); isLambda {
				bind(recv.X, l.Params)
			}
		}
	}
	return out
}
