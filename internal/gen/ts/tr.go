package tsgen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// tr translates one body of the portable subset, one checked helper per operation, in the evaluator's order (CONFORMANCE.md §1, §3).
type tr struct {
	g      *gen
	fn     *ir.ExportFn
	inputs []input
}

// block is the fn's statements: a lone expression is its return, a Block its statements (CONFORMANCE.md §2.2).
func (t *tr) block() string {
	var b strings.Builder
	t.stmts(t.fn.Body, &b, 1)
	return b.String()
}

// stmts translates n in statement position at depth levels of indentation: a Block's statements, a let, an if returning from each branch, or the returned value with its exit checks.
func (t *tr) stmts(n ir.PExpr, b *strings.Builder, depth int) {
	pad := strings.Repeat(indent, depth)
	switch x := n.(type) {
	case *ir.Block:
		t.stmtList(x, b, depth)
	case *ir.Let:
		fmt.Fprintf(b, letFormat, pad, escape(x.Name), t.expr(x.Value))
		t.stmts(x.Body, b, depth)
	case *ir.If:
		fmt.Fprintf(b, ifOpenFormat, pad, t.expr(x.Cond))
		t.stmts(x.Then, b, depth+1)
		b.WriteString(pad + rbrace + newline)
		t.stmts(x.Else, b, depth)
	default:
		fmt.Fprintf(b, returnFormat, pad, t.g.exitChecks(t.fn, t.expr(n)))
	}
}

// stmtList emits a Block's statements in order: a let binds for the rest of the block, an if is a block per branch that falls through when it does not return, a return ends the path.
func (t *tr) stmtList(x *ir.Block, b *strings.Builder, depth int) {
	pad := strings.Repeat(indent, depth)
	for _, st := range x.Stmts {
		switch s := st.(type) {
		case *ir.LetStmt:
			fmt.Fprintf(b, letFormat, pad, escape(s.Name), t.expr(s.Value))
		case *ir.IfStmt:
			t.ifStmt(s, b, depth)
		case *ir.ReturnStmt:
			t.stmts(s.X, b, depth)
		default:
			t.g.failf(ErrMalformed, malformedExpr, st, t.g.at)
		}
	}
}

// ifStmt is `if (c) { ... } else { ... }`; an else holding one if is `else if`.
func (t *tr) ifStmt(s *ir.IfStmt, b *strings.Builder, depth int) {
	pad := strings.Repeat(indent, depth)
	fmt.Fprintf(b, ifOpenFormat, pad, t.expr(s.Cond))
	t.branch(s.Then, b, depth)
	for s.Else != nil {
		if next := soleIf(s.Else); next != nil {
			fmt.Fprintf(b, elseIfFormat, pad, t.expr(next.Cond))
			t.branch(next.Then, b, depth)
			s = next
			continue
		}
		fmt.Fprintf(b, elseFormat, pad)
		t.stmtList(s.Else, b, depth+1)
		break
	}
	b.WriteString(pad + rbrace + newline)
}

// branch writes a branch's block one level deeper.
func (t *tr) branch(x *ir.Block, b *strings.Builder, depth int) {
	if x == nil {
		t.g.failf(ErrMalformed, malformedExpr, x, t.g.at)
		return
	}
	t.stmtList(x, b, depth+1)
}

// soleIf is the one if statement an else block holds, nil otherwise.
func soleIf(x *ir.Block) *ir.IfStmt {
	if x == nil || len(x.Stmts) != 1 {
		return nil
	}
	s, _ := x.Stmts[0].(*ir.IfStmt)
	return s
}

// expr is the TypeScript expression of n.
func (t *tr) expr(n ir.PExpr) string {
	switch x := n.(type) {
	case *ir.Lit:
		return t.lit(x)
	case *ir.ParamRef:
		return t.input(len(t.fn.Reads) + x.Index)
	case *ir.ReadRef:
		return t.input(x.Index)
	case *ir.LocalRef:
		return escape(x.Name)
	case *ir.Unary:
		return t.unary(x)
	case *ir.Binary:
		return t.binary(x)
	case *ir.Call:
		return t.call(x)
	case *ir.CallFn:
		return t.callFn(x)
	case *ir.If:
		return fmt.Sprintf(condFormat, t.operand(x.Cond), t.operand(x.Then), t.operand(x.Else))
	case *ir.Template:
		return t.template(x)
	case *ir.IsCase:
		return t.isCase(x)
	case *ir.Coalesce:
		return t.operand(x.X) + coalesceOp + t.operand(x.Y)
	}
	t.g.failf(ErrMalformed, malformedExpr, n, t.g.at)
	return tsUndefined
}

// operand is expr, parenthesized when the node is a compound expression of native operators.
func (t *tr) operand(n ir.PExpr) string {
	text := t.expr(n)
	switch x := n.(type) {
	case *ir.Binary:
		if _, native := nativeOps[x.Op]; native {
			return lparen + text + rparen
		}
	case *ir.If, *ir.Coalesce, *ir.Template, *ir.IsCase:
		return lparen + text + rparen
	case *ir.Unary:
		if x.Op == ir.OpNot {
			return lparen + text + rparen
		}
	}
	return text
}

// input is the name of the pure function's i-th parameter.
func (t *tr) input(i int) string {
	if i < 0 || i >= len(t.inputs) {
		t.g.failf(ErrMalformed, malformedExpr, i, t.g.at)
		return tsUndefined
	}
	return t.inputs[i].name
}

// lit is a literal of the pure types: a Duration its milliseconds, an enum member its wire, a ref its key.
func (t *tr) lit(x *ir.Lit) string {
	switch x.T.Kind {
	case types.Duration:
		return intText(as[value.Dur](t.g, x.V).Ms)
	case types.Ref:
		return t.g.keyLit(x.T, as[value.Ref](t.g, x.V).Key, bigAt(x.T, false))
	default:
		return t.g.lit(x.T, x.V, false)
	}
}

func (t *tr) unary(x *ir.Unary) string {
	switch x.Op {
	case ir.OpNot:
		return notOp + t.operand(x.X)
	case ir.OpNeg:
		if x.T.Kind == types.Float {
			return minus + t.operand(x.X)
		}
		return fmt.Sprintf(callFormat, t.g.helper(canonNegName), t.expr(x.X))
	default:
		t.g.failf(ErrMalformed, malformedExpr, x, t.g.at)
		return tsUndefined
	}
}

// binary is a checked helper or a native comparison; `and`, `or` short-circuit natively (CONFORMANCE.md §3).
func (t *tr) binary(x *ir.Binary) string {
	if op, native := nativeOps[x.Op]; native {
		if ordered(x) {
			return t.orderedCompare(x, op)
		}
		return t.operand(x.X) + space + op + space + t.operand(x.Y)
	}
	xk, yk := x.X.Type().Kind, x.Y.Type().Kind
	if xk == types.Float || yk == types.Float {
		return t.floatOp(x)
	}
	name := intOps[x.Op]
	if x.Op == ir.OpDiv && xk == types.Duration && yk == types.Duration {
		name = canonDivDurationName
	}
	if name == "" {
		t.g.failf(ErrMalformed, malformedExpr, x, t.g.at)
		return tsUndefined
	}
	return fmt.Sprintf(call2Format, t.g.helper(name), t.expr(x.X), t.expr(x.Y))
}

// floatOp is a Float operation: one IEEE operation whose result must be finite (E4104).
func (t *tr) floatOp(x *ir.Binary) string {
	op, ok := floatOps[x.Op]
	if !ok {
		t.g.failf(ErrMalformed, malformedExpr, x, t.g.at)
		return tsUndefined
	}
	return fmt.Sprintf(callFormat, t.g.helper(canonFName), t.operand(x.X)+space+op+space+t.operand(x.Y))
}

// ordered reports a comparison of two enum values, which TypeScript compares through their Index.
func ordered(x *ir.Binary) bool {
	switch x.Op {
	case ir.OpLt, ir.OpLe, ir.OpGt, ir.OpGe:
		return x.X.Type().Kind == types.Enum
	default:
		return false
	}
}

// orderedCompare is `<E>Index[a] < <E>Index[b]` (CONFORMANCE.md §3, comparisons).
func (t *tr) orderedCompare(x *ir.Binary, op string) string {
	index := t.g.indexConst(x.X.Type().Named)
	return fmt.Sprintf(indexOfFormat, index, t.expr(x.X)) + space + op + space + fmt.Sprintf(indexOfFormat, index, t.expr(x.Y))
}

// call is a built-in of the portable subset (CONFORMANCE.md §3).
func (t *tr) call(x *ir.Call) string {
	if len(x.Args) == 0 {
		t.g.failf(ErrMalformed, malformedExpr, x, t.g.at)
		return tsUndefined
	}
	float := x.Args[0].Type().Kind == types.Float
	args := make([]string, len(x.Args))
	for i, a := range x.Args {
		args[i] = t.expr(a)
	}
	switch {
	case x.Fn == ir.BuiltinFloat:
		return args[0]
	case (x.Fn == ir.BuiltinMin || x.Fn == ir.BuiltinMax) && float:
		return t.foldFloat(builtinFloat[x.Fn], args)
	case x.Fn == ir.BuiltinMin || x.Fn == ir.BuiltinMax:
		return mathCall(builtinInt[x.Fn], args)
	case x.Fn == ir.BuiltinAbs && float:
		return mathCall(mathAbs, args)
	}
	name := builtinInt[x.Fn]
	if float && builtinFloat[x.Fn] != "" {
		name = builtinFloat[x.Fn]
	}
	if name == "" {
		t.g.failf(ErrMalformed, malformedExpr, x, t.g.at)
		return tsUndefined
	}
	return fmt.Sprintf(callFormat, t.g.helper(name), strings.Join(args, listSep))
}

// foldFloat nests a Float min or max left to right.
func (t *tr) foldFloat(name string, args []string) string {
	acc := args[0]
	for _, a := range args[1:] {
		acc = fmt.Sprintf(call2Format, t.g.helper(name), acc, a)
	}
	return acc
}

func mathCall(name string, args []string) string {
	return mathObject + dot + name + lparen + strings.Join(args, listSep) + rparen
}

// callFn calls a package fn: a translated one's pure function, or a lookup function (CONFORMANCE.md §2.2).
func (t *tr) callFn(x *ir.CallFn) string {
	fn := x.Fn
	if fn == nil || !t.g.isPackageFn(fn) {
		t.g.failf(ErrMalformed, malformedExpr, x, t.g.at)
		return tsUndefined
	}
	args := make([]string, len(x.Args))
	for i, a := range x.Args {
		args[i] = t.expr(a)
	}
	return fmt.Sprintf(callFormat, fnName(fn), strings.Join(args, listSep))
}

// isPackageFn reports a package-level fn of this package.
func (g *gen) isPackageFn(fn *ir.ExportFn) bool { return slices.Contains(g.p.Fns, fn) }

// template concatenates text and interpolations: strings, integers in decimal, enums by Canon name (CONFORMANCE.md §3).
func (t *tr) template(x *ir.Template) string {
	var terms []string
	for _, p := range x.Parts {
		if p.Text != "" {
			terms = append(terms, quote(p.Text))
		}
		if p.X != nil {
			terms = append(terms, t.interpolation(p.X))
		}
	}
	if len(terms) == 0 {
		return emptyString
	}
	return strings.Join(terms, plusSep)
}

// interpolation is a value as a string: an integer through String, an enum through its Names.
func (t *tr) interpolation(x ir.PExpr) string {
	switch x.Type().Kind {
	case types.Int:
		return fmt.Sprintf(callFormat, stringFn, t.expr(x))
	case types.Enum:
		return fmt.Sprintf(indexOfFormat, t.g.namesConst(x.Type().Named), t.expr(x))
	default:
		return t.operand(x)
	}
}

// namesConst is the Names table of an enum, imported when another package's.
func (g *gen) namesConst(e ir.Type) string {
	name := typeName(e) + namesSuffix
	if pkg := pkgOf(e); pkg != g.p.Name {
		return g.importValue(pkg, name)
	}
	return name
}

// isCase compares the kind the pure function receives with the case (CONFORMANCE.md §2.2).
func (t *tr) isCase(x *ir.IsCase) string {
	v, ok := x.X.Type().Named.(*ir.Variant)
	if !ok || x.Case < 0 || x.Case >= len(v.Cases) {
		t.g.failf(ErrMalformed, malformedExpr, x, t.g.at)
		return tsUndefined
	}
	return t.operand(x.X) + strictEq + quote(v.Cases[x.Case].Wire)
}
