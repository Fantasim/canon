package ir_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"math"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	planFile          = "plan.txt"
	conformanceSuffix = "_conformance_test.go"
)

// vectorCode is a placeholder: a vector expecting a code has no value to write.
var vectorCode = diag.E3201.Def().Code

// TestGoNamePlanMatchesGeneratedModes is decisions 194 and 203 (CODEGEN.md §3.5): for each case, the plan of every go emit, in baked or data mode and with translated fns, holds exactly the names gen/go writes in its main file, scope by scope, and in its conformance file the package-level names and the imports; the plan's scopes and problems are the golden.
func TestGoNamePlanMatchesGeneratedModes(t *testing.T) {
	golden.Run(t, "testdata/goplan/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		w := newWorld(t)
		for _, f := range c.Archive.Files {
			if f.Name != planFile {
				w.add(t, f.Name, f.Data)
			}
		}
		w.calls = w.fixtureCalls
		var b strings.Builder
		for _, p := range w.build(t) {
			for _, e := range p.Emits {
				if e.Target == ir.TargetGo {
					fakeVectors(p)
					b.WriteString(dumpPlan(ir.PlanGoNames(p, e)))
					compareModes(t, p, e)
				}
			}
		}
		b.WriteString(w.findings(t))
		return []byte(b.String())
	}, golden.Expected(planFile))
}

// compareModes checks that every struct gen/go declares has the plan's names, the package scope too, and that the conformance file's package-level names are planned and its imports are the plan's.
func compareModes(t *testing.T, p *ir.Package, e *ir.Emit) {
	t.Helper()
	files, err := gogen.Generate(p, e)
	if err != nil {
		t.Fatalf("%s: %v", p.Name, err) // the golden's findings say why
	}
	want := ir.GoScopeNames(ir.PlanGoNames(p, e))
	got := declaredNames(t, goFile(t, files))
	imports := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f.Path, conformanceSuffix) {
			var names map[string]bool
			names, imports = conformanceNames(t, f.Content)
			maps.Copy(got[""], names)
		}
	}
	sameNames(t, p.Name+" package", got[""], want["package"])
	sameNames(t, p.Name+" conformance imports", imports, want["conformance imports"])
	for _, scope := range slices.Sorted(maps.Keys(got)) {
		if scope != "" {
			sameNames(t, p.Name+" "+scope, got[scope], want[scope])
		}
	}
}

// conformanceNames are a conformance file's package-level functions and its imports.
func conformanceNames(t *testing.T, src []byte) (names, imports map[string]bool) {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	names, imports = map[string]bool{}, map[string]bool{}
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		name := path.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = true
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			names[fd.Name.Name] = true
		}
	}
	return names, imports
}

// dumpPlan prints a Go plan's scopes and problems.
func dumpPlan(pl *ir.GoNamePlan) string { return dumpScopes(ir.GoScopeNames(pl), pl.Problems()) }

// fakeVectors gives each translated fn with a body one vector expecting a code, its inputs a placeholder of their type: the conform harness, not stage E, computes the real ones.
func fakeVectors(p *ir.Package) {
	each := func(fns []*ir.ExportFn) {
		for _, fn := range fns {
			if fn.Kind != ir.FnTranslated || fn.Body == nil {
				continue
			}
			v := &ir.Vector{Code: vectorCode}
			for _, r := range fn.Reads {
				v.Recv = append(v.Recv, placeholder(r.Type, r.Optional))
			}
			for _, prm := range fn.Params {
				v.Args = append(v.Args, placeholder(prm.Type, false))
			}
			fn.Vectors = []*ir.Vector{v}
		}
	}
	each(p.Fns)
	for _, ty := range p.Types {
		switch x := ty.(type) {
		case *ir.Record:
			each(x.Methods)
		case *ir.Variant:
			for _, c := range x.Cases {
				each(c.Methods)
			}
		}
	}
}

// placeholder is a value of kind t's, none for an optional one; an Int is its upper limit and a Float -0.0, candidates every Int and Float parameter has (CONFORMANCE.md §6.2).
func placeholder(t ir.TypeRef, optional bool) value.Value {
	switch {
	case optional:
		return &value.None{}
	case t.Kind == types.Bool:
		return &value.Bool{}
	case t.Kind == types.Float:
		return &value.Float{V: math.Copysign(0, -1)}
	case t.Kind == types.Duration:
		return &value.Dur{Ms: 1}
	case t.Kind == types.String:
		return &value.Str{}
	case t.Kind == types.Enum:
		return &value.Member{}
	case t.Kind == types.Variant:
		return &value.CaseKind{}
	}
	return &value.Int{V: math.MaxInt64}
}
