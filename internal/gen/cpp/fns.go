package cppgen

import (
	"fmt"
	"math"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// pureType is the type a pure function takes (CONFORMANCE.md §2.3); a variant read is its kind.
func (g *gen) pureType(t ir.TypeRef) string {
	switch t.Kind {
	case types.Bool:
		return cppBool
	case types.Int, types.Duration:
		return cppInt64
	case types.Float:
		return cppDouble
	case types.String:
		return cppStringView
	case types.Enum:
		return g.typeName(t.Named)
	case types.Variant:
		if v, ok := t.Named.(*ir.Variant); ok {
			return g.kindName(v)
		}
	default:
	}
	// ir translates only scalar parameters and reads (E9006), a variant read left of `is`, and
	// scalar or ref results (E9004): unreachable.
	g.malformed(kindText(t.Kind), g.at)
	return cppInvalid
}

// pureResult is what a pure function returns: a String is built, a ref is its key (§2.1).
func (g *gen) pureResult(t ir.TypeRef) string {
	switch t.Kind {
	case types.String:
		return cppString
	case types.Ref:
		return g.storage(t)
	default:
		return g.pureType(t)
	}
}

// localType is a temporary's or a let's type: a String owns its bytes.
func (t *tr) localType(ty ir.TypeRef) string { return t.g.pureResult(ty) }

// readType is a read's pure type: std::optional when its path goes through an optional.
func (g *gen) readType(r *ir.Read) string {
	if r.Optional {
		return fmt.Sprintf(optionalFormat, g.pureType(r.Type))
	}
	return g.pureType(r.Type)
}

// pureSignature is `R name(reads…, params…)`, the paths of self first (CONFORMANCE.md §2.3).
func (g *gen) pureSignature(name string, fn *ir.ExportFn) string {
	var params []string
	for _, r := range fn.Reads {
		params = append(params, g.readType(r)+space+verbatim(r.Name))
	}
	for _, p := range fn.Params {
		params = append(params, g.pureType(p.Type)+space+verbatim(p.Name))
	}
	return fmt.Sprintf(signatureFormat, g.pureResult(fn.Result), name, strings.Join(params, listSep))
}

// pureFn writes an inline pure function: entry checks, then the checked body (CONFORMANCE.md §2.3).
func (g *gen) pureFn(name string, fn *ir.ExportFn) {
	if fn.Body == nil {
		g.fail(fmt.Errorf("%w: translated fn %s without a body", ErrMalformed, g.at))
		return
	}
	g.h.printf(pureOpenFormat, g.pureSignature(name, fn))
	t := &tr{g: g, fn: fn, used: map[string]bool{}}
	for _, r := range fn.Reads {
		t.used[verbatim(r.Name)] = true
	}
	for i, r := range fn.Reads {
		if !anyNode(fn.Body, refersTo(i, true)) {
			g.h.linef(1, unusedFormat, verbatim(r.Name))
		}
	}
	for i, p := range fn.Params {
		t.used[verbatim(p.Name)] = true
		checks := g.entryChecks(p)
		for _, c := range checks {
			g.h.linef(1, assignFormat, verbatim(p.Name), c)
		}
		if len(checks) == 0 && !anyNode(fn.Body, refersTo(i, false)) {
			g.h.linef(1, unusedFormat, verbatim(p.Name)) // -Wunused-parameter under -Werror (CODEGEN.md §9)
		}
	}
	collectLocals(fn.Body, t.used)
	var b block
	t.stmts(fn.Body, &b)
	for _, l := range b.lines {
		g.h.lineAt(1, l)
	}
	g.h.line(closeBrace)
}

// refersTo matches a read of self (read) or a declared parameter at index i.
func refersTo(i int, read bool) func(ir.PExpr) bool {
	return func(n ir.PExpr) bool {
		switch x := n.(type) {
		case *ir.ReadRef:
			return read && x.Index == i
		case *ir.ParamRef:
			return !read && x.Index == i
		default:
			return false
		}
	}
}

// collectLocals marks every `let` name of a body.
func collectLocals(n ir.PExpr, used map[string]bool) {
	switch x := n.(type) {
	case *ir.Let:
		used[verbatim(x.Name)] = true
		collectLocals(x.Body, used)
	case *ir.If:
		collectLocals(x.Then, used)
		collectLocals(x.Else, used)
	case *ir.Block:
		collectBlockLocals(x, used)
	default:
	}
}

// collectBlockLocals marks every `let` name of a Block and of its branches.
func collectBlockLocals(x *ir.Block, used map[string]bool) {
	if x == nil {
		return
	}
	for _, st := range x.Stmts {
		switch s := st.(type) {
		case *ir.LetStmt:
			used[verbatim(s.Name)] = true
		case *ir.IfStmt:
			collectBlockLocals(s.Then, used)
			collectBlockLocals(s.Else, used)
		}
	}
}

// entryChecks are the checks of one declared parameter, in order: a Float's finiteness
// (E4104), the sized type or Duration range (E3201), the range refinement (E3204).
func (g *gen) entryChecks(p *ir.Param) []string {
	var out []string
	name := verbatim(p.Name)
	if p.Type.Kind == types.Float {
		out = append(out, fmt.Sprintf(helperFormat, checkFloatArg, name))
	}
	if w := widthCheck(p.Type, name); w != "" {
		out = append(out, w)
	}
	if r := g.rangeCheck(p.Type, p.Range, name); r != "" {
		out = append(out, r)
	}
	return out
}

// inclusiveHi is the upper bound CheckFloatRange takes: for `..b`, the largest double below b.
func inclusiveHi(b *types.Bound) float64 {
	if b.HiIncluded {
		return b.Hi.F
	}
	return math.Nextafter(b.Hi.F, math.Inf(-1))
}

// exitChecks wraps a result: its sized type or Duration range, a Float32 rounding, then its range refinement.
func (g *gen) exitChecks(t ir.TypeRef, bound *types.Bound, v string) string {
	if w := widthCheck(t, v); w != "" {
		v = w
	}
	if t.Kind == types.Float && t.Bits == float32Bits {
		v = fmt.Sprintf(helperFormat, toFloat32, v)
	}
	if r := g.rangeCheck(t, bound, v); r != "" {
		v = r
	}
	return v
}

// widthCheck is CheckIntWidth for a sized integer or a Duration (TYPES.md §7.2), or "".
func widthCheck(t ir.TypeRef, v string) string {
	var basic types.Basic
	switch {
	case t.Kind == types.Duration:
		basic = types.Basic{K: types.Duration}
	case t.Kind == types.Int && (t.Bits != bits64 || !t.Signed):
		basic = types.Basic{K: types.Int, Bits: t.Bits, Signed: t.Signed}
	default:
		return ""
	}
	lo, hi, _ := basic.Limits()
	return fmt.Sprintf(helper3Format, checkIntWidth, v, intLit(lo), intLit(hi))
}

// rangeCheck is CheckIntRange or CheckFloatRange with inclusive bounds (for `a..b`, b − 1), or "".
func (g *gen) rangeCheck(t ir.TypeRef, b *types.Bound, v string) string {
	if b == nil || !b.HasLo && !b.HasHi {
		return ""
	}
	if t.Kind == types.Float {
		lo, hi := g.floatBounds(b)
		return fmt.Sprintf(helper3Format, checkFloatRange, v, lo, hi)
	}
	lo, hi := intBounds(b)
	return fmt.Sprintf(helper3Format, checkIntRange, v, lo, hi)
}

// floatBounds are a Float range's inclusive bounds as C++ literals, the double limits for an open end.
func (g *gen) floatBounds(b *types.Bound) (lo, hi string) {
	lo, hi = floatMinText, floatMaxText
	if b.HasLo {
		lo = g.floatLit(b.Lo.F, bits64)
	}
	if b.HasHi {
		hi = g.floatLit(inclusiveHi(b), bits64)
	}
	return lo, hi
}

// intBounds are an integer or Duration range's inclusive bounds (for `a..b`, b − 1), the int64 limits for an open end.
func intBounds(b *types.Bound) (lo, hi string) {
	lo, hi = intMinConst, intMaxConst
	if b.HasLo {
		lo = intLit(b.Lo.I)
	}
	if b.HasHi && b.HiIncluded {
		hi = intLit(b.Hi.I)
	} else if b.HasHi {
		hi = intLit(b.Hi.I - 1)
	}
	return lo, hi
}
