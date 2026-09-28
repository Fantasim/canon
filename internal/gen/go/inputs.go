package gogen

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// inputCheck is one refinement of a runtime input (EVALUATION.md §11.3).
type inputCheck struct{ cond, reason string }

// inputSpec is one field's parsed-and-checked env read (CODEGEN.md §5.12).
type inputSpec struct {
	value, ok, env, parsed, errv, raw string
	parseFn                           string
	kind                              types.Kind
	assign                            string
	checks                            []inputCheck
}

// inputVars is field f of record rec's package slot, from the plan (CODEGEN.md §5.12).
func (g *gen) inputVars(rec *ir.Record, f *ir.Field) (value, ok string) {
	in := g.names.Input(rec, f)
	return in.Value, in.OK
}

// inputPatternVars are field f's compiled patterns, one package var per pattern of its alias chain, in Field.Patterns' order (EVALUATION.md §11.3, TYPES.md §7.4).
func (g *gen) inputPatternVars(rec *ir.Record, f *ir.Field) []string {
	return g.names.Input(rec, f).Patterns
}

// inputReason is ir.InputReasonText; "" (a Kind or reason it names no text for) is a plan defect.
func (g *gen) inputReason(r ir.InputReason, t ir.TypeRef) string {
	s := ir.InputReasonText(r, t)
	if s == "" {
		g.failf(ErrMalformed, "no input failure text for kind %s (EVALUATION.md §11.3)", kindText(t.Kind))
	}
	return s
}

// assignStmt is value, ok = expr, true, or value = expr when the field is required.
func assignStmt(value, ok, expr string) string {
	if ok == "" {
		return fmt.Sprintf(inputAssignFormat, value, expr)
	}
	return fmt.Sprintf(inputAssignOKFormat, value, ok, expr)
}

// resetStmt is value, ok = zero, false, or value = zero when the field is required.
func resetStmt(value, ok, zero string) string {
	if ok == "" {
		return fmt.Sprintf(inputAssignFormat, value, zero)
	}
	return fmt.Sprintf(inputResetOKFormat, value, ok, zero)
}

// inputGetter is an input field's getter (CODEGEN.md §5.12).
func (g *gen) inputGetter(s *slot) getter {
	if s.rec == nil {
		g.failf(ErrMalformed, "input field %s without a record owner (EVALUATION.md §11.1)", s.origin)
		return getter{name: s.Getter, result: results(g.mainType(s), s.needsOK())}
	}
	value, ok := g.inputVars(s.rec, s.src)
	code, msg := ir.InputGetterFailure(s.origin)
	body := fmt.Sprintf(inputCheckFormat, g.names.Inputs().Loaded, g.rt(), strconv.Quote(code), strconv.Quote(msg)) +
		returns(value, s.needsOK(), ok)
	return getter{name: s.Getter, result: results(g.mainType(s), s.needsOK()), body: body, doc: s.doc}
}

// inputRecords are the package's records with an input field, in declaration order.
func (g *gen) inputRecords() []*ir.Record {
	var out []*ir.Record
	for _, t := range g.p.Types {
		if r, ok := t.(*ir.Record); ok && hasInputField(r) {
			out = append(out, r)
		}
	}
	return out
}

func hasInputField(r *ir.Record) bool {
	for _, f := range r.Fields {
		if f.Input != nil {
			return true
		}
	}
	return false
}

// runtimeInputs writes every input field's package slot and LoadInputs (CODEGEN.md §5.12).
func (g *gen) runtimeInputs() {
	recs := g.inputRecords()
	if len(recs) == 0 {
		return
	}
	g.inputDecls(recs)
	g.loadInputsFunc(recs)
}

// inputDecls declares every input's package slot, its compiled pattern if any, and the
// package's one loaded flag.
func (g *gen) inputDecls(recs []*ir.Record) {
	g.printf(inputVarBlockOpen)
	for _, rec := range recs {
		for _, f := range rec.Fields {
			if f.Input == nil {
				continue
			}
			value, ok := g.inputVars(rec, f)
			g.printf(inputVarLine, value, g.goType(f.Type))
			if ok != "" {
				g.printf(inputVarLine, ok, goBool)
			}
			for i, name := range g.inputPatternVars(rec, f) {
				g.printf(inputPatternVarLine, name, g.use(regexpPkg, regexpPkg), strconv.Quote(f.Patterns[i].String()))
			}
		}
	}
	g.printf(inputVarLine, g.names.Inputs().Loaded, goBool)
	g.printf(closeParenFormat)
}

// loadInputsFunc writes LoadInputs, one field at a time (CODEGEN.md §5.12).
func (g *gen) loadInputsFunc(recs []*ir.Record) {
	g.temps = 0
	in := g.names.Inputs()
	g.inputErrs = in.Errs
	g.printf(inputsDocFormat, in.Func)
	g.printf(inputsOpenFormat, in.Func, g.inputErrs)
	for _, rec := range recs {
		for _, f := range rec.Fields {
			if f.Input != nil {
				g.inputField(rec, f)
			}
		}
	}
	g.printf(inputsCloseFormat, in.Loaded, g.inputErrs, g.use(errorsPkg, errorsPkg), g.inputErrs)
}

// inputField resets the field's slot, then writes its env guard (EVALUATION.md §11.3).
func (g *gen) inputField(rec *ir.Record, f *ir.Field) {
	defer g.enter(rec.QName() + dot + f.Name)()
	env := strconv.Quote(f.Input.Env)
	raw, present := g.temp(tempRaw), g.temp(tempOK)
	value, ok := g.inputVars(rec, f)
	g.body.WriteString(resetStmt(value, ok, g.inputZero(f.Type)))
	g.printf(inputEnvOpenFormat, raw, present, g.rt(), env, present)
	g.inputBody(rec, f, raw, env)
	if f.Optional {
		g.printf(inputOptionalClose)
		return
	}
	g.printf(elseLine)
	g.printf(inputErrFormat, g.inputErrs, g.rt(), env, strconv.Quote(g.inputReason(ir.InputNotSet, ir.TypeRef{})))
	g.printf(closeBrace)
}

// inputZero is the literal a field's slot resets to before every read.
func (g *gen) inputZero(t ir.TypeRef) string {
	switch t.Kind {
	case types.Bool:
		return strconv.FormatBool(false)
	case types.String:
		return emptyString
	default:
		return zeroLit
	}
}

// inputBody parses and checks one present input, by its type's kind (EVALUATION.md §11.3's table).
func (g *gen) inputBody(rec *ir.Record, f *ir.Field, raw, env string) {
	switch f.Type.Kind {
	case types.Bool:
		g.inputBoolBody(rec, f, raw, env)
	case types.Int:
		g.inputIntBody(rec, f, raw, env)
	case types.Float:
		g.inputFloatBody(rec, f, raw, env)
	case types.Duration:
		g.inputDurationBody(rec, f, raw, env)
	case types.String:
		g.inputStringBody(rec, f, raw, env)
	case types.Enum:
		g.inputEnumBody(rec, f, raw, env)
	default:
		g.failKind(f.Type.Kind)
	}
}

// writeInputLiteral writes a parse call, its checks, then the assignment (CODEGEN.md §6.3).
func (g *gen) writeInputLiteral(s inputSpec) {
	g.printf(inputParseFormat, s.parsed, s.errv, g.rt(), s.parseFn, s.raw)
	g.printf(inputSwitchOpen, s.errv)
	g.printf(inputErrFormat, g.inputErrs, g.rt(), s.env, strconv.Quote(g.inputReason(ir.InputNotValid, ir.TypeRef{Kind: s.kind})))
	for _, c := range s.checks {
		g.printf(caseFormat, c.cond)
		g.printf(inputErrFormat, g.inputErrs, g.rt(), s.env, strconv.Quote(c.reason))
	}
	g.printf(inputDefaultOpen)
	g.body.WriteString(assignStmt(s.value, s.ok, s.assign))
	g.printf(closeBrace)
}

func (g *gen) inputBoolBody(rec *ir.Record, f *ir.Field, raw, env string) {
	value, ok := g.inputVars(rec, f)
	parsed, errv := g.temp(tempValue), g.temp(localErr)
	g.writeInputLiteral(inputSpec{
		value: value, ok: ok, env: env, parsed: parsed, errv: errv, raw: raw,
		parseFn: parseBoolLiteral, kind: types.Bool, assign: parsed,
	})
}

func (g *gen) inputIntBody(rec *ir.Record, f *ir.Field, raw, env string) {
	value, ok := g.inputVars(rec, f)
	parsed, errv := g.temp(tempValue), g.temp(localErr)
	g.writeInputLiteral(inputSpec{
		value: value, ok: ok, env: env, parsed: parsed, errv: errv, raw: raw,
		parseFn: parseIntLiteral, kind: types.Int, checks: g.intChecks(f, parsed), assign: g.intOf(f.Type, parsed),
	})
}

// inputFloatBody dispatches Float32 to its own two-stage codegen (calls log W2: overflow
// checked, then rounded, then the rounded value's own refinement checked).
func (g *gen) inputFloatBody(rec *ir.Record, f *ir.Field, raw, env string) {
	if f.Type.Bits == float32Bits {
		g.inputFloat32Body(rec, f, raw, env)
		return
	}
	value, ok := g.inputVars(rec, f)
	parsed, errv := g.temp(tempValue), g.temp(localErr)
	var checks []inputCheck
	if c, has := g.floatRangeCheck(f.Range, parsed); has {
		checks = append(checks, c)
	}
	g.writeInputLiteral(inputSpec{
		value: value, ok: ok, env: env, parsed: parsed, errv: errv, raw: raw,
		parseFn: parseFloatLiteral, kind: types.Float, checks: checks, assign: parsed,
	})
}

// inputFloat32Body rounds after the overflow check, then checks the rounded value (EVALUATION.md §4.3).
func (g *gen) inputFloat32Body(rec *ir.Record, f *ir.Field, raw, env string) {
	value, ok := g.inputVars(rec, f)
	parsed, errv := g.temp(tempValue), g.temp(localErr)
	rounded := g.temp(tempValue)
	g.printf(inputParseFormat, parsed, errv, g.rt(), parseFloatLiteral, raw)
	g.printf(inputSwitchOpen, errv)
	g.printf(inputErrFormat, g.inputErrs, g.rt(), env, strconv.Quote(g.inputReason(ir.InputNotValid, ir.TypeRef{Kind: types.Float})))
	g.printf(caseFormat, g.rt()+float32Overflow+parsed+rparen)
	g.printf(inputErrFormat, g.inputErrs, g.rt(), env, strconv.Quote(g.inputReason(ir.InputOutsideRange, ir.TypeRef{})))
	g.printf(inputDefaultOpen)
	g.printf(inputRoundFormat, rounded, parsed)
	g.inputFloat32Range(f.Range, rounded, value, ok, env)
	g.printf(closeBrace)
}

// inputFloat32Range writes the rounded value's own range check, or a bare assignment when the
// field has no explicit range.
func (g *gen) inputFloat32Range(r *types.Bound, rounded, value, ok, env string) {
	check, has := g.floatRangeCheck(r, goFloat64+lparen+rounded+rparen)
	if !has {
		g.body.WriteString(assignStmt(value, ok, rounded))
		return
	}
	g.printf(ifOpenFormat, check.cond)
	g.printf(inputErrFormat, g.inputErrs, g.rt(), env, strconv.Quote(check.reason))
	g.printf(elseLine)
	g.body.WriteString(assignStmt(value, ok, rounded))
	g.printf(closeBrace)
}

func (g *gen) inputDurationBody(rec *ir.Record, f *ir.Field, raw, env string) {
	value, ok := g.inputVars(rec, f)
	parsed, errv := g.temp(tempValue), g.temp(localErr)
	g.writeInputLiteral(inputSpec{
		value: value, ok: ok, env: env, parsed: parsed, errv: errv, raw: raw,
		parseFn: parseDurationLiteral, kind: types.Duration, checks: g.durationChecks(f, parsed), assign: parsed,
	})
}

func (g *gen) inputStringBody(rec *ir.Record, f *ir.Field, raw, env string) {
	value, ok := g.inputVars(rec, f)
	parsed, errv := g.temp(tempValue), g.temp(localErr)
	var checks []inputCheck
	if c, has := g.lengthCheck(f, parsed); has {
		checks = append(checks, c)
	}
	checks = append(checks, g.patternChecks(rec, f, parsed)...)
	g.writeInputLiteral(inputSpec{
		value: value, ok: ok, env: env, parsed: parsed, errv: errv, raw: raw,
		parseFn: parseStringLiteral, kind: types.String, checks: checks, assign: parsed,
	})
}

// inputEnumBody refuses a retired member (EVALUATION.md §11.3).
func (g *gen) inputEnumBody(rec *ir.Record, f *ir.Field, raw, env string) {
	e, ok := f.Type.Named.(*ir.Enum)
	if !ok {
		g.failf(ErrMalformed, noEnumFormat, g.at)
		return
	}
	value, okVar := g.inputVars(rec, f)
	g.printf(switchTagFormat, raw)
	for _, m := range e.Members {
		if m.Retired {
			continue
		}
		g.printf(caseFormat, strconv.Quote(m.Wire))
		g.body.WriteString(assignStmt(value, okVar, g.qualify(e.Pkg, g.names.MemberName(e, m))))
	}
	g.printf(inputDefaultOpen)
	g.printf(inputErrFormat, g.inputErrs, g.rt(), env, strconv.Quote(g.inputReason(ir.InputNotMember, f.Type)))
	g.printf(closeBrace)
}

// intBoundCheck is an integer-valued input's own refinement range (EVALUATION.md §11.3).
func intBoundCheck(expr string, hasLo bool, lo int64, hasHi bool, hi int64) (string, bool) {
	var parts []string
	if hasLo {
		parts = append(parts, fmt.Sprintf(condFormat, expr, opText[ir.OpLt], intLit(lo)))
	}
	if hasHi {
		parts = append(parts, fmt.Sprintf(condFormat, expr, opText[ir.OpGt], intLit(hi)))
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, orSep), true
}

// intLit is n as a Go integer literal.
func intLit(n int64) string { return strconv.FormatInt(n, decimal) }

// intChecks is an integer input's own refinement range (EVALUATION.md §11.3).
func (g *gen) intChecks(f *ir.Field, expr string) []inputCheck {
	lo, hi := intBounds(f.Type)
	if r := f.Range; r != nil {
		if r.HasLo && r.Lo.I > lo {
			lo = r.Lo.I
		}
		if r.HasHi {
			h := r.Hi.I
			if !r.HiIncluded {
				h--
			}
			if h < hi {
				hi = h
			}
		}
	}
	if lo == math.MinInt64 && hi == math.MaxInt64 {
		return nil
	}
	cond, _ := intBoundCheck(expr, true, lo, true, hi)
	return []inputCheck{{cond: cond, reason: g.inputReason(ir.InputOutsideRange, ir.TypeRef{})}}
}

// floatRangeCheck is a Float or Float32 input's own explicit range.
func (g *gen) floatRangeCheck(r *types.Bound, expr string) (inputCheck, bool) {
	if r == nil {
		return inputCheck{}, false
	}
	var parts []string
	if r.HasLo {
		parts = append(parts, fmt.Sprintf(condFormat, expr, opText[ir.OpLt], g.floatLit(r.Lo.F, int64Bits)))
	}
	if r.HasHi {
		op := opText[ir.OpGe]
		if r.HiIncluded {
			op = opText[ir.OpGt]
		}
		parts = append(parts, fmt.Sprintf(condFormat, expr, op, g.floatLit(r.Hi.F, int64Bits)))
	}
	if len(parts) == 0 {
		return inputCheck{}, false
	}
	return inputCheck{cond: strings.Join(parts, orSep), reason: g.inputReason(ir.InputOutsideRange, ir.TypeRef{})}, true
}

// durationChecks is a Duration input's own explicit range, in milliseconds (EVALUATION.md §11.3).
func (g *gen) durationChecks(f *ir.Field, expr string) []inputCheck {
	r := f.Range
	if r == nil {
		return nil
	}
	ms := expr + durationMillis
	hi := r.Hi.I
	if r.HasHi && !r.HiIncluded {
		hi--
	}
	cond, has := intBoundCheck(ms, r.HasLo, r.Lo.I, r.HasHi, hi)
	if !has {
		return nil
	}
	return []inputCheck{{cond: cond, reason: g.inputReason(ir.InputOutsideRange, ir.TypeRef{})}}
}

// lengthCheck is a String input's own length-in-bytes refinement (EVALUATION.md §11.3).
func (g *gen) lengthCheck(f *ir.Field, expr string) (inputCheck, bool) {
	r := f.Range
	if r == nil {
		return inputCheck{}, false
	}
	n := lenCall + expr + rparen
	hi := r.Hi.I
	if r.HasHi && !r.HiIncluded {
		hi--
	}
	cond, has := intBoundCheck(n, r.HasLo, r.Lo.I, r.HasHi, hi)
	if !has {
		return inputCheck{}, false
	}
	return inputCheck{cond: cond, reason: g.inputReason(ir.InputOutsideRange, ir.TypeRef{})}, true
}

// patternChecks are a String input's own patterns, every one of its alias chain, innermost first (EVALUATION.md §11.3 step 3, TYPES.md §7.4).
func (g *gen) patternChecks(rec *ir.Record, f *ir.Field, expr string) []inputCheck {
	var out []inputCheck
	for _, name := range g.inputPatternVars(rec, f) {
		call := name + matchStringCall + expr + rparen
		out = append(out, inputCheck{cond: not + call, reason: g.inputReason(ir.InputNoMatch, ir.TypeRef{})})
	}
	return out
}
