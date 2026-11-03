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

// DECISIONS 383, 435, EVALUATION.md §11.3: an input enum holds no case per wire: it reads the enum's tables through ParseMode, whose first declared holder of a wire wins, and refuses a retired member; the smoke test runs the shared and the retired wires.
func TestInputEnumEqualWires(t *testing.T) {
	files := generateData(t, equalWirePkg())
	src := string(files["geninputsdup/out/go/geninputsdup.gen.go"])
	for _, wire := range []string{`"x"`, `"y"`, `"z"`} {
		if strings.Contains(src, "\tcase "+wire+":") {
			t.Errorf("the input enum has a case for the wire %s:\n%s", wire, src)
		}
	}
	if !strings.Contains(src, "ParseMode(") || !strings.Contains(src, ".Retired()") {
		t.Errorf("the input enum does not read ParseMode and Retired:\n%s", src)
	}
	runData(t, files, "geninputsdup/out/go", "testdata/smoke/inputs_equalwire_test.go", nil)
}
