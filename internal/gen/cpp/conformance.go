package cppgen

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

var (
	//go:embed text/conformance_open.txt
	conformanceOpenText string
	//go:embed text/run_open.txt
	runOpenText string
	//go:embed text/run_close.txt
	runCloseText string
)

// vectorSet is one translated function's table of vectors and how the loop calls it.
type vectorSet struct {
	fn         *ir.ExportFn
	label      string // <T>.<fn> or <fn>, as failures name it
	call       string // the pure function
	structName string
	arrayName  string
	inputs     []input // receiver paths, then parameters
}

// input is one field of a vector: its Canon name, its C++ name, its type.
type input struct {
	name, field string
	t           ir.TypeRef
	optional    bool
}

// conformance is <last>_conformance.gen.cpp (CONFORMANCE.md §7, CPP-06, decision 192).
func (g *gen) conformance() []byte {
	sets := g.vectorSets()
	var body writer
	body.write(conformanceOpenText)
	g.showHelpers(&body, sets)
	for _, s := range sets {
		g.vectorTable(&body, s)
	}
	body.printf(runOpenText, g.upper)
	for _, s := range sets {
		g.vectorLoop(&body, s)
	}
	body.write(runCloseText)
	var w writer
	w.printf(markerFormat, g.p.Dir)
	w.printf(conformanceHeaderFormat, g.p.Name)
	w.linef(0, includeQuotedFormat, g.last+genHeaderSuffix)
	w.blank()
	includes(&w, body.String(), nil)
	g.namespaceBody(&w, g.emit.Namespace+scopeSep+conformanceNS, body.String())
	return w.bytes()
}

// vectorSets are the translated methods in declaration order, then the package-level fns.
func (g *gen) vectorSets() []vectorSet {
	sc := newScope(conformanceNS)
	for _, n := range []string{codeGlobal, captureFunc, showFunc} {
		g.fail(sc.add(n, conformanceNS))
	}
	var out []vectorSet
	for _, m := range g.methods {
		owner := g.className(m.class)
		label := m.class.canonName() + qnameSep + m.fn.Name
		out = append(out, g.vectorSet(sc, m.fn, label, owner, detailPrefix+g.pl.PureName(owner, m.fn)))
	}
	for _, fn := range g.pkgFns {
		out = append(out, g.vectorSet(sc, fn, fn.Name, "", g.pl.FnName(fn)))
	}
	return out
}

func (g *gen) vectorSet(sc *scope, fn *ir.ExportFn, label, owner, call string) vectorSet {
	base := owner + upperCamel(fn.Name)
	s := vectorSet{
		fn: fn, label: label, call: call,
		structName: base + vectorSuffix, arrayName: schemaPrefix + base,
	}
	for _, n := range []string{s.structName, s.arrayName} {
		g.fail(sc.add(n, label))
	}
	for _, r := range fn.Reads {
		s.inputs = append(s.inputs, input{name: r.Name, field: vectorField(r.Name), t: r.Type, optional: r.Optional})
	}
	for _, p := range fn.Params {
		s.inputs = append(s.inputs, input{name: p.Name, field: vectorField(p.Name), t: p.Type})
	}
	fields := newScope(s.structName)
	for _, in := range append(s.inputs, input{name: wantField, field: wantField}, input{name: codeField, field: codeField}) {
		g.fail(fields.add(in.field, label))
	}
	if len(fn.Vectors) == 0 {
		g.fail(fmt.Errorf("%w: translated fn %s without vectors", ErrMalformed, label))
	}
	return s
}

// vectorField is an input's field: escaped, and `_` added to the struct's own `want` and `code`.
func vectorField(name string) string {
	if name == wantField || name == codeField {
		return name + underscore
	}
	return verbatim(name)
}

// vectorType is a vector field's type: the pure type, a view for a String or a ref's string key.
func (g *gen) vectorType(t ir.TypeRef) string {
	switch {
	case t.Kind == types.String:
		return cppStringView
	case t.Kind == types.Ref && t.Key != nil:
		return g.vectorType(*t.Key)
	default:
		return g.pureType(t)
	}
}

func (g *gen) inputType(in input) string {
	if in.optional {
		return fmt.Sprintf(optionalFormat, g.vectorType(in.t))
	}
	return g.vectorType(in.t)
}

// vectorTable is the struct of one function's vectors and the constexpr array of them.
func (g *gen) vectorTable(w *writer, s vectorSet) {
	leave := g.enter(s.label)
	defer leave()
	w.printf(structOpenFormat, s.structName)
	for _, in := range s.inputs {
		w.linef(1, fieldDeclFormat, g.inputType(in), in.field)
	}
	w.linef(1, fieldDeclFormat, g.vectorType(s.fn.Result), wantField)
	w.linef(1, codeFieldLine)
	w.line(closeClass)
	w.blank()
	w.printf(arrayOpenFormat, s.structName, s.arrayName)
	for _, v := range s.fn.Vectors {
		in, want, code := g.vectorValues(s, v)
		w.linef(1, vectorRowFormat, strings.Join(in, listSep), want, code)
	}
	w.line(closeClass)
	w.blank()
}

// vectorValues are one vector's literals: the inputs, the result, the code.
func (g *gen) vectorValues(s vectorSet, v *ir.Vector) (in []string, want, code string) {
	if len(v.Recv)+len(v.Args) != len(s.inputs) {
		g.fail(fmt.Errorf("%w: a vector of %s does not match its signature", ErrMalformed, g.at))
		return nil, cppInvalid, cppInvalid
	}
	for i, x := range append(append([]value.Value(nil), v.Recv...), v.Args...) {
		in = append(in, g.vectorLit(s.inputs[i].t, x))
	}
	want = zeroLit(s.fn.Result)
	if v.Code == "" {
		want = g.vectorLit(s.fn.Result, v.Want)
	}
	return in, want, quote(string(v.Code))
}

// vectorLit is a literal of a vector field; a variant read is its case kind.
func (g *gen) vectorLit(t ir.TypeRef, v value.Value) string {
	if _, none := v.(*value.None); none || t.Kind != types.Variant {
		return g.literal(t, v)
	}
	variant, ok := t.Named.(*ir.Variant)
	index := -1
	switch x := v.(type) {
	case *value.CaseKind:
		index = x.Index
	case *value.Record:
		if c, isCase := x.T.(*types.CaseType); isCase {
			index = c.Index
		}
	}
	if !ok || index < 0 || index >= len(variant.Cases) {
		g.fail(fmt.Errorf("%w: variant read without its case at %s", ErrMalformed, g.at))
		return cppInvalid
	}
	return g.kindMember(variant, index)
}

// zeroLit is the placeholder result of a vector that expects an error.
func zeroLit(t ir.TypeRef) string {
	switch t.Kind {
	case types.Bool:
		return falseLit
	case types.Int, types.Duration:
		return zeroInt
	case types.Float:
		return zeroFloat
	case types.String:
		return quote("")
	case types.Ref:
		if t.Key != nil {
			return zeroLit(*t.Key)
		}
	default:
	}
	return initBraces
}

// vectorLoop runs one table: the code, then the value, floats bitwise (CONFORMANCE.md §5, §7.2).
func (g *gen) vectorLoop(w *writer, s vectorSet) {
	leave := g.enter(s.label)
	defer leave()
	args := make([]string, len(s.inputs))
	for i, in := range s.inputs {
		args[i] = vectorPrefix + in.field
	}
	w.linef(1, loopOpenFormat, s.structName, s.arrayName)
	w.linef(depthTwo, resetCodeLine)
	w.linef(depthTwo, gotFormat, g.pureResult(s.fn.Result), s.call, strings.Join(args, listSep))
	differs := fmt.Sprintf(differsFormat, gotVar, vectorPrefix+wantField)
	if s.fn.Result.Kind == types.Float {
		differs = fmt.Sprintf(memcmpFormat, gotVar, vectorPrefix+wantField, gotVar)
	}
	w.linef(depthTwo, failTestFormat, differs)
	w.lineAt(depthThree, g.failurePrint(s))
	w.linef(depthThree, failuresIncrement)
	w.linef(depthTwo, closeBrace)
	w.linef(1, closeBrace)
}

// failurePrint is `<package>: <T>.<fn>(<name>=<value>, …) = <got> [<code>], canon says <want> [<code>]`.
func (g *gen) failurePrint(s vectorSet) string {
	var names, args []string
	for _, in := range s.inputs {
		spec, arg := g.printArg(in.t, in.optional, vectorPrefix+in.field)
		names = append(names, in.name+equals+spec)
		args = append(args, arg)
	}
	gotSpec, gotArg := g.printArg(s.fn.Result, false, gotVar)
	wantSpec, wantArg := g.printArg(s.fn.Result, false, vectorPrefix+wantField)
	format := fmt.Sprintf(failureTextFormat, g.p.Name, s.label, strings.Join(names, listSep), gotSpec, wantSpec)
	args = append(args, gotArg, codeArgs(codeGlobal), wantArg, codeArgs(vectorPrefix+codeField))
	return wrapCall(append([]string{stderrArg, quote(format)}, args...))
}

// wrapCall is the fprintf call, its arguments packed into lines of at most callColumns
// columns, aligned under the first one; the format string stands on its own line.
func wrapCall(args []string) string {
	indent := strings.Repeat(space, depthThree*len(indentUnit)+len(fprintfOpen))
	head, rest := args[:formatArgs], args[formatArgs:]
	lines := []string{fprintfOpen + head[0] + comma, indent + head[1] + comma}
	cur := ""
	for i, a := range rest {
		if i == len(rest)-1 {
			a += callClose
		} else {
			a += comma
		}
		switch {
		case cur == "":
			cur = indent + a
		case len(cur)+len(space)+len(a) > callColumns:
			lines, cur = append(lines, cur), indent+a
		default:
			cur += space + a
		}
	}
	return strings.Join(append(lines, cur), newline)
}

// printArg is the printf conversion and argument of a value of type t; an optional input is
// printed through its Show helper.
func (g *gen) printArg(t ir.TypeRef, optional bool, v string) (spec, arg string) {
	if optional {
		return specString, fmt.Sprintf(showCallFormat, v)
	}
	switch t.Kind {
	case types.Int, types.Duration:
		return specInt, fmt.Sprintf(longLongFormat, v)
	case types.Float:
		return specFloat, fmt.Sprintf(doubleFormat, v)
	case types.Bool:
		return specString, fmt.Sprintf(boolTextFormat, v)
	case types.Enum, types.Variant:
		return specView, codeArgs(fmt.Sprintf(helperFormat, toNameFunc, v))
	case types.Ref:
		if t.Key != nil {
			return g.printArg(*t.Key, false, v)
		}
	default:
	}
	return specView, codeArgs(v)
}

// codeArgs are the `%.*s` arguments of a string or string_view.
func codeArgs(v string) string { return fmt.Sprintf(viewArgsFormat, v, v) }

// showHelpers writes one Show overload per type of an optional input, in first-use order:
// the value as printf would write it, or `none`.
func (g *gen) showHelpers(w *writer, sets []vectorSet) {
	seen := map[string]bool{}
	for _, s := range sets {
		for _, in := range s.inputs {
			typ := g.vectorType(in.t)
			if !in.optional || seen[typ] {
				continue
			}
			seen[typ] = true
			w.printf(g.showFormat(in.t), typ)
			w.blank()
		}
	}
}

func (g *gen) showFormat(t ir.TypeRef) string {
	switch t.Kind {
	case types.Int, types.Duration:
		return showIntFormat
	case types.Float:
		return showFloatFormat
	case types.Bool:
		return showBoolFormat
	case types.Enum, types.Variant:
		return showNameFormat
	default:
		return showStringFormat
	}
}
