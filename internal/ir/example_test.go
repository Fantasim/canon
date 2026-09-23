package ir_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// listing is a toy generator: one file naming every type, constant and value of the package.
func listing(p *ir.Package, e *ir.Emit) ([]ir.File, error) {
	out := fmt.Sprintf("%s mode %d:", p.Name, e.Mode)
	for _, t := range p.Types {
		out += " type " + t.QName()
	}
	for _, c := range p.Consts {
		out += " const " + c.Name + " = " + c.V.CanonText()
	}
	for _, v := range p.Values {
		out += " value " + v.Name + " " + v.Schema
	}
	return []ir.File{{Path: e.Out + "/" + e.GoPackage + ".gen.go", Content: []byte(out)}}, nil
}

// A generator is a pure function of the IR and one emit.
func Example() {
	potion := &ir.Record{Pkg: "pipeline", Name: "Potion", Fields: []*ir.Field{
		{Name: "heal", WirePath: []string{"nHeal"}, Type: ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true}},
		{Name: "cooldown", WirePath: []string{"dwCooldownMs"}, Type: ir.TypeRef{Kind: types.Duration}, Unit: types.UnitMs},
		{Name: "icon", WirePath: []string{"szIcon"}, Type: ir.TypeRef{Kind: types.String}, Cpp: ir.CppFieldOptions{Member: "szIcon", Type: "char[64]", Unit: types.UnitS, HasUnit: true}},
	}, Cpp: ir.CppOptions{Struct: "ItemProp", Header: "ItemProp.h", Access: ir.AccessFields}}
	potions := ir.TypeRef{Kind: types.List, Elem: &ir.TypeRef{Kind: types.Record, Named: potion}, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"id"}}}
	p := &ir.Package{
		Name: "pipeline", Dir: "pipeline", Doc: "Potions.",
		Imports: []*ir.PackageRef{{Name: "sovcommon.time", Dir: "sovcommon/time", Emits: []*ir.Emit{{Target: ir.TargetGo, Mode: ir.ModeTypes}}}},
		Types:   []ir.Type{potion},
		Consts:  []*ir.Const{{Name: "MAX_HEAL", Type: ir.TypeRef{Kind: types.Int}, V: &value.Int{V: 9000, T: types.IntType}}},
		Values:  []*ir.Value{{Name: "potions", Type: potions, Reload: true, Schema: "pipeline.Potion@f750790e", IDs: []string{"II_POT_HEAL_L"}}},
		Emits:   []*ir.Emit{{Target: ir.TargetGo, Out: "out/go", Mode: ir.ModeData, GoPackage: "potions"}, {Target: ir.TargetCpp, Namespace: "pipeline"}},
	}
	var gen ir.Generator = listing
	files, err := gen(p, p.Emits[0])
	fmt.Println(files[0].Path, err)
	fmt.Println(string(files[0].Content))
	// Output:
	// out/go/potions.gen.go <nil>
	// pipeline mode 3: type pipeline.Potion const MAX_HEAL = 9000 value potions pipeline.Potion@f750790e
}
