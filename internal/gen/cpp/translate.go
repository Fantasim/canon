package cppgen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// tr translates one body of the portable subset, one helper per operation (CONFORMANCE.md §2).
type tr struct {
	g     *gen
	fn    *ir.ExportFn
	temps int
	used  map[string]bool // parameter and local names a temporary must not take
}

// block is the statement list an expression is being translated into.
type block struct {
	lines []string
}

func (b *block) add(format string, args ...any) {
	b.lines = append(b.lines, fmt.Sprintf(format, args...))
}

// temp is a fresh temporary, t1, t2, …, skipping the body's own names.
func (t *tr) temp() string {
	for {
		t.temps++
		name := fmt.Sprintf(tempFormat, t.temps)
		if !t.used[name] {
			return name
		}
	}
}

// local declares a const local: a String owns its bytes and is built explicitly from a view.
func (t *tr) local(b *block, ty ir.TypeRef, name, v string) {
	if ty.Kind == types.String {
		b.add(stringLocalFormat, name, v)
		return
	}
	b.add(constLocalFormat, t.localType(ty), name, v)
}

// stmts translates a body in statement position: `let` binds a const, `if` returns early.
func (t *tr) stmts(n ir.PExpr, b *block) {
	switch x := n.(type) {
	case *ir.Block:
		t.block(x, b)
	case *ir.Let:
		t.local(b, x.Value.Type(), verbatim(x.Name), unparen(t.expr(x.Value, b)))
		t.stmts(x.Body, b)
	case *ir.If:
		if x.Else == nil {
			t.g.fail(fmt.Errorf("%w: if without else in %s", ErrMalformed, t.g.at))
			return
		}
		c := t.expr(x.Cond, b)
		var then block
		t.stmts(x.Then, &then)
		b.add(ifOpenFormat, unparen(c))
		for _, l := range then.lines {
			b.add(indentedFormat, l)
		}
		b.add(closeBrace)
		t.stmts(x.Else, b)
	default:
		v := unparen(t.expr(n, b))
		if t.fn.Result.Kind == types.String && !ownsString(n) {
			v = fmt.Sprintf(stringOfFormat, v)
		}
		b.add(returnFormat, t.g.exitChecks(t.fn.Result, t.fn.ResultRange, v))
	}
}

// block emits a Block's statements in order: a `let` is a const local, an `if` a C++ block per
// branch (which scopes its lets) falling through when it does not return, a `return` the result.
func (t *tr) block(x *ir.Block, b *block) {
	for _, st := range x.Stmts {
		switch s := st.(type) {
		case *ir.LetStmt:
			t.local(b, s.Value.Type(), verbatim(s.Name), unparen(t.expr(s.Value, b)))
		case *ir.IfStmt:
			t.ifStmt(s, b)
		case *ir.ReturnStmt:
			t.stmts(s.X, b)
		default:
			t.g.fail(fmt.Errorf("%w: statement %T in %s", ErrMalformed, st, t.g.at))
		}
	}
}

// ifStmt is `if (c) { … } else { … }`, each branch in braces.
func (t *tr) ifStmt(s *ir.IfStmt, b *block) {
	if s.Then == nil {
		t.g.fail(fmt.Errorf("%w: if without then in %s", ErrMalformed, t.g.at))
		return
	}
	b.add(ifOpenFormat, unparen(t.expr(s.Cond, b)))
	t.nested(s.Then, b)
	if s.Else != nil {
		b.add(elseOpen)
		t.nested(s.Else, b)
	}
	b.add(closeBrace)
}

// nested emits a branch's Block, indented inside its braces.
func (t *tr) nested(x *ir.Block, b *block) {
	var inner block
	t.block(x, &inner)
	for _, l := range inner.lines {
		b.add(indentedFormat, l)
	}
}

// ownsString reports an expression whose C++ form is already a std::string.
func ownsString(n ir.PExpr) bool {
	switch n.(type) {
	case *ir.Template, *ir.If, *ir.Coalesce, *ir.LocalRef, *ir.CallFn:
		return true
	default:
		return false
	}
}

// operands binds operands that can signal to temporaries when two can (CONFORMANCE.md §2.3).
func (t *tr) operands(xs []ir.PExpr, b *block) []string {
	n := 0
	for _, x := range xs {
		if signals(x) {
			n++
		}
	}
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = t.expr(x, b)
		if n > 1 && signals(x) {
			tmp := t.temp()
			t.local(b, x.Type(), tmp, unparen(out[i]))
			out[i] = tmp
		}
	}
	return out
}

// sub translates a branch evaluated only when taken; its statements go in a lambda.
func (t *tr) sub(x ir.PExpr) string {
	var inner block
	code := t.expr(x, &inner)
	if len(inner.lines) == 0 {
		return code
	}
	return fmt.Sprintf(lambdaFormat, t.localType(x.Type()), strings.Join(inner.lines, space), unparen(code))
}

// expr is the C++ expression of n; statements it needs first go to b.
func (t *tr) expr(n ir.PExpr, b *block) string {
	switch x := n.(type) {
	case *ir.Lit:
		return t.g.literal(x.T, x.V)
	case *ir.ParamRef:
		return t.param(x.Index)
	case *ir.ReadRef:
		return t.read(x.Index)
	case *ir.LocalRef:
		return verbatim(x.Name)
	case *ir.Unary:
		return t.unary(x, b)
	case *ir.Binary:
		return t.binary(x, b)
	case *ir.Call:
		return t.call(x, b)
	case *ir.CallFn:
		return t.callFn(x, b)
	case *ir.If:
		return t.ifExpr(x, b)
	case *ir.Template:
		return t.template(x, b)
	case *ir.IsCase:
		return t.isCase(x, b)
	case *ir.Coalesce:
		return t.coalesce(x)
	default:
		// ir writes a Let or a Block only as a body, and nothing else outside this switch.
		t.g.malformed(fmt.Sprintf(exprFormat, n), t.g.at)
		return cppInvalid
	}
}

func (t *tr) param(i int) string {
	if i < 0 || i >= len(t.fn.Params) {
		t.g.fail(fmt.Errorf("%w: parameter %d of %s", ErrMalformed, i, t.g.at))
		return cppInvalid
	}
	return verbatim(t.fn.Params[i].Name)
}

func (t *tr) read(i int) string {
	if i < 0 || i >= len(t.fn.Reads) {
		t.g.fail(fmt.Errorf("%w: read %d of %s", ErrMalformed, i, t.g.at))
		return cppInvalid
	}
	return verbatim(t.fn.Reads[i].Name)
}

func (t *tr) unary(x *ir.Unary, b *block) string {
	v := t.expr(x.X, b)
	switch {
	case x.Op == ir.OpNot:
		return fmt.Sprintf(notFormat, v)
	case x.Op == ir.OpNeg && x.T.Kind == types.Float:
		return fmt.Sprintf(helperFormat, negFloat, unparen(v))
	case x.Op == ir.OpNeg:
		return fmt.Sprintf(helperFormat, negInt, unparen(v))
	default:
		t.g.fail(fmt.Errorf("%w: unary operator %d in %s", ErrMalformed, x.Op, t.g.at))
		return cppInvalid
	}
}

// binary is a checked helper or a native comparison; `and`, `or` short-circuit (CONFORMANCE.md §3).
func (t *tr) binary(x *ir.Binary, b *block) string {
	if x.Op == ir.OpAnd || x.Op == ir.OpOr {
		left := t.expr(x.X, b)
		return fmt.Sprintf(nativeFormat, left, opText[x.Op], t.sub(x.Y))
	}
	ops := t.operands([]ir.PExpr{x.X, x.Y}, b)
	if !arithmetic(x.Op) {
		return fmt.Sprintf(nativeFormat, ops[0], opText[x.Op], ops[1])
	}
	xk, yk := x.X.Type().Kind, x.Y.Type().Kind
	helper := opHelper[x.Op] + intSuffix
	switch {
	case x.Op == ir.OpDiv && xk == types.Duration && yk == types.Duration:
		helper = divDuration
	case xk == types.Float || yk == types.Float:
		helper = opHelper[x.Op] + floatSuffix
	}
	return fmt.Sprintf(helper2Format, helper, unparen(ops[0]), unparen(ops[1]))
}

// call is a built-in of the portable subset; min and max of three or more nest left to right.
func (t *tr) call(x *ir.Call, b *block) string {
	args := t.operands(x.Args, b)
	for i := range args {
		args[i] = unparen(args[i])
	}
	suffix := intSuffix
	if len(x.Args) > 0 && x.Args[0].Type().Kind == types.Float {
		suffix = floatSuffix
	}
	name, ok := builtinHelper[x.Fn]
	switch {
	case !ok || len(args) == 0:
		t.g.fail(fmt.Errorf("%w: built-in %d in %s", ErrMalformed, x.Fn, t.g.at))
		return cppInvalid
	case x.Fn == ir.BuiltinMin || x.Fn == ir.BuiltinMax:
		acc := args[0]
		for _, a := range args[1:] {
			acc = fmt.Sprintf(helper2Format, name+suffix, acc, a)
		}
		return acc
	case x.Fn == ir.BuiltinAbs || x.Fn == ir.BuiltinClamp:
		name += suffix
	}
	return fmt.Sprintf(helperFormat, name, strings.Join(args, listSep))
}

// callFn calls a package-level translated fn; a method call is not emitted yet.
func (t *tr) callFn(x *ir.CallFn, b *block) string {
	if x.Fn == nil || x.Fn.Kind != ir.FnTranslated || !slices.Contains(t.g.pkgFns, x.Fn) {
		// ir's CallFn holds package fns only, and a stored one is E8013 in data mode: unreachable.
		t.g.malformed(methodCalls, t.g.at)
		return cppInvalid
	}
	args := t.operands(x.Args, b)
	for i := range args {
		args[i] = unparen(args[i])
	}
	return fmt.Sprintf(helperFormat, t.g.pl.FnName(x.Fn), strings.Join(args, listSep))
}

// ifExpr is a conditional; a String branch is a std::string, so no view outlives its string.
func (t *tr) ifExpr(x *ir.If, b *block) string {
	c := t.expr(x.Cond, b)
	a, e := t.sub(x.Then), t.sub(x.Else)
	if x.T.Kind == types.String {
		a, e = fmt.Sprintf(stringOfFormat, unparen(a)), fmt.Sprintf(stringOfFormat, unparen(e))
	}
	return fmt.Sprintf(ternaryFormat, c, a, e)
}

// coalesce is `x ?? y` on an optional read of self: y is evaluated only when x is none.
func (t *tr) coalesce(x *ir.Coalesce) string {
	r, ok := x.X.(*ir.ReadRef)
	if !ok || r.Index < 0 || r.Index >= len(t.fn.Reads) || !t.fn.Reads[r.Index].Optional {
		t.g.fail(fmt.Errorf("%w: ?? on something else than an optional read in %s", ErrMalformed, t.g.at))
		return cppInvalid
	}
	v, y := t.read(r.Index), t.sub(x.Y)
	some := fmt.Sprintf(derefFormat, v)
	if x.T.Kind == types.String {
		some, y = fmt.Sprintf(stringOfFormat, unparen(some)), fmt.Sprintf(stringOfFormat, unparen(y))
	}
	return fmt.Sprintf(ternaryFormat, v, some, y)
}

// template concatenates text and interpolations: strings, integers in decimal, enums by Canon name.
func (t *tr) template(x *ir.Template, b *block) string {
	var xs []ir.PExpr
	for _, p := range x.Parts {
		if p.X != nil {
			xs = append(xs, p.X)
		}
	}
	vals := t.operands(xs, b)
	var terms []string
	k := 0
	for _, p := range x.Parts {
		if p.Text != "" {
			terms = append(terms, quote(p.Text))
		}
		if p.X != nil {
			terms = append(terms, stringOf(p.X.Type(), unparen(vals[k])))
			k++
		}
	}
	switch {
	case len(terms) == 0:
		return fmt.Sprintf(stringOfFormat, "")
	case strings.HasPrefix(terms[0], quoteMark):
		terms[0] = fmt.Sprintf(stringOfFormat, terms[0])
	}
	return fmt.Sprintf(parenFormat, strings.Join(terms, concat))
}

// stringOf is an interpolated value as a std::string (CONFORMANCE.md §3, template).
func stringOf(ty ir.TypeRef, v string) string {
	switch ty.Kind {
	case types.Int:
		return fmt.Sprintf(toStringFormat, v)
	case types.Enum:
		return fmt.Sprintf(stringOfFormat, fmt.Sprintf(helperFormat, toNameFunc, v))
	default:
		return fmt.Sprintf(stringOfFormat, v)
	}
}

// isCase compares the kind the pure function receives with the case (CONFORMANCE.md §2.2).
func (t *tr) isCase(x *ir.IsCase, b *block) string {
	v := t.expr(x.X, b)
	variant, ok := x.X.Type().Named.(*ir.Variant)
	if !ok || x.Case < 0 || x.Case >= len(variant.Cases) {
		t.g.fail(fmt.Errorf("%w: is on a non-variant in %s", ErrMalformed, t.g.at))
		return cppInvalid
	}
	return fmt.Sprintf(nativeFormat, v, opText[ir.OpEq], t.g.kindMember(variant, x.Case))
}
