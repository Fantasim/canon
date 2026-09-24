package gogen

import (
	"fmt"
	"go/token"
	gotypes "go/types"
	"math"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// pure is a translated fn as written: the plan's names (ir.GoPure), and the numbered temps the plan does not name (CODEGEN.md §5.10).
type pure struct {
	fn    *ir.ExportFn
	label string          // <T>.<fn>, or <fn> at package level: the Canon name failures print
	plan  *ir.GoPure      // the pure function, the public method, the test, the locals (ir's plan)
	taken map[string]bool // the plan's locals and ok parameters, which a fresh temp must not repeat
	temps int
}

// methodView and pureView are the shapes of text/translated.txt.
type methodView struct {
	Doc, Type, Name, Params, Result, Pre, Call string
}

type pureView struct {
	Doc, Name, Params, Result, Body string
}

// translatedMethod writes a record's or case's translated method: the public method, then its pure function with T3 (CODEGEN.md §2.5, §5.10).
func (g *gen) translatedMethod(b *body, fn *ir.ExportFn) {
	label := b.canon + dot + fn.Name
	defer g.enter(label)()
	if !g.translatable(fn) {
		return
	}
	p := g.newPure(fn, label)
	g.publicMethod(b, p)
	g.writePure(p, fmt.Sprintf(pureDocFormat, p.plan.Pure, g.p.Dir+pathSep+fn.File, label))
}

// translatedFn writes a package-level translated fn, public and pure at once (CODEGEN.md §5.10).
func (g *gen) translatedFn(fn *ir.ExportFn) {
	defer g.enter(fn.Name)()
	if !g.translatable(fn) {
		return
	}
	p := g.newPure(fn, fn.Name)
	g.writePure(p, docFor(p.plan.Public, fn.Doc))
}

// translatable refuses a translated fn stage E left without a body, a source file or vectors.
func (g *gen) translatable(fn *ir.ExportFn) bool {
	switch {
	case fn.Err != nil:
		g.fail(fmt.Errorf("%w: %s: %w", ErrMalformed, g.at, fn.Err))
	case fn.Body == nil:
		g.failf(ErrMalformed, "translated fn %s without a body", g.at)
	case fn.File == "":
		g.failf(ErrMalformed, "translated fn %s without its source file", g.at)
	case len(fn.Vectors) == 0:
		g.failf(ErrMalformed, "translated fn %s without conformance vectors", g.at)
	default:
		return true
	}
	return false
}

// newPure takes the fn's names and locals off the plan, never a nil dereference (CODEGEN.md §3.3-§3.4, §5.10).
func (g *gen) newPure(fn *ir.ExportFn, label string) *pure {
	plan := g.names.Pure(fn)
	if plan == nil {
		g.failf(ErrMalformed, "translated fn %s has no plan", label)
		plan = &ir.GoPure{
			Locals: map[string]string{}, OKs: make([]string, len(fn.Reads)),
			Fields: make([]string, len(fn.Reads)+len(fn.Params)), FieldOKs: make([]string, len(fn.Reads)),
		}
	}
	p := &pure{fn: fn, label: label, plan: plan, taken: map[string]bool{plan.Pure: true}}
	for _, local := range plan.Locals { //canon:unordered a set
		p.taken[local] = true
	}
	for _, ok := range plan.OKs {
		if ok != "" {
			p.taken[ok] = true
		}
	}
	g.pures = append(g.pures, p)
	return p
}

// temp is a numbered local of the function being written, which no Canon name takes.
func (p *pure) temp(g *gen, base string) string {
	for {
		p.temps++
		if name := base + strconv.Itoa(p.temps); !g.reserved(name) && !p.taken[name] {
			p.taken[name] = true
			return name
		}
	}
}

// reserved reports a local Go cannot take: a keyword, a predeclared identifier, an import, self (CODEGEN.md §3.4), or a package-level name a body refers to unqualified.
func (g *gen) reserved(name string) bool {
	return token.IsKeyword(name) || gotypes.Universe.Lookup(name) != nil || name == selfRecv ||
		goImportNames[name] || g.taken[name] || g.packageNames()[name]
}

// packageNames are the package-level names a translated body or method may use unqualified: types, enum and kind members, table ids, package fns.
func (g *gen) packageNames() map[string]bool {
	if g.pkgNames != nil {
		return g.pkgNames
	}
	g.pkgNames = map[string]bool{}
	for _, t := range g.p.Types {
		g.typeNames(t)
	}
	for _, v := range g.p.Values {
		if v.Type.Kind == types.Table && v.Type.Elem != nil {
			for _, id := range v.IDs {
				g.pkgNames[g.names.IDMemberName(v.Type.Elem.Named, id)] = true
			}
		}
	}
	for _, fn := range g.p.Fns {
		g.pkgNames[g.names.MethodSlot(fn).Getter] = true
	}
	return g.pkgNames
}

// typeNames adds a type's name, and an enum's member constants or a variant's kind members.
func (g *gen) typeNames(t ir.Type) {
	g.pkgNames[g.names.TypeName(t)] = true
	switch x := t.(type) {
	case *ir.Enum:
		for _, m := range x.Members {
			g.pkgNames[g.names.MemberName(x, m)] = true
		}
	case *ir.Variant:
		for _, c := range x.Cases {
			g.pkgNames[g.names.KindMemberName(x, c)] = true
		}
	}
}

// publicMethod calls the pure function on self's paths and the converted arguments (CONFORMANCE.md §2.3).
func (g *gen) publicMethod(b *body, p *pure) {
	pre, args := g.readArgs(b, p)
	params := make([]string, 0, len(p.fn.Params))
	for _, prm := range p.fn.Params {
		local := p.plan.Locals[prm.Name]
		params = append(params, local+space+g.goType(prm.Type))
		args = append(args, g.toPure(prm.Type, local))
	}
	call := g.fromPure(p.fn.Result, p.plan.Pure+lparen+strings.Join(args, listSep)+rparen)
	g.exec(methodTemplate, methodView{
		Doc: docFor(p.plan.Public, p.fn.Doc), Type: b.goName, Name: p.plan.Public, Params: strings.Join(params, listSep),
		Result: g.goType(p.fn.Result), Pre: pre, Call: call,
	})
}

// writePure writes the pure function: the paths of self, then the parameters, their entry checks, then the checked body (CONFORMANCE.md §2.3).
func (g *gen) writePure(p *pure, doc string) {
	params := make([]string, 0, len(p.fn.Reads)+len(p.fn.Params))
	for i, r := range p.fn.Reads {
		params = append(params, p.plan.Locals[r.Name]+space+g.pureType(r.Type))
		if p.plan.OKs[i] != "" {
			params = append(params, p.plan.OKs[i]+space+goBool)
		}
	}
	var entry strings.Builder
	for _, prm := range p.fn.Params {
		local := p.plan.Locals[prm.Name]
		params = append(params, local+space+g.pureType(prm.Type))
		for _, c := range g.entryChecks(prm, local) {
			fmt.Fprintf(&entry, assignLineFormat, local, c)
		}
	}
	p.temps = 0
	g.exec(pureTemplate, pureView{
		Doc: doc, Name: p.plan.Pure, Params: strings.Join(params, listSep), Result: g.pureType(p.fn.Result),
		Body: entry.String() + (&tr{g: g, p: p}).body(),
	})
}

// pureType is the type a pure function takes and returns (CONFORMANCE.md §2.3): every integer and a Duration's milliseconds are int64, a variant read its kind, a ref result its key.
func (g *gen) pureType(t ir.TypeRef) string {
	switch t.Kind {
	case types.Bool:
		return goBool
	case types.Int, types.Duration:
		return goInt64
	case types.Float:
		return goFloat64
	case types.String, types.LitUnion:
		return goString
	case types.Enum, types.Ref:
		return g.goType(t)
	case types.Variant:
		return g.goType(ir.TypeRef{Kind: types.VariantKind, Named: t.Named})
	default:
		g.failKind(t.Kind)
		return ""
	}
}

// toPure converts what a public method holds to the pure type: sized numbers widen, a
// Duration is its milliseconds, a variant its kind.
func (g *gen) toPure(t ir.TypeRef, x string) string {
	switch {
	case t.Kind == types.Duration:
		return g.rt() + dot + durationToMs + lparen + x + rparen
	case sized(t):
		return goInt64 + lparen + x + rparen
	case t.Kind == types.Float && t.Bits == float32Bits:
		return goFloat64 + lparen + x + rparen
	case t.Kind == types.Variant:
		return x + dot + ir.GoKind + callSuffix
	default:
		return x
	}
}

// fromPure converts a pure result back: milliseconds to a Duration, a sized number narrowed after its check.
func (g *gen) fromPure(t ir.TypeRef, call string) string {
	switch {
	case t.Kind == types.Duration:
		return g.rt() + durationFromMs + call + rparen
	case sized(t), t.Kind == types.Float && t.Bits == float32Bits:
		return g.goType(t) + lparen + call + rparen
	default:
		return call
	}
}

// sized reports a sized integer type: every other width or sign than Int's (TYPES.md §7.2).
func sized(t ir.TypeRef) bool {
	return t.Kind == types.Int && (t.Bits != int64Bits || !t.Signed)
}

// entryChecks are one parameter's checks in order: a Float's finiteness (E4104), the sized type or Duration range (E3201), the range refinement (E3204) (CONFORMANCE.md §2.3).
func (g *gen) entryChecks(p *ir.Param, local string) []string {
	var out []string
	if p.Type.Kind == types.Float {
		out = append(out, g.rtCall(checkFloatArg, local))
	}
	if w := g.widthCheck(p.Type, local); w != "" {
		out = append(out, w)
	}
	if r := g.rangeCheck(p.Type, p.Range, local); r != "" {
		out = append(out, r)
	}
	return out
}

// exitChecks wrap a result: its sized type or Duration range, a Float32 rounding, its range refinement.
func (g *gen) exitChecks(t ir.TypeRef, bound *types.Bound, v string) string {
	if w := g.widthCheck(t, v); w != "" {
		v = w
	}
	if t.Kind == types.Float && t.Bits == float32Bits {
		v = goFloat64 + lparen + g.rtCall(toFloat32, v) + rparen
	}
	if r := g.rangeCheck(t, bound, v); r != "" {
		v = r
	}
	return v
}

// widthCheck is rt.CheckIntWidth for a sized integer or a Duration (TYPES.md §7.2), or "".
func (g *gen) widthCheck(t ir.TypeRef, v string) string {
	var basic types.Basic
	switch {
	case t.Kind == types.Duration:
		basic = types.Basic{K: types.Duration}
	case sized(t):
		basic = types.Basic{K: types.Int, Bits: t.Bits, Signed: t.Signed}
	default:
		return ""
	}
	lo, hi, _ := basic.Limits()
	return g.rtCall(checkIntWidth, v, intText(lo), intText(hi))
}

// rangeCheck is rt.CheckIntRange or rt.CheckFloatRange with inclusive bounds (`a..b` checks b − 1), or "".
func (g *gen) rangeCheck(t ir.TypeRef, b *types.Bound, v string) string {
	if b == nil || !b.HasLo && !b.HasHi {
		return ""
	}
	if t.Kind == types.Float {
		lo, hi := -math.MaxFloat64, math.MaxFloat64
		if b.HasLo {
			lo = b.Lo.F
		}
		if b.HasHi {
			hi = b.Hi.F
			if !b.HiIncluded {
				hi = math.Nextafter(hi, math.Inf(-1))
			}
		}
		return g.rtCall(checkFloatRange, v, g.floatLit(lo, int64Bits), g.floatLit(hi, int64Bits))
	}
	lo, hi := int64(math.MinInt64), int64(math.MaxInt64)
	if b.HasLo {
		lo = b.Lo.I
	}
	if b.HasHi {
		hi = b.Hi.I
		if !b.HiIncluded {
			hi--
		}
	}
	return g.rtCall(checkIntRange, v, intText(lo), intText(hi))
}

// rtCall is a call of an rt helper.
func (g *gen) rtCall(helper string, args ...string) string {
	return g.rt() + dot + helper + lparen + strings.Join(args, listSep) + rparen
}

func intText(n int64) string { return strconv.FormatInt(n, decimal) }
