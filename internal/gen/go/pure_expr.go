package gogen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// helperPair is an rt helper's name for Int operands, then for Float ones.
type helperPair struct{ int, float string }

func (h helperPair) of(float bool) string {
	if float {
		return h.float
	}
	return h.int
}

// expr is the Go expression of n; statements it needs first go to b (Go has no conditional
// expression, so `if`, `??` and a short-circuit with statements become locals).
func (t *tr) expr(n ir.PExpr, b *lines) string {
	switch x := n.(type) {
	case *ir.Lit:
		return t.g.pureLit(x.T, x.V)
	case *ir.ParamRef:
		return t.param(x.Index)
	case *ir.ReadRef:
		return t.read(x.Index, false)
	case *ir.LocalRef:
		return t.local(x.Name)
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
		return t.isCase(x)
	case *ir.Coalesce:
		return t.coalesce(x, b)
	}
	// ir writes a Let or a Block only as a body, and nothing else outside this switch.
	t.g.failf(ErrMalformed, "the expression %T in %s", n, t.g.at)
	return nilLit
}

// pureLit is a literal of the pure types: a Duration its milliseconds, a float at float64 width
// (a Float32 value is exact there), a ref its key.
func (g *gen) pureLit(t ir.TypeRef, v value.Value) string {
	switch t.Kind {
	case types.Duration:
		return intText(as[value.Dur](g, v).Ms)
	case types.Float:
		return g.floatLit(as[value.Float](g, v).V, int64Bits)
	case types.Ref:
		return g.pureKey(t, as[value.Ref](g, v).Key)
	default:
		return g.expr(t, v)
	}
}

// pureKey is a ref's key; a data-mode table id is its string id type (CODEGEN.md §5.3).
func (g *gen) pureKey(t ir.TypeRef, k value.Key) string {
	if g.isData() && isTableRef(t.Ref) {
		return g.keyType(t) + lparen + strconv.Quote(k.S) + rparen
	}
	return g.keyLit(t, k)
}

func (t *tr) param(i int) string {
	if i < 0 || i >= len(t.p.fn.Params) {
		t.g.failf(ErrMalformed, "parameter %d of %s", i, t.g.at)
		return nilLit
	}
	return t.p.plan.Locals[t.p.fn.Params[i].Name]
}

// read is a path of self's parameter; an optional one is read only by `??`.
func (t *tr) read(i int, optional bool) string {
	if i < 0 || i >= len(t.p.fn.Reads) || t.p.fn.Reads[i].Optional != optional {
		t.g.failf(ErrMalformed, "read %d of %s", i, t.g.at)
		return nilLit
	}
	return t.p.plan.Locals[t.p.fn.Reads[i].Name]
}

func (t *tr) unary(x *ir.Unary, b *lines) string {
	v := t.expr(x.X, b)
	switch x.Op {
	case ir.OpNot:
		return not + nativeOperand(ir.OpNot, x.X, v, true)
	case ir.OpNeg:
		return t.g.rtCall(negHelper.of(x.T.Kind == types.Float), v)
	default:
		t.g.failf(ErrMalformed, "unary operator %d in %s", x.Op, t.g.at)
		return nilLit
	}
}

// binary is a checked helper or a native comparison; `and`, `or` short-circuit (CONFORMANCE.md §3).
func (t *tr) binary(x *ir.Binary, b *lines) string {
	if x.Op == ir.OpAnd || x.Op == ir.OpOr {
		return t.logic(x, b)
	}
	ops := t.operands([]ir.PExpr{x.X, x.Y}, b)
	helper, arithmetic := opHelper[x.Op]
	text, native := opText[x.Op]
	xk, yk := x.X.Type().Kind, x.Y.Type().Kind
	switch {
	case !arithmetic && !native:
		t.g.failf(ErrMalformed, "binary operator %d in %s", x.Op, t.g.at)
		return nilLit
	case !arithmetic:
		return nativeOperand(x.Op, x.X, ops[0], false) + space + text + space + nativeOperand(x.Op, x.Y, ops[1], true)
	case x.Op == ir.OpDiv && xk == types.Duration && yk == types.Duration:
		return t.g.rtCall(divDuration, ops...)
	}
	return t.g.rtCall(helper.of(xk == types.Float || yk == types.Float), ops...)
}

// logic is `x && y` or `x || y`; a right side that needs statements runs only when it decides.
func (t *tr) logic(x *ir.Binary, b *lines) string {
	left := nativeOperand(x.Op, x.X, t.expr(x.X, b), false)
	var sub lines
	right := nativeOperand(x.Op, x.Y, t.expr(x.Y, &sub), true)
	if len(sub.l) == 0 {
		return left + space + opText[x.Op] + space + right
	}
	tmp := t.p.temp(t.g, tempLocal)
	b.add(defineFormat, tmp, left)
	cond := tmp
	if x.Op == ir.OpOr {
		cond = not + tmp
	}
	b.add(ifOpenFormat, cond)
	b.put(sub.l...)
	b.add(assignLineFormat, tmp, right)
	b.add(closeBrace)
	return tmp
}

// nativeOperand parenthesizes a comparison or a short-circuit operand of parent that Go would
// otherwise group differently: one binding less tightly, or as tightly on the right.
func nativeOperand(parent ir.Op, x ir.PExpr, v string, right bool) string {
	n, ok := x.(*ir.Binary)
	if !ok || inert(v) || opPrecedence[n.Op] == 0 {
		return v
	}
	if p, c := opPrecedence[parent], opPrecedence[n.Op]; c < p || c == p && right {
		return lparen + v + rparen
	}
	return v
}

// inert reports Go text that cannot fail when evaluated: a name, a qualified name or a number.
func inert(v string) bool {
	s := strings.TrimPrefix(v, minus)
	return s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune(inertMarks, r)
	}) < 0
}

// operands translates xs in order; when one needs statements, the operands before it that can fail are bound to locals first, so errors keep the evaluator's order (CONFORMANCE.md §1, §2.3).
func (t *tr) operands(xs []ir.PExpr, b *lines) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		var sub lines
		out[i] = t.expr(x, &sub)
		if len(sub.l) == 0 {
			continue
		}
		for j := range i {
			if !inert(out[j]) {
				out[j] = t.bind(xs[j], out[j], b)
			}
		}
		b.put(sub.l...)
	}
	return out
}

// call is a built-in of the portable subset: Int min and max are Go's, Float ones nest left to right.
func (t *tr) call(x *ir.Call, b *lines) string {
	args := t.operands(x.Args, b)
	name, ok := builtinHelper[x.Fn]
	if !ok || len(args) == 0 {
		t.g.failf(ErrMalformed, "built-in %d in %s", x.Fn, t.g.at)
		return nilLit
	}
	float := x.Args[0].Type().Kind == types.Float
	minMax := x.Fn == ir.BuiltinMin || x.Fn == ir.BuiltinMax
	switch {
	case minMax && !float:
		return name.int + lparen + strings.Join(args, listSep) + rparen
	case minMax:
		acc := args[0]
		for _, a := range args[1:] {
			acc = t.g.rtCall(name.float, acc, a)
		}
		return acc
	}
	return t.g.rtCall(name.of(float), args...)
}

// callFn calls a package fn: a translated one's pure function, or a lookup function read as a pure value.
func (t *tr) callFn(x *ir.CallFn, b *lines) string {
	fn := x.Fn
	if fn == nil || !slices.Contains(t.g.p.Fns, fn) {
		t.g.failf(ErrMalformed, "a call of an export fn that is no package fn of %s, in %s", t.g.p.Name, t.g.at)
		return nilLit
	}
	args := lparen + strings.Join(t.operands(x.Args, b), listSep) + rparen
	switch {
	case fn.Kind == ir.FnTranslated:
		return t.g.names.MethodSlot(fn).Getter + args
	case fn.Kind == ir.FnLookup && scalarKinds[fn.Result.Kind]:
		return t.g.toPure(fn.Result, t.g.names.Finite(fn).Name+args)
	case fn.Kind == ir.FnLookup && fn.Result.Kind == types.Ref:
		return t.g.lookupKey(fn, args)
	}
	// ir's translator calls a lookup only when its result is a scalar or a ref (E9001 otherwise).
	t.g.fail(newDetail(ErrMalformed, fn.Name, "a call of %s, whose result is an optional or composite lookup, in %s", fn.Name, t.g.at))
	return nilLit
}

// lookupKey is the key a package lookup of a ref result returns: read off its entry when the lookup resolves it, as a derived key getter does (CODEGEN.md §5.8, §5.10).
func (g *gen) lookupKey(fn *ir.ExportFn, args string) string {
	f := g.names.Finite(fn)
	if !f.Result.Main {
		return f.Name + args
	}
	return f.Name + args + dot + g.targetKey(f.Result.Ref)
}

// ifExpr is a conditional: a local assigned in the branch taken, the other never evaluated.
func (t *tr) ifExpr(x *ir.If, b *lines) string {
	c := t.expr(x.Cond, b)
	tmp := t.p.temp(t.g, tempLocal)
	b.add(varFormat, tmp, t.g.pureType(x.T))
	b.add(ifOpenFormat, c)
	t.assign(tmp, x.Then, b)
	b.add(elseLine)
	t.assign(tmp, x.Else, b)
	b.add(closeBrace)
	return tmp
}

// assign sets tmp to x inside a branch.
func (t *tr) assign(tmp string, x ir.PExpr, b *lines) {
	v := t.expr(x, b)
	b.add(assignLineFormat, tmp, v)
}

// coalesce is `x ?? y` on an optional read of self: y is evaluated only when x is none.
func (t *tr) coalesce(x *ir.Coalesce, b *lines) string {
	r, ok := x.X.(*ir.ReadRef)
	if !ok {
		t.g.failf(ErrMalformed, "?? on something else than a read of self in %s", t.g.at)
		return nilLit
	}
	v := t.read(r.Index, true)
	tmp := t.p.temp(t.g, tempLocal)
	b.add(defineFormat, tmp, v)
	b.add(ifOpenFormat, not+t.p.plan.OKs[r.Index])
	t.assign(tmp, x.Y, b)
	b.add(closeBrace)
	return tmp
}

// template concatenates text and interpolations: strings, integers in decimal, enums by Canon name.
func (t *tr) template(x *ir.Template, b *lines) string {
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
			terms = append(terms, strconv.Quote(p.Text))
		}
		if p.X != nil {
			terms = append(terms, t.g.stringOf(p.X.Type(), vals[k]))
			k++
		}
	}
	if len(terms) == 0 {
		return emptyString
	}
	return strings.Join(terms, concat)
}

// stringOf is an interpolated value as a string (CONFORMANCE.md §3, template).
func (g *gen) stringOf(t ir.TypeRef, v string) string {
	switch t.Kind {
	case types.Int:
		return fmt.Sprintf(formatIntFormat, g.use(strconvPkg, strconvPkg), v)
	case types.Enum:
		return v + dot + ir.GoString + callSuffix
	default:
		return v
	}
}

// isCase compares the kind the pure function receives with the case (CONFORMANCE.md §2.2).
func (t *tr) isCase(x *ir.IsCase) string {
	r, ok := x.X.(*ir.ReadRef)
	variant, isVariant := x.X.Type().Named.(*ir.Variant)
	if !ok || !isVariant || x.Case < 0 || x.Case >= len(variant.Cases) {
		t.g.failf(ErrMalformed, "is on something else than a variant read in %s", t.g.at)
		return nilLit
	}
	return t.read(r.Index, false) + space + opText[ir.OpEq] + space + t.g.kindLit(variant, x.Case)
}
