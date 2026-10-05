package ir_test

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// TestForeignPlanNames is DECISIONS 323 on the Emberfall shape, name by name: game.world imports game.tone, which it reaches only through game.core's types, with its go, cpp and ts emits (IMPLEMENTATION-PLAN §4.5, CODEGEN.md §2.8); its rows, id types, readers, rt import, hooks and free loader are named as CODEGEN.md §3.3, §5.9 and §5.14 say; game.core's baked hook fills the resolved status (§5.14 Refs); a forwarded getter named like a row's own is E8005 (§5.9).
func TestForeignPlanNames(t *testing.T) {
	pkgs := foreignWorld(t)
	world := pkgs["game.world"]
	core := typesByName(pkgs["game.core"])
	levels, reward := core["LevelRange"].(*ir.Record), core["Reward"].(*ir.Variant)
	coins := reward.Cases[0]
	start := world.Values[slices.IndexFunc(world.Values, func(v *ir.Value) bool { return v.Name == "start" })]
	goPl, cppPl := ir.PlanGoNames(world, emitOf(world, ir.TargetGo)), ir.PlanCppNames(world, emitOf(world, ir.TargetCpp))
	rt := goPl.RTImports()
	got := map[string]string{
		"tone targets":        importTargets(world, "game.tone"),
		"go row":              goPl.RowName(levels) + " " + goPl.Row(levels).ID + " " + goPl.RowHook(levels),
		"go readers":          goPl.ReaderName(levels) + " " + goPl.ReaderName(coins),
		"go hooks":            goPl.MakeName(levels) + " " + goPl.MakeCaseName(reward, coins) + " " + goPl.MakeCaseTypeName(reward, coins),
		"uses and rows":       fmt.Sprint(len(ir.ForeignUses(world, emitOf(world, ir.TargetGo)).Read), len(ir.ForeignRows(world, emitOf(world, ir.TargetGo)))),
		"go rt import":        fmt.Sprint(rt),
		"go resolved by core": fmt.Sprint(goPl.RecordHook(levels).Slots[4].Slot.Resolved),
		"go row collision":    problemNames(goPl.Problems()),
		"cpp make struct":     cppPl.MakeStruct("game.core"),
		"cpp hooks":           cppPl.MakeName(levels) + " " + cppPl.MakeCaseName(reward, coins) + " " + cppPl.MakeCaseTypeName(reward, coins),
		"cpp row":             cppPl.RowName(levels) + " " + cppPl.RowHook(levels),
		"cpp reader":          cppPl.ReaderName(levels),
		"cpp free loader":     cppPl.ForeignLoader(start),
		"cpp row collision":   problemNames(cppPl.Problems()),
	}
	want := map[string]string{
		"tone targets": "go cpp ts", "go row": "LevelRangeRow LevelRangeID Make_LevelRangeRow",
		"go readers": "decode_core_LevelRange decode_core_RewardCoins", "go hooks": "Make_LevelRange Make_Reward_Coins MakeCase_Reward_Coins",
		"go rt import": "[{game.core corert example.com/features/core/go/rt}]", "go resolved by core": "true",
		"go row collision": "MarkerRow.Record", "cpp make struct": "CoreMake", "cpp hooks": "LevelRange Reward_Coins Case_Reward_Coins",
		"uses and rows": "4 2", "cpp row": "LevelRangeRow LevelRangeRow", "cpp reader": "Read_Game_Core_LevelRange", "cpp free loader": "LoadStart", "cpp row collision": "MarkerRow.GetId",
	}
	for _, k := range slices.Sorted(maps.Keys(want)) {
		if got[k] != want[k] {
			t.Errorf("%s: got %q, want %q", k, got[k], want[k])
		}
	}
}

// foreignWorld builds testdata/foreign/emberfall.txtar's packages, by name.
func foreignWorld(t *testing.T) map[string]*ir.Package {
	t.Helper()
	return worldOf(t, "testdata/foreign/emberfall.txtar")
}

// worldOf builds a case's packages, by name.
func worldOf(t *testing.T, file string) map[string]*ir.Package {
	t.Helper()
	ar, err := txtar.ParseFile(file)
	if err != nil {
		t.Fatal(err)
	}
	w := newWorld(t)
	for _, f := range ar.Files {
		if f.Name != planFile {
			w.add(t, f.Name, f.Data)
		}
	}
	w.calls = w.fixtureCalls
	pkgs := map[string]*ir.Package{}
	for _, p := range w.build(t) {
		pkgs[p.Name] = p
	}
	return pkgs
}

// typesByName are p's types by Canon name.
func typesByName(p *ir.Package) map[string]ir.Type {
	out := map[string]ir.Type{}
	for _, t := range p.Types {
		name := t.QName()
		out[name[strings.LastIndex(name, ".")+1:]] = t
	}
	return out
}

// emitOf is p's emit of target t.
func emitOf(p *ir.Package, t ir.Target) *ir.Emit {
	return p.Emits[slices.IndexFunc(p.Emits, func(e *ir.Emit) bool { return e.Target == t })]
}

// importTargets are the targets of the emits p's import pkg carries.
func importTargets(p *ir.Package, pkg string) string {
	var out []string
	for _, ref := range p.Imports {
		for _, e := range ref.Emits {
			if ref.Name == pkg {
				out = append(out, targetNames[e.Target])
			}
		}
	}
	return strings.Join(out, " ")
}

// problemNames are a plan's problems as scope.name.
func problemNames(problems []ir.GoNameProblem) string {
	var out []string
	for _, pr := range problems {
		out = append(out, pr.Scope+"."+pr.Name)
	}
	return strings.Join(out, " ")
}

// TestForeignUsesTranslatedLiterals is log-2026-10-06 "U1 review" 5: a translated body writing a record of another package as a literal builds it through that package's hook, so ForeignUses counts it as written.
func TestForeignUsesTranslatedLiterals(t *testing.T) {
	other := &ir.Record{Pkg: "lib", Name: "Band"}
	fn := &ir.ExportFn{Name: "band", Kind: ir.FnTranslated, Body: &ir.Lit{T: ir.TypeRef{Kind: types.Record, Named: other}}}
	p := &ir.Package{Name: "app", Fns: []*ir.ExportFn{fn}}
	u := ir.ForeignUses(p, &ir.Emit{Target: ir.TargetGo, Mode: ir.ModeData})
	if len(u.Written) != 1 || u.Written[0] != other {
		t.Errorf("written %v, want lib.Band", u.Written)
	}
}

// TestOwnerAccessor is CODEGEN.md §3.3 and §5.9: a ref of another package's keyed list names the owner's accessor of that value exactly as the owner's own plan does, the value's @cpp(name:) when it has one (testdata/foreign/accessor.txtar).
func TestOwnerAccessor(t *testing.T) {
	pkgs := worldOf(t, "testdata/foreign/accessor.txtar")
	app, lib := pkgs["app"], pkgs["lib"]
	spawn := typesByName(app)["Spawn"].(*ir.Record)
	pl, own := ir.PlanCppNames(app, emitOf(app, ir.TargetCpp)), ir.PlanCppNames(lib, emitOf(lib, ir.TargetCpp))
	for i, want := range []string{"AllMonsters", "GetItems"} {
		r := spawn.Fields[i].Type.Ref
		v := lib.Values[slices.IndexFunc(lib.Values, func(v *ir.Value) bool { return v.Name == r.Value })]
		if _, owners := own.Accessor(v); pl.OwnerAccessor(r) != want || owners != want {
			t.Errorf("%s: OwnerAccessor %q, the owner's own accessor %q, want %q", r.Value, pl.OwnerAccessor(r), owners, want)
		}
	}
}
