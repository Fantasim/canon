package ir_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// listing is a toy generator: one file naming its imports, then every type, constant and
// value of the package; the emits carry the import paths.
func listing(p *ir.Package, e *ir.Emit) ([]ir.File, error) {
	out := fmt.Sprintf("%s mode %d: import %s/rt", p.Name, e.Mode, e.GoImport)
	for _, imp := range p.Imports {
		for _, ie := range imp.Emits {
			if ie.Target == e.Target {
				out += " import " + ie.GoImport
			}
		}
	}
	for _, t := range p.Types {
		out += " type " + t.QName()
	}
	for _, c := range p.Consts {
		out += " const " + c.Name + " = " + c.V.CanonText()
	}
	for _, v := range p.Values {
		out += " value " + v.Name + " " + v.Schema
	}
	return []ir.File{{Path: e.GoPackage + ".gen.go", Content: []byte(out)}}, nil
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
		Imports: []*ir.PackageRef{{Name: "sovcommon.time", Dir: "sovcommon/time", Emits: []*ir.Emit{{
			Target: ir.TargetGo, Out: "@sovcommon/time", Dir: "../../sovcommon/time", GoImport: "gitlab.com/sovereign15/sovcommon/time", Mode: ir.ModeTypes,
		}}}},
		Types:  []ir.Type{potion},
		Consts: []*ir.Const{{Name: "MAX_HEAL", Type: ir.TypeRef{Kind: types.Int}, V: &value.Int{V: 9000, T: types.IntType}}},
		Values: []*ir.Value{{Name: "potions", Type: potions, Reload: true, Schema: "pipeline.Potion@f750790e", IDs: []string{"II_POT_HEAL_L"}}},
		Emits: []*ir.Emit{
			{Target: ir.TargetGo, Out: "out/go", Dir: "pipeline/out/go", GoImport: "example.com/potions", Mode: ir.ModeData, GoPackage: "potions"},
			{Target: ir.TargetCpp, Out: "@source/pipeline", Dir: "../../../Source/pipeline", Namespace: "pipeline"},
		},
	}
	var gen ir.Generator = listing
	files, err := gen(p, p.Emits[0])
	fmt.Println(p.Emits[0].Dir+"/"+files[0].Path, err)
	fmt.Println(string(files[0].Content))
	// Output:
	// pipeline/out/go/potions.gen.go <nil>
	// pipeline mode 3: import example.com/potions/rt import gitlab.com/sovereign15/sovcommon/time type pipeline.Potion const MAX_HEAL = 9000 value potions pipeline.Potion@f750790e
}

// A value's `$schema`: its type's name and the hash of its canon-fp v1 text.
func ExampleSchema() {
	deck := &ir.Record{Pkg: "teamboard", Name: "Deck", Fields: []*ir.Field{
		{Name: "layouts", WirePath: []string{"layouts"}, Type: ir.TypeRef{Kind: types.List, Elem: &ir.TypeRef{Kind: types.String}}},
		{Name: "maxHidden", WirePath: []string{"maxHidden"}, Type: ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true}},
	}}
	root := ir.TypeRef{Kind: types.Record, Named: deck}
	text, _ := ir.Fingerprint(&root, nil)
	id, err := ir.Schema("teamboard", "deck", &root, nil)
	fmt.Println(len(text), id, err)
	// Output: 155 teamboard.Deck@02af81fb <nil>
}
