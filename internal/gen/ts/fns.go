package tsgen

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// fnSite is an export fn with what owns it: a record or case for a method, nothing for a package fn.
type fnSite struct {
	fn    *ir.ExportFn
	rec   *ir.Record // a record's method
	cs    *ir.Case   // a case's method
	label string     // <T>.<fn>, or <fn> at package level: the Canon name failures print
}

// fnSites are the fns written as functions, in declaration order (ExportFn.Order, CONFORMANCE.md §7.2): translated methods and the package fns; a record's other methods are properties.
func (g *gen) fnSites() []fnSite {
	var out []fnSite
	for _, t := range g.p.Types {
		switch x := t.(type) {
		case *ir.Record:
			out = g.methodSites(out, x.Name, x.Methods, func(s *fnSite) { s.rec = x })
		case *ir.Variant:
			for _, c := range x.Cases {
				out = g.methodSites(out, x.Name+dot+c.Name, c.Methods, func(s *fnSite) { s.cs = c })
			}
		}
	}
	for _, fn := range g.p.Fns {
		out = append(out, fnSite{fn: fn, label: fn.Name})
	}
	slices.SortStableFunc(out, func(a, b fnSite) int { return cmp.Compare(a.fn.Order, b.fn.Order) })
	return out
}

// methodSites adds the translated methods of one owner.
func (g *gen) methodSites(out []fnSite, owner string, fns []*ir.ExportFn, set func(*fnSite)) []fnSite {
	for _, fn := range fns {
		if fn.Kind != ir.FnTranslated {
			continue
		}
		s := fnSite{fn: fn, label: owner + dot + fn.Name}
		set(&s)
		out = append(out, s)
	}
	return out
}

// fns writes the export fns: a package-level precomputed fn returns its value, a lookup indexes a dense table, a translated fn is a pure function and its public form (CODEGEN.md §5.10).
func (g *gen) fns() {
	for _, s := range g.fnSites() {
		g.fn(s)
	}
}

func (g *gen) fn(s fnSite) {
	defer g.enter(s.label)()
	switch s.fn.Kind {
	case ir.FnTranslated:
		g.translated(s)
	case ir.FnLookup:
		g.lookup(s.fn)
	default:
		g.precomputed(s.fn)
	}
}

// fnName is a package-level fn's name: lowerCamel(fn), or its override (CODEGEN.md §3.3).
func fnName(fn *ir.ExportFn) string { return escape(effective(fn.TS, lowerCamel(fn.Name))) }

// precomputed is `export function foo(): T { return value; }`; a composite value is frozen once, in a private constant.
func (g *gen) precomputed(fn *ir.ExportFn) {
	name := g.declare(fnName(fn), fn.Name)
	if fn.Value == nil {
		g.failf(ErrMalformed, malformedNoValue, fn.Name, g.at)
		return
	}
	typ, text := g.tsType(fn.Result, false), g.lit(fn.Result, fn.Value, false)
	doc := docComment("", fn.Doc)
	switch fn.Result.Kind {
	case types.Record, types.Variant, types.Case, types.List:
		table := g.declare(upperSnake(fn.Name), fn.Name)
		g.add(fmt.Sprintf(frozenConstFormat, table, typ, g.helper(canonFreezeName), text))
		g.add(doc + fmt.Sprintf(getterFormat, name, typ, table))
	default:
		g.add(doc + fmt.Sprintf(getterFormat, name, typ, text))
	}
}

// lookup is a function over a dense table of every answer, indexed by the parameters' ordinals, the first varying slowest (CODEGEN.md §5.10).
func (g *gen) lookup(fn *ir.ExportFn) {
	name := g.declare(fnName(fn), fn.Name)
	tab := fn.Table
	if tab == nil || len(tab.Domains) != len(fn.Params) {
		g.failf(ErrMalformed, malformedNoTable, fn.Name, g.at)
		return
	}
	table := g.declare(upperSnake(fn.Name), fn.Name)
	typ := g.tsType(fn.Result, false)
	cells := make([]string, len(tab.Cells))
	for i, c := range tab.Cells {
		cells[i] = g.lit(fn.Result, c, false)
	}
	g.add(fmt.Sprintf(tableConstFormat, table, typ, g.helper(canonFreezeName), g.cellLines(cells, tab, fn.Result)))
	params := make([]string, len(fn.Params))
	index := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		pn := escape(p.Name)
		params[i] = pn + keyValueSep + g.argType(p.Type)
		index[i] = g.indexTerm(p.Type, pn, tab.Domains[i+1:])
	}
	g.add(docComment("", fn.Doc) + fmt.Sprintf(lookupFormat, name, strings.Join(params, listSep), typ, table, strings.Join(index, plusSep)))
}

// cellLines lays the cells out: one per line when a cell is a composite value, else a line for each run of the innermost parameter, split in chunks of cellsPerLine.
func (g *gen) cellLines(cells []string, tab *ir.LookupTable, result ir.TypeRef) string {
	run := 1
	if n := len(tab.Domains); n > 0 && scalarResult(result) {
		run = min(max(len(tab.Domains[n-1]), 1), cellsPerLine)
	}
	var lines []string
	for start := 0; start < len(cells); start += run {
		lines = append(lines, indent+strings.Join(cells[start:min(start+run, len(cells))], listSep)+comma)
	}
	return strings.Join(lines, newline)
}

// indexTerm is a parameter's ordinal times the number of cells it strides over: Bool as `(x ? 1 : 0)`, an enum or a table's ids through their Index (CODEGEN.md §5.10).
func (g *gen) indexTerm(t ir.TypeRef, param string, later [][]value.Value) string {
	stride := 1
	for _, d := range later {
		stride *= len(d)
	}
	term := g.ordinal(t, param)
	if stride == 1 {
		return term
	}
	return fmt.Sprintf(strideFormat, term, stride)
}

// ordinal is the position of a parameter value in its domain.
func (g *gen) ordinal(t ir.TypeRef, param string) string {
	switch {
	case t.Kind == types.Bool:
		return fmt.Sprintf(boolOrdinalFormat, param)
	case t.Kind == types.Enum:
		return fmt.Sprintf(indexOfFormat, g.indexConst(t.Named), param)
	case t.Kind == types.Ref && isTableRef(t.Ref) && t.Ref.Elem != nil:
		return fmt.Sprintf(indexOfFormat, g.idIndexConst(t.Ref), param)
	}
	g.failf(ErrMalformed, malformedDomainType, g.at)
	return param
}

// indexConst is the Index table of an enum, imported when another package's.
func (g *gen) indexConst(e ir.Type) string {
	name := typeName(e) + indexSuffix
	if pkg := pkgOf(e); pkg != g.p.Name {
		return g.importValue(pkg, name)
	}
	return name
}

// idIndexConst is the Index table of a table's ids.
func (g *gen) idIndexConst(r *ir.RefTarget) string {
	name := idName(r.Elem) + indexSuffix
	if r.Pkg != g.p.Name {
		return g.importValue(r.Pkg, name)
	}
	return name
}

// scalarResult reports a result that fits many to a line: a scalar, an enum, a ref, or an optional of one.
func scalarResult(t ir.TypeRef) bool {
	if t.Kind == types.Optional && t.Elem != nil {
		return scalarResult(*t.Elem)
	}
	switch t.Kind {
	case types.List, types.Map, types.DepMap, types.Table, types.Record, types.Variant, types.Case:
		return false
	default:
		return true
	}
}

// argType is the type of a lookup function's parameter: a Bool is boolean, an enum or a ref its own type.
func (g *gen) argType(t ir.TypeRef) string {
	if t.Kind == types.Bool {
		return tsBoolean
	}
	return g.keyType(t, false)
}
