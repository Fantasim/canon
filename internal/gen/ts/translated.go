package tsgen

import (
	"fmt"
	"math"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// pureFn is a translated fn as written: its names, and what the conformance file needs to call it (CONFORMANCE.md §2.3, §7).
type pureFn struct {
	site         fnSite
	public, pure string
	inputs       []input
}

// input is a parameter of the pure function: a path of self, then a declared parameter.
type input struct {
	canon, name string
	t           ir.TypeRef
	optional    bool
	param       *ir.Param
}

// translated writes a translated fn: the pure function, and for a method the public function calling it (CODEGEN.md §5.10).
func (g *gen) translated(s fnSite) {
	fn := s.fn
	if fn.Err != nil || fn.Body == nil || len(fn.Vectors) == 0 {
		g.failf(ErrMalformed, malformedFn, s.label)
		return
	}
	p := &pureFn{site: s}
	p.public, p.pure = g.pureNames(s)
	g.declare(p.public, s.label)
	if p.pure != p.public {
		g.declare(p.pure, s.label)
	}
	p.inputs = g.inputsOf(fn)
	g.pures = append(g.pures, p)
	if p.pure != p.public {
		g.add(g.publicFn(p))
	}
	g.add(g.pureDecl(p))
}

// pureNames are the public and the pure name of a fn: a package-level fn is both; a method's pure form is `$` + the public name (CODEGEN.md §3.3).
func (g *gen) pureNames(s fnSite) (public, pure string) {
	if s.rec == nil && s.cs == nil {
		name := fnName(s.fn)
		return name, name
	}
	owner := lowerCamel(g.ownerName(s))
	public = escape(effective(s.fn.TS, owner+upperCamel(s.fn.Name)))
	return public, pureMark + public
}

// ownerName is the Canon name of a method's owner: the record, or the variant and its case.
func (g *gen) ownerName(s fnSite) string {
	if s.rec != nil {
		return s.rec.Name
	}
	return g.variantOf[s.cs].Name + underscore + s.cs.Name
}

// inputsOf are the pure function's parameters: the paths of self the body reads, then the declared parameters (CONFORMANCE.md §2.3).
func (g *gen) inputsOf(fn *ir.ExportFn) []input {
	var out []input
	for _, r := range fn.Reads {
		out = append(out, input{canon: r.Name, name: escape(r.Name), t: r.Type, optional: r.Optional})
	}
	for _, p := range fn.Params {
		out = append(out, input{canon: p.Name, name: escape(p.Name), t: p.Type, param: p})
	}
	return out
}

// pureType is the type a pure function takes and returns (CONFORMANCE.md §2.3): numbers, a variant read as its kind, a ref as its key.
func (g *gen) pureType(t ir.TypeRef) string {
	switch t.Kind {
	case types.Variant:
		return g.kindType(t.Named)
	case types.Ref:
		return g.keyType(t, false)
	default:
		return g.tsType(t, false)
	}
}

// paramText is `name: type`, `type | null` for an optional read.
func (g *gen) paramText(in input) string {
	typ := g.pureType(in.t)
	if in.optional {
		typ += unionSep + tsNull
	}
	return in.name + keyValueSep + typ
}

// pureDecl is the pure function: entry checks, then the body with its exit checks.
func (g *gen) pureDecl(p *pureFn) string {
	fn := p.site.fn
	params := make([]string, len(p.inputs))
	var body strings.Builder
	for i, in := range p.inputs {
		params[i] = g.paramText(in)
		body.WriteString(g.entryChecks(in))
	}
	t := &tr{g: g, fn: fn, inputs: p.inputs}
	body.WriteString(t.block())
	doc := ""
	if p.pure == p.public {
		doc = docComment("", fn.Doc)
	}
	return doc + fmt.Sprintf(functionFormat, p.pure, strings.Join(params, listSep), g.pureType(fn.Result), body.String())
}

// publicFn is the public method: self first, then the declared parameters; it calls the pure function on the paths of self (CODEGEN.md §3.3, §5.10).
func (g *gen) publicFn(p *pureFn) string {
	fn := p.site.fn
	params := []string{selfParam + keyValueSep + g.selfType(p.site)}
	args := make([]string, 0, len(p.inputs))
	for _, r := range fn.Reads {
		args = append(args, g.readExpr(p.site, r))
	}
	for _, in := range p.inputs[len(fn.Reads):] {
		params = append(params, g.paramText(in))
		args = append(args, in.name)
	}
	call := fmt.Sprintf(callFormat, p.pure, strings.Join(args, listSep))
	return docComment("", fn.Doc) + fmt.Sprintf(functionFormat, p.public, strings.Join(params, listSep), g.pureType(fn.Result), fmt.Sprintf(returnFormat, indent, call))
}

// selfType is the type of a method's receiver.
func (g *gen) selfType(s fnSite) string {
	if s.rec != nil {
		return typeName(s.rec)
	}
	return caseName(g.variantOf[s.cs], s.cs)
}

// readExpr is a path of self as the pure function takes it: `self.a.b`, `?.` after an optional step, `?? null` when a step made it optional, `.kind` for a variant (CONFORMANCE.md §2.3).
func (g *gen) readExpr(s fnSite, r *ir.Read) string {
	fields := s.fieldsOf()
	var b strings.Builder
	b.WriteString(selfParam)
	chained := false
	for i, seg := range r.Path {
		f, m := fieldByName(fields, seg), methodByName(s.methods(), seg)
		var prop string
		switch {
		case f != nil:
			prop, fields = fieldProp(f), nested(f)
		case i == 0 && m != nil:
			prop = fnProp(m)
		default:
			g.failf(ErrMalformed, malformedRead, strings.Join(r.Path, dot), g.at)
			return tsUndefined
		}
		b.WriteString(g.step(chained) + prop)
		chained = chained || f != nil && f.Optional && i < len(r.Path)-1
	}
	if r.Type.Kind == types.Variant {
		b.WriteString(g.step(chained) + kindProp)
	}
	if chained {
		b.WriteString(coalesceNull)
	}
	return b.String()
}

// step is the access operator after a path that may have passed an optional record.
func (g *gen) step(chained bool) string {
	if chained {
		return optionalDot
	}
	return dot
}

// fieldsOf are the fields of a method's owner.
func (s fnSite) fieldsOf() []*ir.Field {
	if s.rec != nil {
		return s.rec.Fields
	}
	return s.cs.Fields
}

// methods are the methods of a method's owner, which a path may read as a field.
func (s fnSite) methods() []*ir.ExportFn {
	if s.rec != nil {
		return s.rec.Methods
	}
	return s.cs.Methods
}

func fieldByName(fields []*ir.Field, name string) *ir.Field {
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func methodByName(fns []*ir.ExportFn, name string) *ir.ExportFn {
	for _, fn := range fns {
		if fn.Name == name && fn.Kind == ir.FnPrecomputed {
			return fn
		}
	}
	return nil
}

// nested are the fields of the record a field holds, nil for any other type.
func nested(f *ir.Field) []*ir.Field {
	if r, ok := f.Type.Named.(*ir.Record); ok && f.Type.Kind == types.Record {
		return r.Fields
	}
	return nil
}

// entryChecks are one input's checks on entry, in CONFORMANCE.md §2.3's order: representability (E8303 for an integer, E4104 for a Float), the sized type or Duration range (E3201), the range refinement (E3204).
func (g *gen) entryChecks(in input) string {
	var out strings.Builder
	set := func(call string) { out.WriteString(indent + in.name + assignSep + call + semicolon + newline) }
	switch in.t.Kind {
	case types.Int, types.Duration:
		set(g.optionalCall(in, g.helper(canonIntName)))
	case types.Float:
		set(g.optionalCall(in, g.helper(canonFName)))
	default:
	}
	if in.param == nil {
		return out.String()
	}
	if w := g.widthCheck(in.t, in.name); w != "" {
		set(w)
	}
	if r := g.rangeCheck(in.t, in.param.Range, in.name); r != "" {
		set(r)
	}
	return out.String()
}

// optionalCall applies a check to a read that may be null.
func (g *gen) optionalCall(in input, helper string) string {
	if in.optional {
		return fmt.Sprintf(nullGuardFormat, in.name, helper, in.name)
	}
	return fmt.Sprintf(callFormat, helper, in.name)
}

// widthCheck is canonCheckWidth for a sized integer or a Duration (TYPES.md §7.2), or "".
func (g *gen) widthCheck(t ir.TypeRef, v string) string {
	var basic types.Basic
	switch {
	case t.Kind == types.Duration:
		basic = types.Basic{K: types.Duration}
	case t.Kind == types.Int && (t.Bits != int64Bits || !t.Signed):
		basic = types.Basic{K: types.Int, Bits: t.Bits, Signed: t.Signed}
	default:
		return ""
	}
	lo, hi, _ := basic.Limits()
	return fmt.Sprintf(checkFormat, g.helper(canonCheckWidthName), v, intText(lo), intText(hi))
}

// rangeCheck is canonCheckRange with inclusive bounds (`a..b` checks b − 1; a Float's, the double below b), or "".
func (g *gen) rangeCheck(t ir.TypeRef, b *types.Bound, v string) string {
	if b == nil || !b.HasLo && !b.HasHi {
		return ""
	}
	lo, hi := intBounds(b)
	if t.Kind == types.Float {
		lo, hi = floatBounds(b)
	}
	return fmt.Sprintf(checkFormat, g.helper(canonCheckRangeName), v, lo, hi)
}

func intBounds(b *types.Bound) (lo, hi string) {
	l, h := int64(math.MinInt64), int64(math.MaxInt64)
	if b.HasLo {
		l = b.Lo.I
	}
	if b.HasHi {
		h = b.Hi.I
		if !b.HiIncluded {
			h--
		}
	}
	return intText(l), intText(h)
}

func floatBounds(b *types.Bound) (lo, hi string) {
	l, h := -math.MaxFloat64, math.MaxFloat64
	if b.HasLo {
		l = b.Lo.F
	}
	if b.HasHi {
		h = b.Hi.F
		if !b.HiIncluded {
			h = math.Nextafter(h, math.Inf(-1))
		}
	}
	return floatText(l), floatText(h)
}

// exitChecks wrap a result: its sized type or Duration range, a Float32 rounding, its range refinement (CONFORMANCE.md §2.3).
func (g *gen) exitChecks(fn *ir.ExportFn, v string) string {
	if w := g.widthCheck(fn.Result, v); w != "" {
		v = w
	}
	if fn.Result.Kind == types.Float && fn.Result.Bits == float32Bits {
		v = fmt.Sprintf(callFormat, g.helper(canonF32Name), v)
	}
	if r := g.rangeCheck(fn.Result, fn.ResultRange, v); r != "" {
		v = r
	}
	return v
}
