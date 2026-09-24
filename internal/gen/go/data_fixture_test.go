package gogen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	dataModule   = "example.com/data"
	shopPkg      = "demo.shop"
	basePkg      = "demo.base"
	potionSchema = "pipeline.Potion@f750790e" // FINGERPRINT.md vector 1
)

var (
	strT   = ir.TypeRef{Kind: types.String}
	durT   = ir.TypeRef{Kind: types.Duration}
	fltT   = ir.TypeRef{Kind: types.Float, Bits: 64}
	f32T   = ir.TypeRef{Kind: types.Float, Bits: 32}
	i8T    = ir.TypeRef{Kind: types.Int, Bits: 8, Signed: true}
	i16T   = ir.TypeRef{Kind: types.Int, Bits: 16, Signed: true}
	i32T   = ir.TypeRef{Kind: types.Int, Bits: 32, Signed: true}
	u8T    = ir.TypeRef{Kind: types.Int, Bits: 8}
	u16T   = ir.TypeRef{Kind: types.Int, Bits: 16}
	u64T   = ir.TypeRef{Kind: types.Int, Bits: 64}
	schema = map[string]string{
		"items": "demo.shop.Item@0000000a", "shelves": "demo.shop.Shelf@0000000b",
		"config": "demo.shop.Config@0000000c", "home": "demo.shop.Point@0000000d",
	}
)

// wired is a field whose wire name is one key.
func wired(name, key, doc string, t ir.TypeRef) *ir.Field {
	return &ir.Field{Name: name, Doc: doc, WirePath: []string{key}, Type: t}
}

func opt(f *ir.Field) *ir.Field { f.Optional = true; return f }

func optT(t ir.TypeRef) ir.TypeRef { return ir.TypeRef{Kind: types.Optional, Elem: &t} }

func listT(t ir.TypeRef) ir.TypeRef { return ir.TypeRef{Kind: types.List, Elem: &t} }

func typed(n ir.Type, k types.Kind) ir.TypeRef { return ir.TypeRef{Kind: k, Named: n} }

// refT is a ref into the value of pkg whose rows are elem; keyed lists key by String.
func refT(pkg, value string, elem ir.Type, keyed bool) ir.TypeRef {
	return ir.TypeRef{Kind: types.Ref, Key: &strT, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: pkg, Value: value, Elem: elem, Keyed: keyed}}
}

func goData(dir, pkg string) *ir.Emit {
	return &ir.Emit{Target: ir.TargetGo, Out: "out/go/", Dir: dir + "/out/go", GoImport: dataModule + "/" + dir + "/out/go", Mode: ir.ModeData, GoPackage: pkg}
}

// potionPipeline is examples/pipeline in data mode, without its translated healFor (not this unit's).
func potionPipeline() *ir.Package {
	potion := &ir.Record{Pkg: "pipeline", Name: "Potion", Doc: "A healing potion. Each one is a file under data/."}
	potion.Fields = []*ir.Field{
		wired("id", "dwID", "The item define. Stored in inventories: never renamed.", strT),
		wired("name", "szName", "Text key of the displayed name.", strT),
		wired("heal", "nHeal", "Hit points restored.", intT),
		wired("cooldown", "dwCooldownMs", "Time before the same potion can be drunk again.", durT),
		wired("stack", "nStack", "How many potions fit in one inventory slot.", intT),
	}
	potion.Methods = []*ir.ExportFn{{
		Name: "isStrong", Kind: ir.FnPrecomputed, Result: boolT,
		Doc: "No runtime input: computed by `canon build` for every potion and shipped as a value.\n" +
			"The generated getter just returns it, in every language.",
	}}
	elem := typed(potion, types.Record)
	potions := &ir.Value{
		Name: "potions", Reload: true, Schema: potionSchema,
		Doc: "One JSON file per potion, as today. Each file is one entry; findings point into it.\n" +
			"Reloadable: tuned live during balance sessions, and runtimes keep potion ids, never\n" +
			"Potion pointers, across ticks.",
		Type: ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"dwID"}}},
	}
	return &ir.Package{
		Name: "pipeline", Dir: "pipeline", Types: []ir.Type{potion}, Values: []*ir.Value{potions},
		Emits: []*ir.Emit{goData("pipeline", "potions")},
	}
}

// base is a baked package the shop imports: an enum and a table whose ids are an enum.
func base() *ir.Package {
	role := &ir.Enum{Pkg: basePkg, Name: "Role", Members: []*ir.EnumMember{{Name: "viewer", Wire: "viewer"}, {Name: "admin", Wire: "admin", Index: 1}}}
	rank := &ir.Record{Pkg: basePkg, Name: "Rank", Fields: []*ir.Field{wired("label", "label", "", strT)}}
	twin := &types.RecordType{Pkg: basePkg, Name: "Rank", Fields: []*types.Field{{Name: "label"}}}
	coll := &types.Collection{Kind: types.CollLet, Pkg: basePkg, Name: "ranks"}
	entry := func(id, label string) *value.Record {
		return &value.Record{T: twin, Fields: []value.Value{&value.Str{V: label}}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: id}}}
	}
	ranks := &ir.Value{
		Name: "ranks", Type: ir.TypeRef{Kind: types.Table, Elem: &ir.TypeRef{Kind: types.Record, Named: rank}},
		IDs: []string{"low", "high"}, V: &value.Table{Entries: []*value.Record{entry("low", "Low"), entry("high", "High")}},
	}
	e := goData("demo/base", "base")
	e.Mode = ir.ModeBaked
	return &ir.Package{Name: basePkg, Dir: "demo/base", Types: []ir.Type{role, rank}, Values: []*ir.Value{ranks}, Emits: []*ir.Emit{e}}
}
