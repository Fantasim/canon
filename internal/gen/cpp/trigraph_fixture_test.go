package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

const (
	triPkg  = "demo.tri"
	triText = "a??=b ??/ c ???" // three trigraphs' worth: `??=`, `??/` and a run of `?`
)

// triSpec is the baked-trigraph fixture's package: every string path holds triText (CODEGEN.md §2.6, DECISIONS 423).
var triSpec = textSpec{triPkg, "demo/tri", "demo::tri", triText}

func triPackage() *ir.Package { return triSpec.bakedPackage() }

// triTypesPackage is a types-mode package whose field wire name and variant case wire hold triText, which its decoder compares byte for byte (CODEGEN.md §5.13, DECISIONS 423).
func triTypesPackage() *ir.Package {
	prize := &ir.Variant{Pkg: triPkg, Name: "Prize", Tag: "kind", Cases: []*ir.Case{
		{Name: "coins", Wire: triText, Fields: []*ir.Field{field("amount", "amount", "", tInt)}}, {Name: "none", Wire: "none"},
	}}
	holder := &ir.Record{Pkg: triPkg, Name: "Holder", Fields: []*ir.Field{
		field("count", triText, "", tInt), field("prize", "prize", "", ir.TypeRef{Kind: types.Variant, Named: prize}),
	}}
	return &ir.Package{
		Name: triPkg, Dir: "demo/tri", Types: []ir.Type{prize, holder},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/tri/out", Mode: ir.ModeTypes, Namespace: "demo::tri"}},
	}
}
