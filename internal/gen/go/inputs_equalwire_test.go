package gogen_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// equalWirePkg is one input record whose enum has members on equal wires: first and second on x, third on y, a retired old and then new on z.
func equalWirePkg() *ir.Package {
	e := &ir.Enum{Pkg: "geninputsdup", Name: "Mode", Members: []*ir.EnumMember{
		{Name: "first", Wire: "x", Index: 0},
		{Name: "second", Wire: "x", Index: 1},
		{Name: "third", Wire: "y", Index: 2},
		{Name: "old", Wire: "z", Index: 3, Retired: true},
		{Name: "new", Wire: "z", Index: 4},
	}}
	rec := &ir.Record{Pkg: "geninputsdup", Name: "Gen", Fields: []*ir.Field{
		{Name: "mode", Type: ir.TypeRef{Kind: types.Enum, Named: e}, Optional: true, Input: &types.Input{Env: "GENINPUTSDUP_MODE"}},
	}}
	return &ir.Package{
		Name: "geninputsdup", Dir: "geninputsdup", Types: []ir.Type{e, rec},
		Emits: []*ir.Emit{{
			Target: ir.TargetGo, Out: "out/go/", Dir: "geninputsdup/out/go",
			GoImport: dataModule + "/geninputsdup/out/go", Mode: ir.ModeBaked, GoPackage: "geninputsdup",
		}},
	}
}

// DECISIONS 383, 393, EVALUATION.md §11.3: an input enum's switch holds each wire once, the first declared member wins, and a retired first holder leaves its wire refused.
func TestInputEnumEqualWires(t *testing.T) {
	files := generateData(t, equalWirePkg())
	src := string(files["geninputsdup/out/go/geninputsdup.gen.go"])
	for _, wire := range []string{`"x"`, `"y"`} {
		if n := strings.Count(src, "\tcase "+wire+":"); n != 1 {
			t.Errorf("case %s appears %d times in the input switch, want 1:\n%s", wire, n, src)
		}
	}
	if strings.Contains(src, "\tcase \"z\":") {
		t.Errorf("a retired first holder's wire z has a case:\n%s", src)
	}
	runData(t, files, "geninputsdup/out/go", "testdata/smoke/inputs_equalwire_test.go", nil)
}
