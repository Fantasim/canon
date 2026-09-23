package jsongen_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	jsongen "github.com/fantasim/canonlang/internal/gen/json"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	potionsGolden = "../../../examples/pipeline/expected/potions.json"
	potionsSHA    = "2a51028fc9ef1b8b783b4a1c6f8470f61cfc81e06c3f06ef96c738ab7d29bb75"
	potionsSchema = "pipeline.Potion@f750790e"
)

// pipeline is examples/pipeline after stage E: `potions`, @reload, emitted to one json file
// and read by the data-mode go and cpp emits; isStrong is precomputed, healFor translated.
func pipeline() *ir.Package {
	potion := newRecord("pipeline", "Potion", fld("id", tString).at("dwID"), fld("name", tString).at("szName"),
		fld("heal", tInt).at("nHeal"), fld("cooldown", tDuration).at("dwCooldownMs"), fld("stack", tInt).at("nStack"))
	coll := &types.Collection{Kind: types.CollLet, Pkg: "pipeline", Name: "potions", Elem: potion.t, KeyedBy: potion.t.Fields[0]}
	row := func(id, name string, heal, cooldown, stack int64) *value.Record {
		r := potion.rec(str(id), str(name), num(heal), dur(cooldown), num(stack))
		r.Ident = &value.Identity{Coll: coll, Key: value.Key{S: id}}
		return r
	}
	rows := []*value.Record{
		row("II_POT_HEAL_L", "IDS_PROPITEM_TXT_POT_L", 2500, 8000, 20),
		row("II_POT_HEAL_S", "IDS_PROPITEM_TXT_POT_S", 500, 3000, 99),
	}
	potion.method("isStrong", tBool, rows, func(r *value.Record) value.Value {
		return boolean(r.Fields[2].(*value.Int).V >= 2000)
	})
	potion.ir.Methods = append(potion.ir.Methods, &ir.ExportFn{Name: "healFor", Kind: ir.FnTranslated,
		Params: []*ir.Param{{Name: "missingHp", Type: tInt.ir}}, Result: tInt.ir})
	elems := make([]value.Value, len(rows))
	for i, r := range rows {
		elems[i] = r
	}
	elem := potion.typ().ir
	potions := &ir.Value{Name: "potions", Reload: true, Schema: potionsSchema, IDs: []string{"II_POT_HEAL_L", "II_POT_HEAL_S"},
		Type: ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"dwID"}}},
		V:    &value.List{T: &types.ListType{Elem: potion.t, KeyedBy: potion.t.Fields[0]}, Elems: elems}}
	return &ir.Package{Name: "pipeline", Dir: "pipeline", Types: []ir.Type{potion.ir}, Values: []*ir.Value{potions},
		Emits: []*ir.Emit{
			{Target: ir.TargetJSON, Out: "out/potions.json", Dir: "pipeline/out", FileName: "potions.json", Values: []string{"potions"}},
			{Target: ir.TargetCpp, Out: "out/", Dir: "pipeline/out", Mode: ir.ModeData, Namespace: "sov::gen"},
			{Target: ir.TargetGo, Out: "out/go/", Dir: "pipeline/out/go", GoImport: "example.com/pipeline/out/go", Mode: ir.ModeData, GoPackage: "potions"},
			{Target: ir.TargetView, Out: "out/potion.view.json", Dir: "pipeline/out"},
		}}
}

// WIRE.md §8.1 file mode and §8.3 (GEN-01): out/potions.json is expected/potions.json, byte for byte.
func TestPipelinePotions(t *testing.T) {
	p := pipeline()
	files, err := jsongen.Generate(p, p.Emits[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "potions.json" {
		t.Fatalf("files: %+v", files)
	}
	want, err := os.ReadFile(potionsGolden)
	if err != nil {
		t.Fatal(err)
	}
	got := files[0].Content
	sum := sha256.Sum256(got)
	if !bytes.Equal(got, want) || hex.EncodeToString(sum[:]) != potionsSHA {
		t.Errorf("potions.json:\n%s\nwant:\n%s", got, want)
	}
}
