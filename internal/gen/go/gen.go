package gogen

import (
	"bytes"
	"fmt"
	"go/format"
	"go/token"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// gen is one run of the generator over a package and one of its go emits; every Go name it writes comes from names, the plan stage E checks too (decision 194).
type gen struct {
	p         *ir.Package
	e         *ir.Emit
	names     *ir.GoNamePlan
	body      bytes.Buffer
	imports   map[string]string // import path → the name code uses
	importOf  map[string]string // the name code uses → its import path
	err       error
	emitted   []*ir.Value
	byValue   map[string]*valueInfo
	tableOf   map[*ir.Record]*ir.Value
	instances map[*ir.ExportFn]map[*value.Record]*ir.Instance
	usedData  bool   // an expression read the baked data (d.…) since the flag was cleared
	data      string // the local that holds the baked data: d, escaped (§3.4)
	at        string // the Canon item being written: the Subject of a kind refusal
	bodies    map[any]*body
	variantOf map[*ir.Case]*ir.Variant
	lc        locals          // data mode: the escaped locals of loaders, decoders and resolvers
	inputErrs string          // LoadInputs' escaped local of failures (CODEGEN.md §5.12)
	taken     map[string]bool // the Go names of the imported Canon packages, which locals avoid
	temps     int             // the last numbered local of the function being written
	walkClass any             // the class whose resolver is being written
	walkSnap  bool            // that resolver finds entries in the snapshot, else its holder
	pures     []*pure         // the translated fns written, which the conformance file tests
	pkgNames  map[string]bool // package-level names a translated fn's locals avoid, built once
	rtPkg     string          // the Canon package whose rt types the code being written holds: "" for this one (CODEGEN.md §2.8)
	called    map[string]bool // the loader helpers the decoders written so far call
}

// Generate is the Go generator (ir.Generator), in baked, data and types mode: <gopkg>.gen.go, rt/rt.go verbatim, and <gopkg>_conformance_test.go when the package has a translated fn (§2.3, §6.3); p is narrowed by ir.CopyOf.
func Generate(p *ir.Package, e *ir.Emit) ([]ir.File, error) {
	if e.Target != ir.TargetGo {
		return nil, fmt.Errorf("%w: %s", ErrTarget, e.Out)
	}
	if e.Mode != ir.ModeBaked && e.Mode != ir.ModeData && e.Mode != ir.ModeTypes {
		// stage E refuses the other modes first: embedded is E8019 `unbuilt` (DECISIONS 320), none is check's E8009.
		return nil, fmt.Errorf("%w: mode %s of %s", ErrMalformed, modeText(e.Mode), e.Out)
	}
	if !token.IsIdentifier(e.GoPackage) || e.GoImport == "" {
		return nil, fmt.Errorf("%w: go emit %s without a package or an import path", ErrMalformed, e.Out)
	}
	g := newGen(p, e)
	src := g.source()
	test := g.conformance()
	if g.err != nil {
		return nil, g.err
	}
	files := []ir.File{
		{Path: rtPath, Content: []byte(rtText)},
		{Path: e.GoPackage + genSuffix, Content: src},
	}
	if test != nil {
		files = append(files, ir.File{Path: e.GoPackage + conformanceSuffix, Content: test})
	}
	return files, nil
}

func newGen(p *ir.Package, e *ir.Emit) *gen {
	g := &gen{
		p: p, e: e, names: ir.PlanGoNames(p, e), imports: map[string]string{}, importOf: map[string]string{}, at: p.Name,
		byValue: map[string]*valueInfo{}, tableOf: map[*ir.Record]*ir.Value{}, bodies: map[any]*body{}, called: map[string]bool{},
	}
	g.data = g.names.Data().Local
	g.taken = importedNames(p)
	if problems := g.names.Problems(); len(problems) > 0 {
		g.fail(nameError(problems[0]))
	}
	g.indexValues()
	g.indexInstances()
	if g.isData() {
		g.indexVariants()
		g.lc = g.newLocals()
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

// printf writes generated text; go/format lays it out afterwards.
func (g *gen) printf(format string, args ...any) {
	fmt.Fprintf(&g.body, format, args...)
}

// source is the formatted main file: every section in CODEGEN.md §2.7's order.
func (g *gen) source() []byte {
	sections := []func(){g.constants, g.enums, g.kindEnums, g.branchEnums, g.idEnums, g.types, g.rowTypes, g.hooks, g.defineTables, g.containers, g.values, g.fns, g.runtimeInputs}
	switch {
	case g.isTypes():
		sections = g.typesSections()
	case g.isData():
		sections = g.dataSections()
	}
	for _, section := range sections {
		section()
	}
	var out bytes.Buffer
	g.header(&out)
	out.Write(g.body.Bytes())
	src, err := format.Source(out.Bytes())
	if err != nil {
		g.fail(fmt.Errorf("%w: %w", errFormat, err))
		return src
	}
	g.fail(checkNames(src, g.isData()))
	return src
}

// header is the marker, the one-line package doc, the clause and the imports (decision 192).
func (g *gen) header(out *bytes.Buffer) {
	fmt.Fprintf(out, markerFormat, g.p.Dir)
	fmt.Fprintf(out, packageDocFormat, g.e.GoPackage, g.p.Name)
	fmt.Fprintf(out, "package %s\n\n", g.e.GoPackage)
	g.writeImports(out)
}

// use records an import and returns the name code refers to it by; the plan declared it, so two paths under one name are a plan defect.
func (g *gen) use(importPath, name string) string {
	if prev, ok := g.importOf[name]; ok && prev != importPath {
		g.failf(errNameCollision, "the imports %s and %s are both %s", prev, importPath, name)
	}
	g.imports[importPath], g.importOf[name] = name, importPath
	return name
}

// rt is the rt package of the code being written: this package's, or under withRT another package's, imported as <gopkg>rt when the plan imports it (CODEGEN.md §2.8, §5.14).
func (g *gen) rt() string {
	if g.rtPkg == "" || g.rtPkg == g.p.Name {
		return g.ownRT()
	}
	for _, imp := range g.names.RTImports() {
		if imp.Pkg == g.rtPkg {
			return g.use(imp.Path, imp.Name)
		}
	}
	g.failf(ErrMalformed, "%s holds an rt type of %s, which the plan does not import", g.at, g.rtPkg)
	return g.ownRT()
}

// ownRT is this package's rt: the helpers a loader calls, whatever class it reads.
func (g *gen) ownRT() string {
	return g.use(g.e.GoImport+rtDir, rtName)
}

// withRT makes pkg's rt the current one until the returned func restores the previous one.
func (g *gen) withRT(pkg string) (restore func()) {
	prev := g.rtPkg
	g.rtPkg = pkg
	return func() { g.rtPkg = prev }
}

// writeImports groups the standard library, then the others, each sorted (CODEGEN.md §2.8).
func (g *gen) writeImports(out *bytes.Buffer) {
	var std, other []string
	for _, p := range sortedKeys(g.imports) {
		line := g.importLine(p)
		if goStdImports[p] {
			std = append(std, line)
		} else {
			other = append(other, line)
		}
	}
	if len(std)+len(other) == 0 {
		return
	}
	groups := slices.DeleteFunc([]string{strings.Join(std, ""), strings.Join(other, "")}, isEmpty)
	fmt.Fprintf(out, "import (\n%s)\n\n", strings.Join(groups, newline))
}

// importLine names the import when its name is not the last element of its path.
func (g *gen) importLine(p string) string {
	if name := g.imports[p]; name != path.Base(p) {
		return fmt.Sprintf("%s %q\n", name, p)
	}
	return fmt.Sprintf("%q\n", p)
}

func isEmpty(s string) bool { return s == "" }

// commentLines is doc text as Go comment lines (CODEGEN.md §2.6).
func commentLines(doc string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(doc, newline) {
		if line == "" {
			b.WriteString(commentEmpty + newline)
			continue
		}
		b.WriteString(commentPrefix + line + newline)
	}
	return b.String()
}

// docFor is the doc comment of a generated item: its first line starts with its Go name.
func docFor(goName, doc string) string {
	if doc == "" {
		return ""
	}
	if !strings.HasPrefix(doc, goName+space) {
		doc = goName + keyValueSep + doc
	}
	return commentLines(doc)
}
