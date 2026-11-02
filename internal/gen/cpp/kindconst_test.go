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

// kindVariant is a variant of the cases (the last has a field), with its checker twin that kind values point at.
func kindVariant(pkg, name string, cases ...string) (*ir.Variant, *types.VariantKindType) {
	iv := &ir.Variant{Pkg: pkg, Name: name, Tag: "kind"}
	twin := &types.VariantType{Pkg: pkg, Name: name}
	for i, c := range cases {
		cs := &ir.Case{Name: c, Wire: c}
		if i == len(cases)-1 {
			cs.Fields = []*ir.Field{field("n", "n", "", tInt)}
		}
		iv.Cases = append(iv.Cases, cs)
		twin.Cases = append(twin.Cases, &types.CaseType{Variant: twin, Name: c, Wire: c, Index: i})
	}
	return iv, &types.VariantKindType{Variant: twin}
}

// kindConst is a constant of the kind of v at the index; kindList a list of kinds at the indexes.
func kindConst(name string, v *ir.Variant, kind *types.VariantKindType, index int) *ir.Const {
	return &ir.Const{Name: name, Doc: "A kind.", Type: ir.TypeRef{Kind: types.VariantKind, Named: v}, V: &value.CaseKind{T: kind, Index: index}}
}

func kindList(name string, v *ir.Variant, kind *types.VariantKindType, indexes ...int) *ir.Const {
	var vals []value.Value
	for _, i := range indexes {
		vals = append(vals, &value.CaseKind{T: kind, Index: i})
	}
	return &ir.Const{Name: name, Doc: "Kinds.", Type: listOf(ir.TypeRef{Kind: types.VariantKind, Named: v}), V: &value.List{Elems: vals}}
}

// kindPackages are demo.reward (a variant) and demo.kinds, both types-mode cpp emits: constants of its own kinds (one, a list of two, a list of eight, a list of seven) and one of demo.reward's.
func kindPackages() (*ir.Package, *ir.Package) {
	reward, rkind := kindVariant("demo.reward", "Reward", "gold", "item")
	rp := &ir.Package{
		Name: "demo.reward", Dir: "demo/reward", Types: []ir.Type{reward},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Dir: "demo/reward/out", Mode: ir.ModeTypes, Namespace: "demo::reward"}},
	}
	shape, skind := kindVariant("demo.kinds", "Shape", "dot", "box")
	sp := &ir.Package{
		Name: "demo.kinds", Dir: "demo/kinds", Types: []ir.Type{shape},
		Imports: []*ir.PackageRef{{Name: rp.Name, Dir: rp.Dir, Emits: rp.Emits}},
		Consts: []*ir.Const{
			kindConst("K", shape, skind, 0), kindList("KS", shape, skind, 0, 1),
			kindList("Seven", shape, skind, 0, 1, 0, 1, 0, 1, 0), kindList("Eight", shape, skind, 0, 1, 0, 1, 0, 1, 0, 1),
			kindConst("BK", reward, rkind, 0),
		},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Dir: "demo/kinds/out", Mode: ir.ModeTypes, Namespace: "demo::kinds"}},
	}
	return rp, sp
}

// kindLines are the constants in the header: a kind is a constexpr enumerator, a short list an inline std::vector, a list of eight or more reads a detail array (DECISIONS 392).
var kindLines = []string{
	"inline constexpr ShapeKind K = ShapeKind::dot;\n",
	"inline constexpr ::demo::reward::RewardKind BK = ::demo::reward::RewardKind::gold;\n",
	"inline const std::vector<ShapeKind> KS = {ShapeKind::dot, ShapeKind::box};\n",
	"inline const std::vector<ShapeKind> Seven = {ShapeKind::dot, ShapeKind::box, ShapeKind::dot, ShapeKind::box, ShapeKind::dot, ShapeKind::box, ShapeKind::dot};\n",
	"inline constexpr ShapeKind kEightElems[] = {\n",
}

// CODEGEN.md §4.4, §5.1, DECISIONS 438 (ADR-0026 §10.2), 392: gen/cpp writes a constant of a variant's kind as an inline constexpr enumerator after the enums, a short list of kinds inline, a list of eight or more from a detail array; one of another package's variant is qualified; the header compiles, static_asserts hold and the driver runs, on every compiler and mode.
func TestKindConstantsCompile(t *testing.T) {
	t.Parallel()
	rp, sp := kindPackages()
	dir := t.TempDir()
	var header string
	for _, p := range []*ir.Package{rp, sp} {
		out := filepath.Join(dir, filepath.FromSlash(cppEmit(p).Dir))
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		files := generate(t, p)
		writeFiles(t, out, files)
		for _, f := range files {
			if p == sp && strings.HasSuffix(f.Path, ".gen.h") {
				header = string(f.Content)
			}
		}
	}
	for _, want := range kindLines {
		if !strings.Contains(header, want) {
			t.Errorf("header lacks:\n%s", want)
		}
	}
	if enums, consts := strings.Index(header, "enum class ShapeKind"), strings.Index(header, "ShapeKind K ="); enums < 0 || consts < enums {
		t.Errorf("the constants (%d) do not follow the kind enum (%d)", consts, enums)
	}
	copyFile(t, filepath.Join("testdata", "main", "kindconst_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	sources := []string{"demo/reward/out/reward.gen.cpp", "demo/kinds/out/kinds.gen.cpp", "main.cpp"}
	for _, out := range buildAndRun(t, dir, sources) {
		if out != "ok\n" {
			t.Errorf("driver printed %q", out)
		}
	}
}
