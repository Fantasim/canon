package cppgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// importPair is demo.base (an enum, a record, a variant, a dependent type) and demo.app, whose
// record holds them; both are data-mode cpp emits in their own directories and namespaces.
func importPair() (base, app *ir.Package) {
	color := &ir.Enum{Pkg: "demo.base", Name: "Color", Members: []*ir.EnumMember{
		{Name: "red", Wire: "red"}, {Name: "green", Wire: "green", Index: 1}, {Name: "blue", Wire: "blue", Index: 2},
	}}
	tColor := ir.TypeRef{Kind: types.Enum, Named: color}
	pt := &ir.Record{Pkg: "demo.base", Name: "Pt", Fields: []*ir.Field{field("x", "x", "", tInt32)}}
	tPt := ir.TypeRef{Kind: types.Record, Named: pt}
	paint := &ir.Variant{Pkg: "demo.base", Name: "Paint", Tag: "kind", Cases: []*ir.Case{
		{Name: "solid", Wire: "solid", Fields: []*ir.Field{field("tint", "tint", "", tColor)}}, {Name: "clear", Wire: "clear"},
	}}
	// type Shade(c: Color) = match c { red => String, green => Int32, blue => Never } (CODEGEN.md §5.6).
	shade := &ir.Dependent{
		Pkg: "demo.base", Name: "Shade", Params: 1, Disc: &tColor, ByMember: []int{0, 1, ir.NoBranch},
		Branches: []*ir.Branch{{Name: "red", Members: []int{0}, Type: tString}, {Name: "green", Members: []int{1}, Type: tInt32}},
	}
	baseEmit := &ir.Emit{Target: ir.TargetCpp, Dir: "demo/base/out", Mode: ir.ModeData, Namespace: "demo::base"}
	base = &ir.Package{Name: "demo.base", Dir: "demo/base", Types: []ir.Type{color, pt, paint, shade}, Emits: []*ir.Emit{baseEmit}}
	return base, appPackage(base, tColor, tPt, paint)
}

// appPackage holds base's types; its record demo, a class of namespace demo::app, would hijack
// an imported name written demo::base::… (log-2026-09-25 "imported qualifiers can be hijacked").
func appPackage(base *ir.Package, tColor, tPt ir.TypeRef, paint *ir.Variant) *ir.Package {
	inline := &ir.Field{Name: "paint", Type: ir.TypeRef{Kind: types.Variant, Named: paint}, Inline: true}
	alt := field("alt", "alt", "", tPt)
	alt.Optional = true
	shade := field("shade", "shade", "", ir.TypeRef{
		Kind: types.TypeApp, Named: base.Types[3], Args: []*ir.Source{{From: types.ArgField, WirePath: []string{"color"}}},
	})
	shade.Optional = true
	order := &ir.Record{Pkg: "demo.app", Name: "Order", Fields: []*ir.Field{
		field("id", "id", "", tString), field("color", "color", "", tColor), field("at", "at", "", tPt),
		field("trail", "trail", "", listOf(tPt)), inline, alt, shade,
	}}
	hijack := &ir.Record{Pkg: "demo.app", Name: "demo", Fields: []*ir.Field{field("n", "n", "", tInt)}}
	order.Methods = []*ir.ExportFn{{Name: "hue", Kind: ir.FnLookup, Result: tInt, Params: []*ir.Param{{Name: "c", Type: tColor}}}}
	elem := ir.TypeRef{Kind: types.Record, Named: order}
	orders := &ir.Value{Name: "orders", Schema: "demo.app.Order@00000001", Type: ir.TypeRef{
		Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"id"}},
	}}
	red, blue := &value.Member{Index: 0}, &value.Member{Index: 2}
	warm := fnSpec{
		name: "warm", params: []*ir.Param{{Name: "c", Type: tColor}}, result: tBool,
		body:    bin(ir.OpEq, tBool, param(0, tColor), lit(tColor, red)),
		vectors: [][]value.Value{vals(red, boolean(true)), vals(blue, boolean(false))},
	}
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "demo/app/out", Mode: ir.ModeData, Namespace: "demo::app"}
	return &ir.Package{
		Name: "demo.app", Dir: "demo/app", Types: []ir.Type{hijack, order}, Values: []*ir.Value{orders},
		Fns:     []*ir.ExportFn{warm.build()},
		Imports: []*ir.PackageRef{{Name: base.Name, Dir: base.Dir, Emits: base.Emits}},
		Emits:   []*ir.Emit{{Target: ir.TargetJSON, Dir: "demo/app/out"}, emit},
	}
}

func importBase() *ir.Package { base, _ := importPair(); return base }

func importApp() *ir.Package { _, app := importPair(); return app }

// CODEGEN.md §2.8: imported headers by relative path, qualified types, their own decoders.
func TestImportsCompileAndRun(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []*ir.Package{importBase(), importApp()} {
		out := filepath.Join(dir, filepath.FromSlash(cppEmit(p).Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, out, generate(t, p))
	}
	copyFile(t, filepath.Join("testdata", "main", "imports_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	data := filepath.Join("testdata", "main", "imports", "orders.json")
	copyFile(t, data, filepath.Join(dir, "orders.json"), same)
	sources := []string{"demo/base/out/base.gen.cpp", "demo/app/out/app.gen.cpp", "demo/app/out/app_conformance.gen.cpp", "main.cpp"}
	for _, out := range buildAndRun(t, dir, sources, dir) {
		if !strings.Contains(out, "failures: 0\n") {
			t.Errorf("driver output:\n%s", out)
		}
	}
}
