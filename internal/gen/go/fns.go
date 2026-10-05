package gogen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// finiteMethod is an export fn of finite parameters, read from a dense table (CODEGEN.md §5.10).
type finiteMethod struct {
	fn      *ir.ExportFn
	origin  string
	name    string   // the Go function or method
	store   string   // the table: a struct member, or a package variable
	read    string   // how the function reads the table: self.x, xTable or xTable()
	params  []string // the parameters' locals, escaped (decision 182)
	indexes []string // each parameter's index local, or "" (decision 122)
	res     *slot    // how a cell is read, like a getter of the result type
	pair    bool     // cells are {v, ok}: an optional result nil cannot mark
	dims    []int
	method  bool   // a record's or case's method, whose ref result also gets its key getter
	split   bool   // a make hook takes the pair cells as a values array and a presence array (CODEGEN.md §5.14)
	entry   string // the owner's entry getter of a ref result it resolves, which a reader checks (§2.8)
}

func (g *gen) newFinite(origin string, fn *ir.ExportFn) *finiteMethod {
	return g.newFiniteFrom(origin, fn, g.names.Finite(fn))
}

// newFiniteFrom is a finite method laid out as names says: this package's plan, or the owner's layout of another package's method (CODEGEN.md §5.14).
func (g *gen) newFiniteFrom(origin string, fn *ir.ExportFn, names ir.GoFinite) *finiteMethod {
	f := &finiteMethod{
		fn: fn, origin: origin, name: names.Name, store: names.Store, read: selfDot + names.Store,
		params: names.Params, indexes: names.Indexes, res: g.newSlot(origin, names.Result),
	}
	g.setPair(f)
	for i, p := range fn.Params {
		f.dims = append(f.dims, g.domainSize(origin, fn, i, p.Type))
	}
	return f
}

// setPair makes the cells {v, ok} pairs when the result is optional and nil cannot mark none.
func (g *gen) setPair(f *finiteMethod) {
	f.pair = f.res.Optional && !strings.HasPrefix(g.cellType(f), pointer)
}

// domainSize is how many values finite parameter i of fn has (CODEGEN.md §5.10, domain order): a table of another package's baked emit counts the domain its precomputed tables hold.
func (g *gen) domainSize(origin string, fn *ir.ExportFn, i int, t ir.TypeRef) int {
	switch {
	case t.Kind == types.Bool:
		return boolDomain
	case t.Kind == types.Enum:
		if e, ok := t.Named.(*ir.Enum); ok {
			return len(e.Members)
		}
	case t.Kind == types.Ref && isTableRef(t.Ref) && t.Ref.Pkg == g.p.Name:
		for _, v := range g.p.Values {
			if v.Name == t.Ref.Value {
				return len(v.IDs)
			}
		}
	case t.Kind == types.Ref && isTableRef(t.Ref) && g.enumIDs(t.Ref.Pkg):
		if d, ok := tableDomain(fn, i); ok {
			return len(d)
		}
	}
	// a lookup's parameters are finite (a Bool, an enum, a table ref), and a foreign table's without an id enum is E8019 ForeignTableLookupParam.
	g.failf(ErrMalformed, "a parameter of %s that is not a Bool, an enum or a table with an id enum", origin)
	return 0
}

// tableDomain is domain i of fn, stage E's whatever receivers exist; for a ref into a table with an id enum, its entries in entry order (CODEGEN.md §5.10; log-2026-10-06 "U5 review FAIL" 2).
func tableDomain(fn *ir.ExportFn, i int) ([]value.Value, bool) {
	if i >= len(fn.Domains) {
		return nil, false
	}
	return fn.Domains[i], true
}

// cellType is what one cell holds: the result as a getter returns it, entries resolved.
func (g *gen) cellType(f *finiteMethod) string {
	if f.res.hasMain() {
		return g.mainType(f.res)
	}
	return g.slotKeyType(f.res)
}

// keyStorageType is the table of a resolved result's keys, read from the file (CODEGEN.md §5.8).
func (g *gen) keyStorageType(f *finiteMethod) string {
	cell := g.slotKeyType(f.res)
	if f.res.Optional {
		cell = fmtPairCell(cell)
	}
	return g.dims(f) + cell
}

func (g *gen) dims(f *finiteMethod) string {
	var b strings.Builder
	for _, n := range f.dims {
		b.WriteString(lbracket + strconv.Itoa(n) + rbracket)
	}
	return b.String()
}

func (g *gen) storageType(f *finiteMethod) string {
	if f.pair {
		return g.dims(f) + fmtPairCell(g.cellType(f))
	}
	return g.dims(f) + g.cellType(f)
}

// cellArray is the table of a lookup as a Go expression: nested arrays, one level per
// parameter; a precomputed fn's single result is the cell itself. typed reports whether the
// expression already carries its own type (so a caller need not repeat it).
func (g *gen) cellArray(f *finiteMethod, t *ir.LookupTable) (lit string, typed bool) {
	if !g.checkTable(f, t) {
		return nilLit, false
	}
	cell := func(v value.Value) string { return g.cell(f, v) }
	if len(f.dims) == 0 && !f.pair {
		return g.nest(f, t.Cells, 0, cell), false
	}
	return g.storageType(f) + g.nest(f, t.Cells, 0, cell), true
}

// splitCells are a hook's arguments of a lookup's pair cells: the values, zero for none, then the presence flags (CODEGEN.md §5.14).
func (g *gen) splitCells(f *finiteMethod, t *ir.LookupTable) []string {
	if !g.checkTable(f, t) {
		return []string{nilLit, nilLit}
	}
	values := func(v value.Value) string {
		if _, none := v.(*value.None); none {
			return g.cellZero(f)
		}
		return g.cell(&finiteMethod{res: f.res}, v)
	}
	oks := func(v value.Value) string {
		_, none := v.(*value.None)
		return strconv.FormatBool(!none)
	}
	return []string{g.dims(f) + g.cellType(f) + g.nest(f, t.Cells, 0, values), g.dims(f) + goBool + g.nest(f, t.Cells, 0, oks)}
}

// cellZero is the zero a values array holds for none: an empty list, a zero key, a zero scalar.
func (g *gen) cellZero(f *finiteMethod) string {
	s := f.res
	switch {
	case s.List:
		return g.cellType(f) + emptyBraces
	case s.Ref != nil:
		return g.zeroKeyOf(s.refType())
	}
	return g.hookZero(s, member{s.Store, g.cellType(f)})
}

// checkTable refuses a lookup table whose cells or domains do not match f's dimensions.
func (g *gen) checkTable(f *finiteMethod, t *ir.LookupTable) bool {
	want := 1
	for _, n := range f.dims {
		want *= n
	}
	if t == nil || len(t.Cells) != want || len(t.Domains) != len(f.dims) {
		g.failf(ErrMalformed, "the lookup table of %s does not have %d cells", f.origin, want)
		return false
	}
	for i, d := range t.Domains {
		if len(d) != f.dims[i] {
			g.failf(ErrMalformed, "domain %d of %s has %d values, not %d", i, f.origin, len(d), f.dims[i])
		}
	}
	return true
}

func (g *gen) nest(f *finiteMethod, cells []value.Value, dim int, cell func(value.Value) string) string {
	switch {
	case dim == len(f.dims):
		return cell(cells[0])
	case f.dims[dim] == 0:
		return emptyBraces
	}
	stride := len(cells) / f.dims[dim]
	items := make([]string, f.dims[dim])
	for i := range items {
		items[i] = g.nest(f, cells[i*stride:(i+1)*stride], dim+1, cell)
	}
	if dim+1 < len(f.dims) {
		return lbrace + newline + strings.Join(items, listEnd) + listEnd + rbrace
	}
	return braced(elide(g.cellType(f), items))
}

// cell is one result: nil or {} for none, the value otherwise.
func (g *gen) cell(f *finiteMethod, v value.Value) string {
	if _, none := v.(*value.None); none {
		if f.pair {
			return emptyBraces
		}
		return nilLit
	}
	var x string
	if f.res.hasMain() {
		x = g.mainExpr(f.res, v)
	} else {
		x = g.keyExpr(f.res, v)
	}
	if f.pair {
		return fmtPairValue(x)
	}
	return x
}

// writeFinite writes the function or method that reads a finite table (CODEGEN.md §5.10).
func (g *gen) writeFinite(prefix string, f *finiteMethod) {
	defer g.enter(f.origin)()
	sig := make([]string, len(f.fn.Params))
	var prelude, index strings.Builder
	for i, p := range f.fn.Params {
		name := f.params[i]
		sig[i] = name + space + g.paramType(p.Type)
		index.WriteString(lbracket + g.paramIndex(&prelude, name, f.indexes[i], p.Type) + rbracket)
	}
	h := finiteHead{prefix: prefix, params: strings.Join(sig, listSep), prelude: prelude.String()}
	cell := f.read + index.String()
	if f.method && f.res.Ref != nil {
		g.writeRefFinite(h, f, cell, index.String())
		return
	}
	g.writeFiniteFunc(h, f.name, f.fn.Doc, results(g.cellType(f), f.pair), pairBody(cell, f.pair))
}

func (g *gen) paramType(t ir.TypeRef) string {
	if t.Kind == types.Ref {
		return g.keyType(t)
	}
	return g.goType(t)
}

// paramIndex is the array index of a parameter: itself for an enum or a table id, else the
// plan's local the prelude computes (a Bool, or an enum whose values are @codes).
func (g *gen) paramIndex(prelude *strings.Builder, name, local string, t ir.TypeRef) string {
	switch {
	case t.Kind == types.Bool:
		prelude.WriteString(fmtBoolIndex(local, name))
	case t.Kind == types.Enum && isCodes(t.Named):
		prelude.WriteString(g.codesIndex(local, name, t))
	default:
		return name
	}
	return local
}

func fmtBoolIndex(local, name string) string { return fmt.Sprintf(boolIndexFormat, local, name) }

func fmtPairCell(t string) string { return fmt.Sprintf(pairCellFormat, t) }

func fmtPairValue(x string) string { return fmt.Sprintf(pairValueFormat, x) }

func isCodes(t ir.Type) bool {
	e, ok := t.(*ir.Enum)
	return ok && e.Codes != nil
}

// codesIndex switches a @codes enum onto its declaration index; another value indexes -1.
func (g *gen) codesIndex(local, name string, t ir.TypeRef) string {
	e, _ := t.Named.(*ir.Enum)
	var b strings.Builder
	b.WriteString(local + declareMinusOne + switchKw + name + openBlock)
	for i := range e.Members {
		fmt.Fprintf(&b, codesCaseFormat, g.memberLit(t, i), local, i)
	}
	return b.String() + rbrace + newline
}

// fns writes the package-level export fns in declaration order; data mode has translated ones only (CODEGEN.md §5.10).
func (g *gen) fns() {
	for _, fn := range g.p.Fns {
		if g.isData() && fn.Kind != ir.FnTranslated {
			g.fail(newDetail(ErrMalformed, fn.Name, packageFnFormat, fn.Name)) // E8013 package
			continue
		}
		switch fn.Kind {
		case ir.FnLookup:
			g.packageTable(g.newFinite(fn.Name, fn), fn.Table)
		case ir.FnPrecomputed:
			g.packageTable(g.newFinite(fn.Name, fn), &ir.LookupTable{Cells: []value.Value{fn.Value}})
		default:
			g.translatedFn(fn)
		}
	}
}

// packageTable is a lookup's table, built once by pointer when a cell reads d (decision 183).
func (g *gen) packageTable(f *finiteMethod, t *ir.LookupTable) {
	g.usedData = false
	lit, typed := g.cellArray(f, t)
	typ := g.storageType(f)
	f.read = f.store
	switch {
	case g.usedData:
		if len(f.dims) > 0 {
			typ, lit = pointer+typ, ampersand+lit
		}
		g.printf("var %s = %s.OnceValue(func() %s {\n%s := %s()\nreturn %s\n})\n\n",
			f.store, g.use(syncPkg, syncPkg), typ, g.data, g.names.Data().Values, lit)
		f.read += callSuffix
	case typed:
		g.printf("var %s = %s\n\n", f.store, lit)
	default:
		g.printf("var %s %s = %s\n\n", f.store, typ, lit)
	}
	g.writeFinite(funcKw, f)
}
