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

// pure is a translated fn as written (CODEGEN.md §5.10): its Go names, which its public method, its body and the conformance test share.
type pure struct {
	fn    *ir.ExportFn
	label string            // <T>.<fn>, or <fn> at package level: the Canon name failures print
	name  string            // the pure function
	test  string            // Test<T><Fn>Conformance (CONFORMANCE.md §7.1)
	names map[string]string // the Go local of each Canon read, parameter and let (CODEGEN.md §3.4)
	oks   []string          // per read, its ok parameter; "" when the read is not optional
	taken map[string]bool   // every Go name given in the fn
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
	name := g.names.MethodSlot(fn).Getter
	p := g.newPure(fn, label, lowerCamel(b.goName)+name, testPrefix+b.goName+name+conformanceWord)
	g.publicMethod(b, p, name)
	g.writePure(p, fmt.Sprintf(pureDocFormat, p.name, g.p.Dir+pathSep+fn.File, label))
}

// translatedFn writes a package-level translated fn, public and pure at once (CODEGEN.md §5.10).
func (g *gen) translatedFn(fn *ir.ExportFn) {
	defer g.enter(fn.Name)()
	if !g.translatable(fn) {
		return
	}
	name := g.names.MethodSlot(fn).Getter
	g.writePure(g.newPure(fn, fn.Name, name, testPrefix+name+conformanceWord), docFor(name, fn.Doc))
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

// newPure names the fn's locals: reads, then parameters, then lets, each escaped once.
func (g *gen) newPure(fn *ir.ExportFn, label, name, test string) *pure {
	p := &pure{fn: fn, label: label, name: name, test: test, names: map[string]string{}, taken: map[string]bool{name: true}}
	for _, r := range fn.Reads {
		p.bind(g, r.Name)
	}
	for _, prm := range fn.Params {
		p.bind(g, prm.Name)
	}
	for _, l := range letNames(fn.Body, nil) {
		p.bind(g, l)
	}
	for _, r := range fn.Reads {
		ok := ""
		if r.Optional {
			ok = p.fresh(g, p.names[r.Name]+okSuffix)
		}
		p.oks = append(p.oks, ok)
	}
	g.pures = append(g.pures, p)
	return p
}

// bind gives a Canon name its Go local, the same in every block that binds it.
func (p *pure) bind(g *gen, canon string) {
	if _, done := p.names[canon]; !done {
		p.names[canon] = p.fresh(g, canon)
	}
}

// fresh is name with `_` appended while it is reserved or already given (CODEGEN.md §3.4).
func (p *pure) fresh(g *gen, name string) string {
	for g.reserved(name) || p.taken[name] {
		name += underscore
	}
	p.taken[name] = true
	return name
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

// letNames are the names every `let` of a body binds, in order.
func letNames(n ir.PExpr, out []string) []string {
	switch x := n.(type) {
	case *ir.Let:
		return letNames(x.Body, append(out, x.Name))
	case *ir.If:
		return letNames(x.Else, letNames(x.Then, out))
	case *ir.Block:
		return blockLets(x, out)
	}
	return out
}

func blockLets(x *ir.Block, out []string) []string {
	if x == nil {
		return out
	}
	for _, st := range x.Stmts {
		switch s := st.(type) {
		case *ir.LetStmt:
			out = append(out, s.Name)
		case *ir.IfStmt:
			out = blockLets(s.Else, blockLets(s.Then, out))
		}
	}
	return out
}

// lowerCamel is a Go type name with its first word lower-cased: Potion, potion (CODEGEN.md §5.10).
func lowerCamel(goName string) string {
	ws := ir.Words(goName)
	if len(ws) == 0 {
		return goName
	}
	return strings.ToLower(ws[0]) + strings.Join(ws[1:], "")
}

// publicMethod calls the pure function on self's paths and the converted arguments (CONFORMANCE.md §2.3).
func (g *gen) publicMethod(b *body, p *pure, name string) {
	pre, args := g.readArgs(b, p)
	params := make([]string, 0, len(p.fn.Params))
	for _, prm := range p.fn.Params {
		local := p.names[prm.Name]
		params = append(params, local+space+g.goType(prm.Type))
		args = append(args, g.toPure(prm.Type, local))
	}
	call := g.fromPure(p.fn.Result, p.name+lparen+strings.Join(args, listSep)+rparen)
	g.exec(methodTemplate, methodView{
		Doc: docFor(name, p.fn.Doc), Type: b.goName, Name: name, Params: strings.Join(params, listSep),
		Result: g.goType(p.fn.Result), Pre: pre, Call: call,
	})
}

// writePure writes the pure function: the paths of self, then the parameters, their entry checks, then the checked body (CONFORMANCE.md §2.3).
func (g *gen) writePure(p *pure, doc string) {
	params := make([]string, 0, len(p.fn.Reads)+len(p.fn.Params))
	for i, r := range p.fn.Reads {
		params = append(params, p.names[r.Name]+space+g.pureType(r.Type))
		if p.oks[i] != "" {
			params = append(params, p.oks[i]+space+goBool)
		}
	}
	var entry strings.Builder
	for _, prm := range p.fn.Params {
		local := p.names[prm.Name]
		params = append(params, local+space+g.pureType(prm.Type))
		for _, c := range g.entryChecks(prm, local) {
			fmt.Fprintf(&entry, assignLineFormat, local, c)
		}
	}
	p.temps = 0
	g.exec(pureTemplate, pureView{
		Doc: doc, Name: p.name, Params: strings.Join(params, listSep), Result: g.pureType(p.fn.Result),
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
