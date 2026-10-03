package tsgen

import (
	_ "embed"
	"fmt"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

//go:embed text/canon_catch.ts.txt
var canonCatchText string

// conformance is <last>.conformance.test.ts: one test per translated fn in declaration order, each a table of vectors and a loop calling the pure function (CONFORMANCE.md §7); "" without a translated fn.
func (g *gen) conformance() string {
	if len(g.pures) == 0 {
		return ""
	}
	names := map[string]bool{canonEvalErrorName: true}
	for _, p := range g.pures {
		names[p.pure] = true
	}
	var tests []string
	for _, p := range g.pures {
		tests = append(tests, g.conformanceTest(p, names))
	}
	var b strings.Builder
	fmt.Fprintf(&b, markerFormat, g.p.Dir)
	fmt.Fprintf(&b, conformanceDocFormat, g.p.Name)
	b.WriteString(importNodeTest + newline + importNodeAssert + newline)
	fmt.Fprintf(&b, importPureFormat, strings.Join(slices.Sorted(maps.Keys(names)), listSep), "./"+strings.TrimSuffix(g.e.FileName, path.Ext(g.e.FileName))+jsExt)
	b.WriteString(newline + canonCatchText)
	for _, t := range tests {
		b.WriteString(newline + t)
	}
	return b.String()
}

// conformanceTest is one fn's test (CONFORMANCE.md §7.2); taken are the names the loop variables must not shadow.
func (g *gen) conformanceTest(p *pureFn, taken map[string]bool) string {
	defer g.enter(p.site.label)()
	locals := loopNames(p.inputs, taken)
	types := g.tupleTypes(p)
	rows := make([]string, len(p.site.fn.Vectors))
	for i, v := range p.site.fn.Vectors {
		rows[i] = indent + indent + g.vectorRow(p, v)
	}
	shown := make([]string, len(locals))
	for i, in := range p.inputs {
		shown[i] = fmt.Sprintf(shownFormat, in.canon, locals[i])
	}
	label := p.site.label
	call := fmt.Sprintf(callFormat, p.pure, strings.Join(locals, listSep))
	args := strings.Join(shown, listSep)
	return fmt.Sprintf(testFormat, label, strings.Join(types, listSep), strings.Join(rows, newline),
		strings.Join(append(locals, vectorWant, vectorCode), listSep), call, fmt.Sprintf(messageFormat, label, args), fmt.Sprintf(failureFormat, label, args))
}

// loopNames are the loop variables of a test: the inputs' names, suffixed with `_` while they would shadow another name of the file.
func loopNames(inputs []input, taken map[string]bool) []string {
	used := maps.Clone(taken)
	for _, k := range reservedLoop {
		used[k] = true
	}
	out := make([]string, len(inputs))
	for i, in := range inputs {
		name := in.name
		for used[name] {
			name += underscore
		}
		used[name], out[i] = true, name
	}
	return out
}

// tupleTypes are the element types of a vector: the inputs, the expected value, the expected code; a primitive is spelled out, anything else is read off the function's own signature.
func (g *gen) tupleTypes(p *pureFn) []string {
	out := make([]string, 0, len(p.inputs)+vectorTail)
	for i, in := range p.inputs {
		if prim, ok := primitive(in.t, in.optional); ok {
			out = append(out, prim)
			continue
		}
		out = append(out, fmt.Sprintf(paramTypeFormat, p.pure, i))
	}
	if prim, ok := primitive(p.site.fn.Result, false); ok {
		out = append(out, prim)
	} else {
		out = append(out, fmt.Sprintf(returnTypeFormat, p.pure))
	}
	return append(out, tsString)
}

// primitive is the TypeScript type of a number, boolean or string input.
func primitive(t ir.TypeRef, optional bool) (string, bool) {
	s, ok := scalarTypes[t.Kind]
	if t.Kind == types.Int {
		s, ok = tsNumber, true
	}
	if ok && optional {
		s += unionSep + tsNull
	}
	return s, ok
}

// vectorRow is `[inputs..., want, code]`: the TS expectation of the vector (CONFORMANCE.md §4).
func (g *gen) vectorRow(p *pureFn, v *ir.Vector) string {
	vals := append(slices.Clone(v.Recv), v.Args...)
	if len(vals) != len(p.inputs) {
		g.failf(ErrMalformed, malformedVector, g.at)
		return tsUndefined
	}
	cells := make([]string, 0, len(vals)+vectorTail)
	for i, in := range p.inputs {
		cells = append(cells, g.vectorLit(in.t, vals[i]))
	}
	want := g.placeholder(p.site.fn.Result)
	if v.TSCode == "" {
		want = g.vectorLit(p.site.fn.Result, v.TSWant)
	}
	return lbracket + strings.Join(append(cells, want, quote(string(v.TSCode))), listSep) + rbracket + comma
}

// vectorLit is a vector value (CONFORMANCE.md §4, §5, §7.2): an integer outside the safe range as its nearest double, a float in ECMAScript form, a variant read as its kind.
func (g *gen) vectorLit(t ir.TypeRef, v value.Value) string {
	switch x := v.(type) {
	case *value.Int:
		return intText(x.V)
	case *value.Dur:
		return intText(x.Ms)
	case *value.CaseKind:
		return quote(g.caseWire(t.Named, x.Index))
	case *value.Record:
		if c, ok := caseOf(x); ok && t.Kind == types.Variant {
			return quote(g.caseWire(t.Named, c.Index))
		}
	case *value.None:
		return tsNull
	case nil:
	default:
		return g.lit(t, v, false)
	}
	g.failf(ErrMalformed, malformedVectorValue, v, kindText(t.Kind), g.at)
	return tsUndefined
}

// caseOf is the case a variant value is.
func caseOf(r *value.Record) (*types.CaseType, bool) {
	if r.T == nil {
		return nil, false
	}
	c, ok := r.T.Base().(*types.CaseType)
	return c, ok
}

// placeholder is the value a row holds when an error code is expected: false, "", 0 or undefined.
func (g *gen) placeholder(t ir.TypeRef) string {
	switch t.Kind {
	case types.Bool:
		return strconv.FormatBool(false)
	case types.String, types.LitUnion:
		return emptyString
	case types.Int, types.Float, types.Duration:
		return zeroLit
	default:
		return tsUndefined
	}
}
