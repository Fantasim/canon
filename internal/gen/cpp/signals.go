package cppgen

import (
	"slices"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// signals reports an expression whose evaluation can signal an evaluation error (CONFORMANCE.md §3).
func signals(n ir.PExpr) bool {
	switch x := n.(type) {
	case *ir.Unary:
		return x.Op == ir.OpNeg && x.T.Kind != types.Float || signals(x.X)
	case *ir.Binary:
		return arithmetic(x.Op) || anySignals(x.X, x.Y)
	case *ir.Call:
		return callSignals(x) || anySignals(x.Args...)
	case *ir.CallFn:
		return true
	case *ir.If:
		return anySignals(x.Cond, x.Then, x.Else)
	case *ir.Template:
		return templateSignals(x)
	case *ir.IsCase:
		return signals(x.X)
	case *ir.Let:
		return anySignals(x.Value, x.Body)
	case *ir.Coalesce:
		return signals(x.Y)
	default:
		return false
	}
}

// callsPackageFn reports a translated body calling a package-level fn: it then needs prototypes.
func (g *gen) callsPackageFn() bool {
	bodies := make([]ir.PExpr, 0, len(g.pkgFns)+len(g.methods))
	for _, fn := range g.pkgFns {
		bodies = append(bodies, fn.Body)
	}
	for _, m := range g.methods {
		bodies = append(bodies, m.fn.Body)
	}
	return slices.ContainsFunc(bodies, func(n ir.PExpr) bool { return anyNode(n, g.isPackageCall) })
}

func (g *gen) isPackageCall(n ir.PExpr) bool {
	c, ok := n.(*ir.CallFn)
	return ok && (slices.Contains(g.pkgFns, c.Fn) || g.baked() && slices.Contains(g.p.Fns, c.Fn))
}

// anyNode reports a node of the tree n for which pred holds.
func anyNode(n ir.PExpr, pred func(ir.PExpr) bool) bool {
	if n == nil {
		return false
	}
	if pred(n) {
		return true
	}
	return slices.ContainsFunc(children(n), func(c ir.PExpr) bool { return anyNode(c, pred) })
}

// children are the direct operands of a node.
func children(n ir.PExpr) []ir.PExpr {
	switch x := n.(type) {
	case *ir.Unary:
		return []ir.PExpr{x.X}
	case *ir.Binary:
		return []ir.PExpr{x.X, x.Y}
	case *ir.Call:
		return x.Args
	case *ir.CallFn:
		return x.Args
	case *ir.If:
		return []ir.PExpr{x.Cond, x.Then, x.Else}
	case *ir.Let:
		return []ir.PExpr{x.Value, x.Body}
	case *ir.Coalesce:
		return []ir.PExpr{x.X, x.Y}
	case *ir.IsCase:
		return []ir.PExpr{x.X}
	case *ir.Template:
		var out []ir.PExpr
		for _, p := range x.Parts {
			if p.X != nil {
				out = append(out, p.X)
			}
		}
		return out
	case *ir.Block:
		return blockChildren(x)
	default:
		return nil
	}
}

// blockChildren are a Block's let values, if conditions and branches, and returned values.
func blockChildren(x *ir.Block) []ir.PExpr {
	var out []ir.PExpr
	for _, st := range x.Stmts {
		switch s := st.(type) {
		case *ir.LetStmt:
			out = append(out, s.Value)
		case *ir.IfStmt:
			out = append(out, s.Cond, s.Then)
			if s.Else != nil {
				out = append(out, s.Else)
			}
		case *ir.ReturnStmt:
			out = append(out, s.X)
		}
	}
	return out
}

func anySignals(xs ...ir.PExpr) bool {
	for _, x := range xs {
		if signals(x) {
			return true
		}
	}
	return false
}

func templateSignals(x *ir.Template) bool {
	for _, p := range x.Parts {
		if p.X != nil && signals(p.X) {
			return true
		}
	}
	return false
}

// callSignals reports a built-in that can fail by itself: Int(f), floor, ceil, round, clamp, an integer abs.
func callSignals(c *ir.Call) bool {
	switch c.Fn {
	case ir.BuiltinInt, ir.BuiltinFloor, ir.BuiltinCeil, ir.BuiltinRound, ir.BuiltinClamp:
		return true
	case ir.BuiltinAbs:
		return c.T.Kind != types.Float
	default:
		return false
	}
}

func arithmetic(op ir.Op) bool { return op <= ir.OpMod }

// unparen drops the parentheses that wrap a whole expression, string literals skipped.
func unparen(s string) string {
	if len(s) < minParens || s[0] != '(' || s[len(s)-1] != ')' {
		return s
	}
	depth, inString := 0, false
	for i := 0; i < len(s)-1; i++ {
		switch c := s[i]; {
		case inString && c == '\\':
			i++
		case c == '"':
			inString = !inString
		case inString:
		case c == '(':
			depth++
		case c == ')':
			depth--
		}
		if depth == 0 && !inString {
			return s
		}
	}
	return s[1 : len(s)-1]
}
