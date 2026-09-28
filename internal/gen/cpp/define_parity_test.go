package cppgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// definesParityPackage is demo.defs: a table hits of Hit, whose fields ref the define table
// monsters (one, optional, a list, an optional list), with a Go and a C++ data emit.
func definesParityPackage() (*ir.Package, *ir.Emit) {
	mon := ir.TypeRef{Kind: types.Ref, Key: &tString, Ref: &ir.RefTarget{Coll: types.CollDefines, Pkg: "demo.defs", Value: "monsters", Local: true}}
	optional := func(f *ir.Field) *ir.Field { f.Optional = true; return f }
	bonus := &ir.Record{Pkg: "demo.defs", Name: "Bonus", Fields: []*ir.Field{field("kind", "kind", "", mon), field("amount", "amount", "", tInt)}}
	hit := &ir.Record{Pkg: "demo.defs", Name: "Hit", Fields: []*ir.Field{
		field("monster", "monster", "", mon), optional(field("alt", "alt", "", mon)),
		field("all", "all", "", listOf(mon)), optional(field("maybe", "maybe", "", listOf(mon))),
		{Name: "deep", WirePath: []string{"legacy", "deep"}, Type: mon},
		{Name: "bonuses", Type: listOf(ir.TypeRef{Kind: types.Record, Named: bonus}), Pairs: &types.Pairs{Keys: [2]string{"k{i}", "a{i}"}, Slots: 2}},
	}}
	elem := ir.TypeRef{Kind: types.Record, Named: hit}
	hits := &ir.Value{Name: "hits", Schema: "demo.defs.Hit@00000001", IDs: []string{"a"}, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}}
	goEmit := &ir.Emit{Target: ir.TargetGo, Out: "out/go/", Dir: "demo/defs/out/go", GoImport: parityModule + "/defs", Mode: ir.ModeData, GoPackage: "defs"}
	cppEmit := &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "demo/defs/out", Mode: ir.ModeData, Namespace: "demo::defs"}
	return &ir.Package{
		Name: "demo.defs", Dir: "demo/defs", Types: []ir.Type{bonus, hit}, Values: []*ir.Value{hits}, Emits: []*ir.Emit{cppEmit, goEmit},
		Defines: []*ir.DefineTable{{Pkg: "demo.defs", Value: "monsters", Names: []string{"MI_A", "MI_B"}, Values: []int64{7, 9}}},
	}, goEmit
}

// defineParityRows are hits.json rows and what both drivers print for them.
var defineParityRows = []struct{ name, rows, want string }{
	{"good", `{"$id": "a", "monster": "MI_A", "alt": "MI_B", "all": ["MI_A", "MI_B"], "maybe": null, "legacy": {"deep": "MI_B"}, "k0": "MI_B", "a0": 3},` +
		`{"$id": "b", "monster": "MI_B", "alt": null, "all": [], "maybe": ["MI_A"], "legacy": {"deep": "MI_A"}}`,
		"a monster=MI_A=7 alt=MI_B=9 all=[MI_A=7,MI_B=9] maybe=none deep=MI_B=9 bonuses=[MI_B=9:3]" +
			" | b monster=MI_B=9 alt=none all=[] maybe=[MI_A=7] deep=MI_A=7 bonuses=[]"},
	{"one", `{"$id": "a", "monster": "MI_Z", "alt": null, "all": [], "maybe": null, "legacy": {"deep": "MI_A"}}`,
		"rows[0].monster: define MI_Z is not in this program's Monsters table"},
	{"optional", `{"$id": "a", "monster": "MI_A", "alt": "mi_b", "all": [], "maybe": null, "legacy": {"deep": "MI_A"}}`,
		"rows[0].alt: define mi_b is not in this program's Monsters table"},
	{"list", `{"$id": "a", "monster": "MI_A", "alt": null, "all": ["MI_B", "MI_C"], "maybe": null, "legacy": {"deep": "MI_A"}}`,
		"rows[0].all[1]: define MI_C is not in this program's Monsters table"},
	{"optlist", `{"$id": "a", "monster": "MI_A", "alt": null, "all": [], "maybe": ["MI_"], "legacy": {"deep": "MI_A"}}`,
		"rows[0].maybe[0]: define MI_ is not in this program's Monsters table"},
	{"order", `{"$id": "a", "monster": "MI_Z", "alt": 3, "all": [], "maybe": null, "legacy": {"deep": "MI_A"}}`,
		"rows[0].monster: define MI_Z is not in this program's Monsters table"},
	{"path", `{"$id": "a", "monster": "MI_A", "alt": null, "all": [], "maybe": null, "legacy": {"deep": "MI_D"}}`,
		"rows[0].legacy.deep: define MI_D is not in this program's Monsters table"},
	{"pairs", `{"$id": "a", "monster": "MI_A", "alt": null, "all": [], "maybe": null, "legacy": {"deep": "MI_A"}, "k0": "MI_A", "a0": 1, "k1": "MI_E", "a1": 2}`,
		"rows[0].k1: define MI_E is not in this program's Monsters table"},
	{"kind", `{"$id": "a", "monster": 1, "alt": null, "all": [], "maybe": null, "legacy": {"deep": "MI_A"}}`, "rows[0].monster: expected a string"},
}

// CODEGEN.md §5.8: both loaders look define refs up as they read them, with the same accepts, refusals and texts.
func TestDefineParity(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("compiles generated Go and C++")
	}
	p, goEmit := definesParityPackage()
	dir := t.TempDir()
	args := make([]string, len(defineParityRows))
	for i, r := range defineParityRows {
		args[i] = filepath.Join(dir, r.name+".json")
		body := `{"$schema": "demo.defs.Hit@00000001", "rows": [` + r.rows + "]}\n"
		if err := os.WriteFile(args[i], []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	check := func(target, out string) {
		got := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if len(got) != len(defineParityRows) {
			t.Fatalf("%s: %d lines for %d files:\n%s", target, len(got), len(defineParityRows), out)
		}
		for i, r := range defineParityRows {
			want := r.want
			if strings.HasPrefix(want, "rows[") {
				want = "error " + args[i] + ": " + want
			}
			if got[i] != want {
				t.Errorf("%s: %s:\n got %s\nwant %s", target, r.name, got[i], want)
			}
		}
	}
	check("go", runGo(t, []parityUnit{{pkg: p, goEmit: goEmit}}, "defines_main.go", args))
	writeFiles(t, dir, generate(t, p))
	copyFile(t, filepath.Join("testdata", "main", "defines_main.cpp"), filepath.Join(dir, "main.cpp"), same)
	for _, out := range buildAndRun(t, dir, []string{"main.cpp", "defs.gen.cpp"}, args...) {
		check("c++", out)
	}
}
