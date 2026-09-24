package cppgen_test

import (
	"math"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

var (
	tString   = ir.TypeRef{Kind: types.String}
	tInt      = ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true}
	tBool     = ir.TypeRef{Kind: types.Bool}
	tDuration = ir.TypeRef{Kind: types.Duration}
	tFloat    = ir.TypeRef{Kind: types.Float, Bits: 64}
)

const pipelineSchema = "pipeline.Potion@f750790e"

func num(v int64) *value.Int                  { return &value.Int{V: v} }
func flt(v float64) *value.Float              { return &value.Float{V: v} }
func str(s string) *value.Str                 { return &value.Str{V: s} }
func boolean(b bool) *value.Bool              { return &value.Bool{V: b} }
func dur(ms int64) *value.Dur                 { return &value.Dur{Ms: ms} }
func lit(t ir.TypeRef, v value.Value) *ir.Lit { return &ir.Lit{T: t, V: v} }

// field is a field whose wire name is one key.
func field(name, wire, doc string, t ir.TypeRef) *ir.Field {
	return &ir.Field{Name: name, Doc: doc, WirePath: []string{wire}, Type: t}
}

// vec is a conformance vector over the receiver paths and arguments; a code means no value.
func vec(recv, args []value.Value, want value.Value, code diag.Code) *ir.Vector {
	return &ir.Vector{Recv: recv, Args: args, Want: want, Code: code, TSWant: want, TSCode: code}
}

// pipeline is examples/pipeline after stage E, with the vectors of CONFORMANCE.md §6.6.
func pipeline() *ir.Package {
	potion := &ir.Record{Pkg: "pipeline", Name: "Potion", Doc: "A healing potion. Each one is a file under data/."}
	potion.Fields = []*ir.Field{
		field("id", "dwID", "The item define. Stored in inventories: never renamed.", tString),
		field("name", "szName", "Text key of the displayed name.", tString),
		field("heal", "nHeal", "Hit points restored.", tInt),
		field("cooldown", "dwCooldownMs", "Time before the same potion can be drunk again.", tDuration),
		field("stack", "nStack", "How many potions fit in one inventory slot.", tInt),
	}
	potion.Fields[2].Range = &types.Bound{Lo: types.Limit{I: 1}, Hi: types.Limit{I: 100_000}, HasLo: true, HasHi: true, HiIncluded: true}
	potion.Fields[4].Default = num(99)
	heal := func(missing int64, want int64) *ir.Vector {
		return vec([]value.Value{num(500)}, []value.Value{num(missing)}, num(want), "")
	}
	potion.Methods = []*ir.ExportFn{
		{
			Name: "isStrong", Kind: ir.FnPrecomputed, Result: tBool,
			Doc: "No runtime input: computed by `canon build` for every potion and shipped as a value.\n" +
				"The generated getter just returns it, in every language.",
		},
		{
			Name: "healFor", File: "potion.canon", Kind: ir.FnTranslated, Result: tInt,
			Doc: "Needs a runtime input (the player's missing HP), so the body is translated into\n" +
				"each target. Only the portable subset is allowed, and `canon build` emits a\n" +
				"conformance test per target so the translations cannot drift.",
			Params: []*ir.Param{{Name: "missingHp", Type: tInt}},
			Reads:  []*ir.Read{{Name: "heal", Path: []string{"heal"}, Type: tInt}},
			Body: &ir.Call{T: tInt, Fn: ir.BuiltinMin, Args: []ir.PExpr{
				&ir.ReadRef{T: tInt, Index: 0},
				&ir.Call{T: tInt, Fn: ir.BuiltinMax, Args: []ir.PExpr{&ir.ParamRef{T: tInt, Index: 0}, lit(tInt, num(0))}},
			}},
			Vectors: []*ir.Vector{
				heal(200, 200), heal(9000, 500), heal(-5, 0), heal(math.MinInt64, 0), heal(-1, 0), heal(0, 0),
				heal(1, 1), heal(499, 499), heal(500, 500), heal(501, 500), heal(math.MaxInt64, 500),
			},
		},
	}
	elem := ir.TypeRef{Kind: types.Record, Named: potion}
	potions := &ir.Value{
		Name: "potions", Reload: true, Schema: pipelineSchema, IDs: []string{"II_POT_HEAL_L", "II_POT_HEAL_S"},
		Doc: "One JSON file per potion, as today. Each file is one entry; findings point into it.\n" +
			"Reloadable: tuned live during balance sessions, and runtimes keep potion ids, never\n" +
			"Potion pointers, across ticks.",
		Type: ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"dwID"}}},
	}
	return &ir.Package{
		Name: "pipeline", Dir: "pipeline", Types: []ir.Type{potion}, Values: []*ir.Value{potions},
		Emits: []*ir.Emit{
			{Target: ir.TargetJSON, Out: "out/potions.json", Dir: "pipeline/out", FileName: "potions.json", Values: []string{"potions"}},
			{Target: ir.TargetCpp, Out: "out/", Dir: "pipeline/out", Mode: ir.ModeData, Namespace: "sov::gen"},
			{Target: ir.TargetGo, Out: "out/go/", Dir: "pipeline/out/go", GoImport: "example.com/potions", Mode: ir.ModeData, GoPackage: "potions"},
			{Target: ir.TargetView, Out: "out/potion.view.json", Dir: "pipeline/out"},
		},
	}
}

// cppEmit is the package's cpp emit.
func cppEmit(p *ir.Package) *ir.Emit {
	for _, e := range p.Emits {
		if e.Target == ir.TargetCpp {
			return e
		}
	}
	return nil
}
