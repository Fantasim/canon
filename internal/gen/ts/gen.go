package tsgen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// gen is one run of the generator over a package and one of its ts emits: the declarations are items, joined by blank lines.
type gen struct {
	p            *ir.Package
	e            *ir.Emit
	err          error
	at           string
	items        []string
	names        map[string]string // generated name → what declared it: the module's one scope (CODEGEN.md §3.5)
	typeImports  map[string]map[string]bool
	valueImports map[string]map[string]bool
	used         map[string]bool // helper units the code references
	emitted      []*ir.Value
	entries      map[*ir.Record]bool // records that are the rows of a table: they hold `id` and `retired`
	loose        map[*ir.Record]bool // rows also held as plain values: `id` and `retired` are optional (CODEGEN.md §5.4)
	rowSites     map[*ir.Record]int  // the table types holding a record as rows: its id is its table's id type only when one does
	variantOf    map[*ir.Case]*ir.Variant
	pures        []*pureFn // translated fns written, in declaration order: the conformance file tests them
	decoded      map[any]bool
	props        map[string]bool // the properties declared per interface
	instances    map[*ir.ExportFn]map[*value.Record]*ir.Instance
	temps        int
	decoderStart int // where the decoders begin among the items
}

// Generate is the TypeScript generator (ir.Generator): the module of one ts emit, and its conformance file when the package has a translated fn (CODEGEN.md §2.3, §8); p is narrowed by ir.CopyOf.
func Generate(p *ir.Package, e *ir.Emit) ([]ir.File, error) {
	if p == nil || e == nil || e.Target != ir.TargetTS {
		return nil, ErrTarget
	}
	if e.FileName == "" || p.Dir == "" || p.Name == "" {
		return nil, fmt.Errorf("%w: ts emit %s without a file name or a package directory", ErrMalformed, e.Out)
	}
	g := newGen(p, e)
	src := g.source()
	test := g.conformance()
	if g.err != nil {
		return nil, g.err
	}
	files := []ir.File{{Path: e.FileName, Content: []byte(src)}}
	if test != "" {
		files = append(files, ir.File{Path: last(p.Name) + conformanceSuffix, Content: []byte(test)})
	}
	return files, nil
}

// last is the last segment of a Canon package path: the stem of the conformance file.
func last(pkg string) string {
	return pkg[strings.LastIndex(pkg, dot)+1:]
}

func newGen(p *ir.Package, e *ir.Emit) *gen {
	g := &gen{
		p: p, e: e, at: p.Name, names: map[string]string{}, used: map[string]bool{},
		typeImports: map[string]map[string]bool{}, valueImports: map[string]map[string]bool{},
		entries: map[*ir.Record]bool{}, loose: map[*ir.Record]bool{}, rowSites: map[*ir.Record]int{}, variantOf: map[*ir.Case]*ir.Variant{}, decoded: map[any]bool{}, props: map[string]bool{},
		instances: map[*ir.ExportFn]map[*value.Record]*ir.Instance{},
	}
	for _, name := range ir.TSHelperNames() {
		g.names[name] = helperOrigin
	}
	g.selectValues()
	g.indexEntries()
	for _, t := range p.Types {
		if v, ok := t.(*ir.Variant); ok {
			for _, c := range v.Cases {
				g.variantOf[c] = v
			}
		}
	}
	return g
}

// fail keeps the first error: generation goes on, so every function stays linear.
func (g *gen) fail(err error) {
	if g.err == nil && err != nil {
		g.err = err
	}
}

func (g *gen) failf(sentinel error, format string, args ...any) {
	g.fail(fmt.Errorf("%w: "+format, append([]any{sentinel}, args...)...))
}

// enter makes origin the item being written until the returned func restores the previous one.
func (g *gen) enter(origin string) (leave func()) {
	prev := g.at
	g.at = origin
	return func() { g.at = prev }
}

// declare adds a name to the module's scope; two items with one name are a collision stage E reports as E8005 first (ir.PlanTSNames), so meeting one here is malformed IR.
func (g *gen) declare(name, origin string) string {
	if prev, ok := g.names[name]; ok && prev != origin {
		g.failf(errCollision, collisionFormat, prev, origin, name)
	}
	g.names[name] = origin
	return name
}

// helper marks a helper unit as used and returns its name (CODEGEN.md §8.2).
func (g *gen) helper(name string) string {
	g.used[name] = true
	return name
}

// use marks several helper units as used: a fixed code shape names them itself.
func (g *gen) use(names ...string) {
	for _, name := range names {
		g.used[name] = true
	}
}

// add appends a declaration item.
func (g *gen) add(text string) {
	if text != "" {
		g.items = append(g.items, strings.TrimSuffix(text, newline))
	}
}

// selectValues is the emit's values: `values` or every public value, none in types mode (CODEGEN.md §2.1, §2.2).
func (g *gen) selectValues() {
	if g.e.Mode == ir.ModeTypes {
		return
	}
	for _, v := range g.p.Values {
		if len(g.e.Values) == 0 || slices.Contains(g.e.Values, v.Name) {
			g.emitted = append(g.emitted, v)
		}
	}
}

// isData reports data mode; baked and embedded are one output (CODEGEN.md §2.1).
func (g *gen) isData() bool { return g.e.Mode == ir.ModeData }

func (g *gen) isTypes() bool { return g.e.Mode == ir.ModeTypes }

// source is the main file: marker, imports, the helper block and the declarations in CODEGEN.md §2.7's order.
func (g *gen) source() string {
	for _, section := range g.sections() {
		section()
	}
	helperUnits, decoderUnits := g.neededUnits()
	if len(decoderUnits) > 0 {
		g.items = slices.Insert(g.items, g.decoderStart, joinUnits(decoderUnits))
	}
	var b strings.Builder
	fmt.Fprintf(&b, markerFormat, g.p.Dir)
	for _, line := range g.importLines() {
		b.WriteString(line + newline)
	}
	b.WriteString(newline + g.helperSource(helperUnits) + newline)
	for _, item := range g.items {
		b.WriteString(newline + item + newline)
	}
	return b.String()
}

// sections are the writers of the declarations: constants, enums, kind enums, branch enums, id enums, records and variants, values, export fns, decoders.
func (g *gen) sections() []func() {
	switch g.e.Mode {
	case ir.ModeData:
		return []func(){g.constants, g.enums, g.kindEnums, g.branchEnums, g.idTypes, g.records, g.schemas, g.fns, g.decoders}
	case ir.ModeTypes:
		return []func(){g.constants, g.enums, g.kindEnums, g.branchEnums, g.records, g.fns, g.decoders}
	default:
		return []func(){g.constants, g.enums, g.kindEnums, g.branchEnums, g.idTypes, g.records, g.values, g.fns}
	}
}

// tableOf is the emitted table value whose rows are of record elem, nil when none: its id type exists only then.
func (g *gen) tableOf(elem ir.Type) *ir.Value {
	for _, v := range g.emitted {
		if v.Type.Kind == types.Table && v.Type.Elem != nil && v.Type.Elem.Named == elem {
			return v
		}
	}
	return nil
}
