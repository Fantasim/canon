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
	fn     *ir.ExportFn
	origin string
	name   string // the Go function or method
	store  string // the table: a struct member, or a package variable
	read   string // how the function reads the table: self.x, xTable or xTable()
	res    *slot  // how a cell is read, like a getter of the result type
	pair   bool   // cells are {v, ok}: an optional result nil cannot mark
	dims   []int
}

func (g *gen) newFinite(origin, name, store string, fn *ir.ExportFn) *finiteMethod {
	t, opt := unwrapOptional(fn.Result)
	f := &finiteMethod{
		fn: fn, origin: origin, name: name, store: store, read: selfDot + store,
		res: g.newSlot(origin, name, store, t, opt),
	}
	f.pair = opt && !strings.HasPrefix(g.cellType(f), pointer)
	for _, p := range fn.Params {
		f.dims = append(f.dims, g.domainSize(origin, p.Type))
	}
	return f
}

// domainSize is how many values a finite parameter has (CODEGEN.md §5.10, domain order).
func (g *gen) domainSize(origin string, t ir.TypeRef) int {
	switch {
	case t.Kind == types.Bool:
		return boolDomain
	case t.Kind == types.Enum:
		if e, ok := t.Named.(*ir.Enum); ok {
			return len(e.Members)
		}
	case t.Kind == types.Ref && isTableRef(t.Ref):
		for _, v := range g.p.Values {
			if v.Name == t.Ref.Value && t.Ref.Pkg == g.p.Name {
				return len(v.IDs)
			}
		}
	}
	g.failf(ErrUnsupported, "a parameter of %s that is not a Bool, an enum or a table of this package", origin)
	return 0
}

// cellType is what one cell holds: the result as a getter returns it, entries resolved.
func (g *gen) cellType(f *finiteMethod) string {
	if f.res.hasMain() {
		return g.mainType(f.res)
	}
	return g.slotKeyType(f.res)
}

func (g *gen) storageType(f *finiteMethod) string {
	var b strings.Builder
	for _, n := range f.dims {
		b.WriteString(lbracket + strconv.Itoa(n) + rbracket)
	}
	if f.pair {
		return b.String() + fmtPairCell(g.cellType(f))
	}
	return b.String() + g.cellType(f)
}

// cellArray is the table of a lookup as a Go expression: nested arrays, one level per
// parameter; a precomputed fn's single result is the cell itself. typed reports whether the
// expression already carries its own type (so a caller need not repeat it).
func (g *gen) cellArray(f *finiteMethod, t *ir.LookupTable) (lit string, typed bool) {
	want := 1
	for _, n := range f.dims {
		want *= n
	}
	if t == nil || len(t.Cells) != want || len(t.Domains) != len(f.dims) {
		g.failf(ErrMalformed, "the lookup table of %s does not have %d cells", f.origin, want)
		return nilLit, false
	}
	for i, d := range t.Domains {
		if len(d) != f.dims[i] {
			g.failf(ErrMalformed, "domain %d of %s has %d values, not %d", i, f.origin, len(d), f.dims[i])
		}
	}
	if len(f.dims) == 0 && !f.pair {
		return g.nest(f, t.Cells, 0), false
	}
	return g.storageType(f) + g.nest(f, t.Cells, 0), true
}

func (g *gen) nest(f *finiteMethod, cells []value.Value, dim int) string {
	switch {
	case dim == len(f.dims):
		return g.cell(f, cells[0])
	case f.dims[dim] == 0:
		return emptyBraces
	}
	stride := len(cells) / f.dims[dim]
	items := make([]string, f.dims[dim])
	for i := range items {
		items[i] = g.nest(f, cells[i*stride:(i+1)*stride], dim+1)
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
func (g *gen) writeFinite(sc *scope, prefix string, f *finiteMethod) {
	defer g.enter(f.origin)()
	g.fail(sc.add(f.name, f.origin))
	params := newScope(f.origin)
	g.fail(params.add(strings.TrimSuffix(strings.Split(f.read, dot)[0], callSuffix), f.origin))
	sig := make([]string, len(f.fn.Params))
	var prelude, index strings.Builder
	for i, p := range f.fn.Params {
		name := g.local(storageName(p.Name), f.origin)
		g.fail(params.add(name, f.origin))
		sig[i] = name + space + g.paramType(p.Type)
		index.WriteString(lbracket + g.paramIndex(&prelude, name, p.Type) + rbracket)
	}
	cell := f.read + index.String()
	body := returnKw + cell
	if f.pair {
		body = returnKw + cell + pairValue + listSep + cell + pairOK
	}
	g.body.WriteString(docFor(f.name, f.fn.Doc))
	result := results(g.cellType(f), f.pair)
	g.printf("%s%s(%s) %s {\n%s%s\n}\n\n", prefix, f.name, strings.Join(sig, listSep), result, prelude.String(), body)
}

func (g *gen) paramType(t ir.TypeRef) string {
	if t.Kind == types.Ref {
		return g.keyType(t)
	}
	return g.goType(t)
}

// paramIndex is the array index of a parameter: itself for an enum or a table id, else a
// local the prelude computes (a Bool, or an enum whose values are @codes).
func (g *gen) paramIndex(prelude *strings.Builder, name string, t ir.TypeRef) string {
	local := name + indexLocalSuffix
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

// fns writes the package-level export fns in declaration order (CODEGEN.md §5.10).
func (g *gen) fns() {
	for _, fn := range g.p.Fns {
		name := exportedName(fn.Go.Name, fn.Name)
		g.declare(name, fn.Name)
		switch fn.Kind {
		case ir.FnLookup:
			g.packageTable(g.newFinite(fn.Name, name, lowerCamel(fn.Name)+tableSuffix, fn), fn.Table)
		case ir.FnPrecomputed:
			f := g.newFinite(fn.Name, name, lowerCamel(fn.Name)+tableSuffix, fn)
			g.packageTable(f, &ir.LookupTable{Cells: []value.Value{fn.Value}})
		default:
			g.failf(ErrUnsupported, translatedFormat, fn.Name)
		}
	}
}

// packageTable is a lookup's table, built once by pointer when a cell reads d (decision 183).
func (g *gen) packageTable(f *finiteMethod, t *ir.LookupTable) {
	g.declare(f.store, f.origin)
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
			f.store, g.use(syncPkg, syncPkg), typ, g.data, g.last+valuesSuffix, lit)
		f.read += callSuffix
	case typed:
		g.printf("var %s = %s\n\n", f.store, lit)
	default:
		g.printf("var %s %s = %s\n\n", f.store, typ, lit)
	}
	g.writeFinite(newScope(f.name), funcKw, f)
}
