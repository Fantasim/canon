package cppgen_test

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// inputsPackage is a record Config with an input field per accepted type and refinement (CODEGEN.md §5.12; EVALUATION.md §11.1).
func inputsPackage() *ir.Package {
	color := enumOf("Color", "red", "green", "blue")
	color.Members[2].Retired = true
	codeType := ir.TypeRef{Kind: types.Int, Bits: 8, Signed: false}
	level := enumOf("Level", "low", "mid", "high")
	level.JSONCodes, level.Codes, level.Cpp.Name = true, &codeType, "Tier"
	for i, m := range level.Members {
		m.Code = int64(i + 1)
	}
	rec := &ir.Record{Pkg: "demo", Name: "Config", Fields: []*ir.Field{
		input("port", "CANON_TEST_PORT", tInt32, false, bound(1, 65_535)),
		input("debug", "CANON_TEST_DEBUG", tBool, false, nil),
		input("name", "CANON_TEST_NAME", tString, true, bound(1, 10)),
		input("tag", "CANON_TEST_TAG", tString, true, nil),
		input("timeout", "CANON_TEST_TIMEOUT", tDuration, true, bound(1_000, 60_000)),
		input("rate", "CANON_TEST_RATE", tFloat32, true, &types.Bound{Hi: types.Limit{F: 1}, HasLo: true, HasHi: true, HiIncluded: true}),
		input("color", "CANON_TEST_COLOR", ir.TypeRef{Kind: types.Enum, Named: color}, true, nil),
		input("level", "CANON_TEST_LEVEL", ir.TypeRef{Kind: types.Enum, Named: level}, true, nil),
		input("note", "CANON_TEST_NOTE", tString, true, nil),
		input("ratio", "CANON_TEST_RATIO", tFloat, true, nil),
		input("count", "CANON_TEST_COUNT", tInt, true, nil),
		input("wait", "CANON_TEST_WAIT", tDuration, true, nil),
	}}
	rec.Fields[3].Pattern = regexp.MustCompile(`^[A-Z]{3}$`)
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "demo/out", Mode: ir.ModeData, Namespace: "demo"}
	return &ir.Package{Name: "demo", Dir: "demo", Types: []ir.Type{color, level, rec}, Emits: []*ir.Emit{emit}}
}

// input is an input field read from env.
func input(name, env string, t ir.TypeRef, optional bool, r *types.Bound) *ir.Field {
	return &ir.Field{Name: name, WirePath: []string{name}, Type: t, Optional: optional, Range: r, Input: &types.Input{Env: env}}
}

// bound is the integer range lo..=hi.
func bound(lo, hi int64) *types.Bound {
	return &types.Bound{Lo: types.Limit{I: lo}, Hi: types.Limit{I: hi}, HasLo: true, HasHi: true, HiIncluded: true}
}
