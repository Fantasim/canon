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

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const goSuffix = ".gen.go"

// TestGoNamePlanMatchesGeneratedGo is decision 194: E8005 checks exactly the names gen/go declares, so the plan of every teamboard package's go emit holds the names of the Go file gen/go writes, scope by scope (the package, each struct's fields and methods).
func TestGoNamePlanMatchesGeneratedGo(t *testing.T) {
	_, pkgs := teamboard(t)
	compared := 0
	for _, p := range pkgs {
		for _, e := range p.Emits {
			if e.Target != ir.TargetGo {
				continue
			}
			comparePlan(t, p.Name, p, e)
			compared++
		}
	}
	if compared == 0 {
		t.Fatal("no go emit compared")
	}
}

// TestGoNamePlanMatchesCells is decisions 202 and 122: a -0.0 cell of a package fn's table makes gen/go write math.Copysign and the plan declare math; the table variable is the plan's (index locals: TestGoPlanIndexLocals); a list constant's -0.0 counts too.
func TestGoNamePlanMatchesCells(t *testing.T) {
	floatT := ir.TypeRef{Kind: types.Float, Bits: 64}
	negZero := &value.Float{V: math.Copysign(0, -1), T: types.FloatType}
	one := &value.Float{V: 1, T: types.FloatType}
	cells := func(a, b value.Value) *ir.Package {
		fn := &ir.ExportFn{
			Name: "weight", Kind: ir.FnLookup, Result: floatT,
			Params: []*ir.Param{{Name: "on", Type: ir.TypeRef{Kind: types.Bool}}},
			Table:  &ir.LookupTable{Domains: [][]value.Value{{&value.Bool{}, &value.Bool{V: true}}}, Cells: []value.Value{a, b}},
		}
		return goPackage(&ir.Package{Fns: []*ir.ExportFn{fn}})
	}
	list := goPackage(&ir.Package{Consts: []*ir.Const{{
		Name: "weights", Type: ir.TypeRef{Kind: types.List, Elem: &floatT},
		V: &value.List{T: &types.ListType{Elem: types.FloatType}, Elems: []value.Value{negZero}},
	}}})
	for name, p := range map[string]*ir.Package{"-0.0 cell": cells(negZero, one), "plain cells": cells(one, one), "-0.0 in a list": list} { //canon:unordered each package compared alone
		comparePlan(t, name, p, p.Emits[0])
	}
}

// goPackage completes p as package p with one baked go emit.
func goPackage(p *ir.Package) *ir.Package {
	p.Name, p.Dir = "p", "p"
	p.Emits = []*ir.Emit{{Target: ir.TargetGo, Out: "out/go", Dir: "p/out/go", GoImport: "example.com/p", Mode: ir.ModeBaked, GoPackage: "p"}}
	return p
}

// comparePlan checks that the plan of p's go emit e has no problem and holds exactly the names of the Go file gen/go writes, scope by scope.
func comparePlan(t *testing.T, what string, p *ir.Package, e *ir.Emit) {
	t.Helper()
	files, err := gogen.Generate(p, e)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	pl := ir.PlanGoNames(p, e)
	if len(pl.Problems()) > 0 {
		t.Errorf("%s: %+v", what, pl.Problems())
	}
	got := declaredNames(t, goFile(t, files))
	want := goPlanned(p, pl)
	for goName, scope := range structScopes(p, pl) { //canon:unordered each struct compared alone
		sameNames(t, what+" "+goName, got[goName], want[scope])
		delete(got, goName)
	}
	sameNames(t, what+" package", got[""], want["package"])
	delete(got, "")
	if len(got) > 0 {
		t.Errorf("%s: structs the plan has no scope for: %v", what, slices.Sorted(maps.Keys(got)))
	}
}

// goFile is the generated <package>.gen.go.
func goFile(t *testing.T, files []ir.File) []byte {
	t.Helper()
	for _, f := range files {
		if strings.HasSuffix(f.Path, goSuffix) {
			return f.Content
		}
	}
	t.Fatal("no generated Go file")
	return nil
}

// structScopes maps the Go name of each generated type with fields or methods to the plan scope of its names, which is that Go name.
func structScopes(p *ir.Package, pl *ir.GoNamePlan) map[string]string {
	out := map[string]string{}
	for _, t := range p.Types {
		switch x := t.(type) {
		case *ir.Record:
			out[pl.TypeName(x)] = pl.TypeName(x)
		case *ir.Enum:
			out[pl.TypeName(x)] = pl.TypeName(x)
		case *ir.Variant:
			variantScopes(out, pl, x)
		}
	}
	for _, v := range p.Values {
		if rec, ok := tableElem(v); ok {
			out[pl.IDTypeName(rec)] = pl.IDTypeName(rec)
		}
	}
	for _, v := range pl.Emitted() {
		if v.Type.Kind == types.Table || v.Type.Kind == types.List && v.Type.KeyedBy != nil {
			out[pl.ContainerName(v)] = pl.ContainerName(v)
		}
	}
	if len(pl.Emitted()) > 0 {
		out[pl.Data().Type] = pl.Data().Type
	}
	return out
}

// variantScopes adds a variant's struct, its kind enum and each case type with fields.
func variantScopes(out map[string]string, pl *ir.GoNamePlan, v *ir.Variant) {
	out[pl.TypeName(v)] = pl.TypeName(v)
	out[pl.KindName(v)] = pl.KindName(v)
	for _, c := range v.Cases {
		if len(c.Fields) > 0 {
			out[pl.CaseName(v, c)] = pl.CaseName(v, c)
		}
	}
}

// tableElem is the record of a table value.
func tableElem(v *ir.Value) (*ir.Record, bool) {
	if v.Type.Kind != types.Table || v.Type.Elem == nil {
		return nil, false
	}
	rec, ok := v.Type.Elem.Named.(*ir.Record)
	return rec, ok
}

// declaredNames are the names a Go file declares: its package scope under "", and each struct's fields and methods under the struct's name.
func declaredNames(t *testing.T, src []byte) map[string]map[string]bool {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]bool{"": {}}
	add := func(scope, name string) {
		if out[scope] == nil {
			out[scope] = map[string]bool{}
		}
		out[scope][name] = true
	}
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		name := path.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		add("", name)
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				add("", d.Name.Name)
				continue
			}
			add(receiverName(d.Recv.List[0].Type), d.Name.Name)
		case *ast.GenDecl:
			genDeclNames(d, add)
		}
	}
	return out
}

// genDeclNames adds a declaration's names to the package, and a struct's fields to its own scope.
func genDeclNames(d *ast.GenDecl, add func(scope, name string)) {
	for _, s := range d.Specs {
		switch s := s.(type) {
		case *ast.TypeSpec:
			add("", s.Name.Name)
			structFields(s, add)
		case *ast.ValueSpec:
			for _, n := range s.Names {
				add("", n.Name)
			}
		}
	}
}

// structFields adds a struct type's fields to its own scope.
func structFields(s *ast.TypeSpec, add func(scope, name string)) {
	st, ok := s.Type.(*ast.StructType)
	if !ok {
		return
	}
	for _, fd := range st.Fields.List {
		for _, n := range fd.Names {
			add(s.Name.Name, n.Name)
		}
	}
}

func receiverName(x ast.Expr) string {
	if star, ok := x.(*ast.StarExpr); ok {
		x = star.X
	}
	if id, ok := x.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func sameNames(t *testing.T, what string, got, want map[string]bool) {
	t.Helper()
	g, w := slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want))
	if !slices.Equal(g, w) {
		t.Errorf("%s:\n generated %v\n planned   %v", what, g, w)
	}
}
