package gogen_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	kindShapePkg  = "demo.kinds"
	kindRewardPkg = "demo.reward"
	kindDir       = "demo/kinds/out/go"
	kindRewardDir = "demo/reward/out/go"
)

// kindVariant is a variant of the cases, with its checker twin that kind values point at.
func kindVariant(pkg, name string, cases ...string) (*ir.Variant, *types.VariantKindType) {
	iv := &ir.Variant{Pkg: pkg, Name: name, Tag: "kind"}
	twin := &types.VariantType{Pkg: pkg, Name: name}
	for i, c := range cases {
		iv.Cases = append(iv.Cases, &ir.Case{Name: c, Wire: c})
		twin.Cases = append(twin.Cases, &types.CaseType{Variant: twin, Name: c, Wire: c, Index: i})
	}
	return iv, &types.VariantKindType{Variant: twin}
}

// kindConst is a constant of the kind of v at the index; kindList a list of kinds at the indexes.
func kindConst(name string, v *ir.Variant, kind *types.VariantKindType, index int) *ir.Const {
	return &ir.Const{Name: name, Doc: "A kind.", Type: typed(v, types.VariantKind), V: &value.CaseKind{T: kind, Index: index}}
}

func kindList(name string, v *ir.Variant, kind *types.VariantKindType, indexes ...int) *ir.Const {
	var vals []value.Value
	for _, i := range indexes {
		vals = append(vals, &value.CaseKind{T: kind, Index: i})
	}
	return &ir.Const{Name: name, Doc: "Kinds.", Type: listT(typed(v, types.VariantKind)), V: &value.List{Elems: vals}}
}

// kindPackages are demo.reward (a variant) and demo.kinds (constants of its own kinds, a list of two, a list of eight, and one of demo.reward's), both baked.
func kindPackages() (*ir.Package, *ir.Package) {
	reward, rkind := kindVariant(kindRewardPkg, "Reward", "gold", "item")
	rp := &ir.Package{Name: kindRewardPkg, Dir: "demo/reward", Types: []ir.Type{reward}, Emits: []*ir.Emit{rangeEmit(kindRewardDir, "reward", ir.ModeBaked)}}
	shape, skind := kindVariant(kindShapePkg, "Shape", "dot", "box")
	eight := kindList("Eight", shape, skind, 0, 1, 0, 1, 0, 1, 0, 1)
	sp := &ir.Package{
		Name: kindShapePkg, Dir: "demo/kinds", Types: []ir.Type{shape},
		Imports: []*ir.PackageRef{{Name: kindRewardPkg, Dir: "demo/reward", Emits: rp.Emits}},
		Consts: []*ir.Const{
			kindConst("K", shape, skind, 0), kindList("KS", shape, skind, 0, 1), eight, kindConst("BK", reward, rkind, 0),
		},
		Emits: []*ir.Emit{rangeEmit(kindDir, "kinds", ir.ModeBaked)},
	}
	return rp, sp
}

// kindLines are the declarations of the constants: a var of the kind enum's member (the kind enum is an opaque struct, CODEGEN.md §5.2), a function for a list (§5.1).
var kindLines = []string{
	"var K = ShapeKindDot\n",
	"var Bk = reward.RewardKindGold\n",
	"func Ks() rt.List[ShapeKind] { return rt.MakeList([]ShapeKind{ShapeKindDot, ShapeKindBox}) }\n",
}

// CODEGEN.md §4.4, §5.1, DECISIONS 438 (ADR-0026 §10.2): gen/go writes a constant of a variant's kind as a var of the kind enum's member, a list of kinds as a function, one of another package's variant qualified; the module compiles, vets and runs (K == ShapeKindDot, Bk == reward.RewardKindGold, Ks has 2, Eight has 8, K.String() is dot).
func TestKindConstants(t *testing.T) {
	rp, sp := kindPackages()
	files := generateData(t, rp, sp)
	src := string(files[kindDir+"/kinds.gen.go"])
	for _, want := range kindLines {
		if !strings.Contains(src, want) {
			t.Errorf("no %q in:\n%s", want, src)
		}
	}
	runData(t, files, kindDir, "testdata/smoke/kindconst_test.go", nil)
}
