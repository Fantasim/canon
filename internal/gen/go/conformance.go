package gogen

import (
	"bytes"
	"cmp"
	"fmt"
	"go/format"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// headView and testView are the shapes of text/conformance.txt.
type headView struct {
	RT, Fmt string
}

type testView struct {
	Name, T, Testing, Result, Call, Math, Format, Shown string
	Fields, Rows                                        []string
}

// input is one field of a vector: its Canon name, its struct field, its type, its ok field.
type input struct {
	canon, field, ok string
	t                ir.TypeRef
}

// conformance is <gopkg>_conformance_test.go, one test per translated fn in declaration order (CONFORMANCE.md §7, decision 192), or nil without one. Its imports are its own.
func (g *gen) conformance() []byte {
	if len(g.pures) == 0 {
		return nil
	}
	imports, importOf := g.imports, g.importOf
	g.imports, g.importOf = map[string]string{}, map[string]string{}
	defer func() { g.imports, g.importOf = imports, importOf }()
	var tests bytes.Buffer
	show := false
	for _, p := range slices.SortedStableFunc(slices.Values(g.pures), byOrder) {
		leave := g.enter(p.label)
		view, optional := g.testView(p)
		show = show || optional
		g.execTo(&tests, testTemplate, view)
		leave()
	}
	head := headView{RT: g.rt()}
	if show {
		head.Fmt = g.use(fmtPkg, fmtPkg)
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, markerFormat, g.p.Dir)
	fmt.Fprintf(&out, conformanceDocFormat, g.p.Name, g.e.GoPackage)
	g.writeImports(&out)
	g.execTo(&out, conformanceTemplate, head)
	out.Write(tests.Bytes())
	src, err := format.Source(out.Bytes())
	if err != nil {
		g.fail(fmt.Errorf("%w: %w", errFormat, err))
		return src
	}
	g.fail(checkNames(src, false))
	return src
}

func byOrder(a, b *pure) int { return cmp.Compare(a.fn.Order, b.fn.Order) }

// testView is one fn's table of vectors and the loop calling its pure function (CONFORMANCE.md §7.2).
func (g *gen) testView(p *pure) (view testView, optional bool) {
	inputs := g.inputs(p)
	view = testView{
		Name: p.test, T: g.testLocal(), Testing: g.use(testingPkg, testingPkg),
		Result: g.pureType(p.fn.Result),
	}
	var args, names, shown []string
	for _, in := range inputs {
		view.Fields = append(view.Fields, in.field+space+g.pureType(in.t))
		args = append(args, vectorLocal+dot+in.field)
		spec, arg := verb(g, in.t), vectorLocal+dot+in.field
		if in.ok != "" {
			optional = true
			view.Fields = append(view.Fields, in.ok+space+goBool)
			args = append(args, vectorLocal+dot+in.ok)
			spec, arg = verbString, fmt.Sprintf(showFormat, vectorLocal+dot+in.field, vectorLocal+dot+in.ok)
		}
		names = append(names, in.canon+equals+spec)
		shown = append(shown, arg)
	}
	view.Call = p.name + lparen + strings.Join(args, listSep) + rparen
	if p.fn.Result.Kind == types.Float {
		view.Math = g.use(mathPkg, mathPkg)
	}
	res := verb(g, p.fn.Result)
	view.Format = strconv.Quote(fmt.Sprintf(failureFormat, p.label, strings.Join(names, listSep), res, res))
	view.Shown = strings.Join(shown, listSep)
	for _, v := range p.fn.Vectors {
		view.Rows = append(view.Rows, g.row(p, inputs, v))
	}
	return view, optional
}

// inputs are the reads of self then the parameters, named as the pure function names them; one named like a field of the template's gets `_` (CONFORMANCE.md §7.2).
func (g *gen) inputs(p *pure) []input {
	var out []input
	taken := map[string]bool{}
	field := func(name string) string {
		for taken[name] || vectorFields[name] {
			name += underscore
		}
		taken[name] = true
		return name
	}
	for i, r := range p.fn.Reads {
		in := input{canon: r.Name, field: field(p.names[r.Name]), t: r.Type}
		if p.oks[i] != "" {
			in.ok = field(p.oks[i])
		}
		out = append(out, in)
	}
	for _, prm := range p.fn.Params {
		out = append(out, input{canon: prm.Name, field: field(p.names[prm.Name]), t: prm.Type})
	}
	return out
}

// testLocal is the test's *testing.T, t unless an imported Canon package is called so.
func (g *gen) testLocal() string {
	name := testingLocal
	for g.taken[name] {
		name += underscore
	}
	return name
}

// verb is the printf verb of a value of type t in a failure message.
func verb(g *gen, t ir.TypeRef) string {
	switch t.Kind {
	case types.Int, types.Duration:
		return verbDecimal
	case types.String, types.LitUnion:
		return verbQuoted
	case types.Ref:
		if !isTableRef(t.Ref) && t.Key != nil {
			return verb(g, *t.Key)
		}
		if g.isData() {
			return verbQuoted
		}
	default:
	}
	return verbValue
}

// row is one vector: its inputs, the expected value (a zero placeholder when a code is), the code.
func (g *gen) row(p *pure, inputs []input, v *ir.Vector) string {
	vals := append(append([]value.Value(nil), v.Recv...), v.Args...)
	if len(vals) != len(inputs) {
		g.failf(ErrMalformed, "a vector of %s does not match its signature", g.at)
		return nilLit
	}
	var cells []string
	for i, in := range inputs {
		_, none := vals[i].(*value.None)
		switch {
		case in.ok != "" && none:
			cells = append(cells, g.zeroOf(in.t), strconv.FormatBool(false))
		case in.ok != "":
			cells = append(cells, g.vectorLit(in.t, vals[i]), strconv.FormatBool(true))
		default:
			cells = append(cells, g.vectorLit(in.t, vals[i]))
		}
	}
	want := g.zeroOf(p.fn.Result)
	if v.Code == "" {
		want = g.vectorLit(p.fn.Result, v.Want)
	}
	return lbrace + strings.Join(append(cells, want, strconv.Quote(string(v.Code))), listSep) + rbrace
}

// vectorLit is a vector's literal (CONFORMANCE.md §5, §7.2): int64 limits are math's, a float its ECMAScript text with `.0` when integral, -0.0 Copysign, a variant read its kind.
func (g *gen) vectorLit(t ir.TypeRef, v value.Value) string {
	switch x := v.(type) {
	case *value.Int:
		return g.int64Lit(x.V)
	case *value.Dur:
		return g.int64Lit(x.Ms)
	case *value.Float:
		return g.floatVector(x.V)
	case *value.CaseKind:
		return g.kindLit(t.Named, x.Index)
	case *value.Record:
		if c, ok := caseOf(x); ok && t.Kind == types.Variant {
			return g.kindLit(t.Named, c.Index)
		}
	case *value.Ref:
		return g.pureKey(t, x.Key)
	case nil, *value.None:
	default:
		return g.expr(t, v)
	}
	g.failf(ErrMalformed, "a vector value %T for a %s in %s", v, kindText(t.Kind), g.at)
	return nilLit
}

// caseOf is the case a variant value is.
func caseOf(r *value.Record) (*types.CaseType, bool) {
	if r.T == nil {
		return nil, false
	}
	c, ok := r.T.Base().(*types.CaseType)
	return c, ok
}

func (g *gen) int64Lit(n int64) string {
	switch n {
	case math.MinInt64:
		return g.use(mathPkg, mathPkg) + minInt64
	case math.MaxInt64:
		return g.use(mathPkg, mathPkg) + maxInt64
	}
	return intText(n)
}

// floatVector is ECMAScript's text of f, an integral one written with one decimal (CONFORMANCE.md §5).
func (g *gen) floatVector(f float64) string {
	if f == 0 && math.Signbit(f) {
		return g.use(mathPkg, mathPkg) + negativeZero
	}
	text := types.FloatText(f, int64Bits)
	if strings.IndexFunc(strings.TrimPrefix(text, minus), notDigit) < 0 {
		return strconv.FormatFloat(f, fixedFormat, 1, int64Bits)
	}
	return text
}

func notDigit(r rune) bool { return !unicode.IsDigit(r) }

// zeroOf is the placeholder of an absent value: false, "", 0.0 or 0.
func (g *gen) zeroOf(t ir.TypeRef) string {
	switch {
	case t.Kind == types.Bool:
		return strconv.FormatBool(false)
	case t.Kind == types.Float:
		return g.floatVector(0)
	case t.Kind == types.String, t.Kind == types.LitUnion:
		return emptyString
	case t.Kind == types.Ref && isTableRef(t.Ref) && g.isData():
		return emptyString
	case t.Kind == types.Ref && !isTableRef(t.Ref) && t.Key != nil:
		return g.zeroOf(*t.Key)
	}
	return zeroLit
}
