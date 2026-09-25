package gogen_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

const inputsGoldens = "testdata/inputs"

// inputEnv is a distinct environment variable name per package, so the baked and data fixtures
// (run in the same test binary) never share one variable.
func inputEnv(pkg, name string) string { return strings.ToUpper(pkg) + "_" + name }

// inputsRecord is Gen, one input field of every kind EVALUATION.md §11.1 allows, plus a required one.
func inputsRecord(pkg string, e *ir.Enum) *ir.Record {
	env := func(name string) string { return inputEnv(pkg, name) }
	return &ir.Record{Pkg: pkg, Name: "Gen", Fields: []*ir.Field{
		{Name: "flag", Type: boolT, Optional: true, Input: &types.Input{Env: env("FLAG")}},
		{Name: "count", Type: i8T, Optional: true, Input: &types.Input{Env: env("COUNT")}},
		{
			Name: "ratio", Type: fltT, Optional: true, Input: &types.Input{Env: env("RATIO")},
			Range: &types.Bound{HasLo: true, HasHi: true, HiIncluded: true, Hi: types.Limit{F: 1}},
		},
		{
			Name: "span", Type: durT, Optional: true, Input: &types.Input{Env: env("SPAN")},
			Range: &types.Bound{HasHi: true, HiIncluded: true, Hi: types.Limit{I: 60_000}},
		},
		{
			Name: "name", Type: strT, Optional: true, Input: &types.Input{Env: env("NAME")},
			Range: &types.Bound{HasLo: true, Lo: types.Limit{I: 1}, HasHi: true, HiIncluded: true, Hi: types.Limit{I: 5}},
		},
		{
			Name: "code", Type: strT, Optional: true, Input: &types.Input{Env: env("CODE")},
			Pattern: regexp.MustCompile("^[A-Z]+$"),
		},
		{
			Name: "scale", Type: f32T, Optional: true, Input: &types.Input{Env: env("SCALE")},
			Range: &types.Bound{HasLo: true, Lo: types.Limit{F: 0}, HasHi: true, HiIncluded: true, Hi: types.Limit{F: 10}},
		},
		{Name: "mode", Type: ir.TypeRef{Kind: types.Enum, Named: e}, Optional: true, Input: &types.Input{Env: env("MODE")}},
		{Name: "required", Type: intT, Input: &types.Input{Env: env("REQUIRED")}},
	}}
}

func inputsEnum(pkg string) *ir.Enum {
	return &ir.Enum{Pkg: pkg, Name: "Mode", Members: []*ir.EnumMember{
		{Name: "fast", Wire: "fast", Index: 0},
		{Name: "slow", Wire: "slow", Index: 1},
		{Name: "old", Wire: "old", Index: 2, Retired: true},
	}}
}

// inputsPkg is a package of one record with input fields, no emitted value (CODEGEN.md §5.12).
func inputsPkg(name string, mode ir.Mode) *ir.Package {
	e := inputsEnum(name)
	rec := inputsRecord(name, e)
	return &ir.Package{
		Name: name, Dir: name, Types: []ir.Type{e, rec},
		Emits: []*ir.Emit{{
			Target: ir.TargetGo, Out: "out/go/", Dir: name + "/out/go",
			GoImport: dataModule + "/" + name + "/out/go", Mode: mode, GoPackage: name,
		}},
	}
}

// CODEGEN.md §5.12.
func TestInputsGolden(t *testing.T) {
	files := generateData(t, inputsPkg("geninputsbaked", ir.ModeBaked), inputsPkg("geninputsdata", ir.ModeData))
	checkGoldens(t, files, sortedPaths(files), inputsGoldens)
}

// EVALUATION.md §11.3.
func TestInputsBakedCompiles(t *testing.T) {
	files := generateData(t, inputsPkg("geninputsbaked", ir.ModeBaked))
	runData(t, files, "geninputsbaked/out/go", "testdata/smoke/inputs_baked_test.go", nil)
}

// CODEGEN.md §5.12.
func TestInputsDataCompiles(t *testing.T) {
	files := generateData(t, inputsPkg("geninputsdata", ir.ModeData))
	runData(t, files, "geninputsdata/out/go", "testdata/smoke/inputs_data_test.go", nil)
}

// CODEGEN.md §5.12: input_<T>_<store> keeps Gen.flagX and GenFlag.x apart.
func TestInputsCollisionFreeNames(t *testing.T) {
	gen := &ir.Record{Pkg: "collide", Name: "Gen", Fields: []*ir.Field{
		{Name: "flagX", Type: boolT, Optional: true, Input: &types.Input{Env: "COLLIDE_FLAGX"}},
	}}
	genFlag := &ir.Record{Pkg: "collide", Name: "GenFlag", Fields: []*ir.Field{
		{Name: "x", Type: boolT, Optional: true, Input: &types.Input{Env: "COLLIDE_X"}},
	}}
	p := &ir.Package{
		Name: "collide", Dir: "collide", Types: []ir.Type{gen, genFlag},
		Emits: []*ir.Emit{{
			Target: ir.TargetGo, Out: "out/go/", Dir: "collide/out/go",
			GoImport: dataModule + "/collide/out/go", Mode: ir.ModeBaked, GoPackage: "collide",
		}},
	}
	files := generateData(t, p)
	src := string(files["collide/out/go/collide.gen.go"])
	for _, want := range []string{"input_Gen_flagX", "input_GenFlag_x"} {
		if !strings.Contains(src, want) {
			t.Errorf("want %s in generated source:\n%s", want, src)
		}
	}
}
