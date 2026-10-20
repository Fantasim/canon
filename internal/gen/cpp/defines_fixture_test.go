package cppgen_test

import (
	"math"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const definesPkg = "demo.defs"

// definesEnum is an enum of definesPkg with @cpp(defines:) prefix at the declaration position order.
func definesEnum(name, prefix string, order int, codes *ir.TypeRef, ms []*ir.EnumMember) *ir.Enum {
	return &ir.Enum{Pkg: definesPkg, Name: name, Codes: codes, CppDefines: prefix, HasCppDefines: true, Order: order, Members: ms}
}

// definesConst is a constant of definesPkg with @cpp(defines:) prefix, at the declaration position order; a nil prefix has none.
func definesConst(name string, t ir.TypeRef, v value.Value, prefix *string, order int) *ir.Const {
	c := &ir.Const{Name: name, Type: t, V: v, Order: order}
	if prefix != nil {
		c.CppDefines, c.HasCppDefines = *prefix, true
	}
	return c
}

// definesPackage is demo.defs in types mode: the defines header of CODEGEN.md §7.9 mixing every allowed kind in declaration order (DECISIONS 389, 390): an @codes enum with a retired member, a constant, an enum with indexes, constants of an integer (negative, INT64_MIN, INT64_MAX), a @codes(Int) enum holding INT64_MIN, a Bool true and false, a String needing escapes, an enum member (its code), an enum under the prefix `_`; and a constant without @cpp(defines:), which stays out.
func definesPackage() *ir.Package {
	wide := sized(32, false)
	none, on := "", ""
	kind := definesEnum("Kind", "IK1_", 0, &wide, members([]string{"IK1_WEAPON", "IK1_WEAPON", "GENERAL", "GENERAL", "OLD", "OLD"}, []int64{1, 4, 9}))
	kind.Members[2].Retired = true
	plain := definesEnum("Plain", "P_", 2, nil, members([]string{"A", "a", "B", "b"}, nil))
	extreme := definesEnum("Extreme", "X_", 6, &tInt, members([]string{"LOW", "LOW", "HIGH", "HIGH"}, []int64{math.MinInt64, math.MaxInt64}))
	legacy := definesEnum("Legacy", "_", 11, nil, members([]string{"NONE", "NONE"}, nil))
	consts := []*ir.Const{
		definesConst("MAX_ITEM_KIND3", tInt, &value.Int{V: 133}, &none, 1),
		definesConst("NEGATIVE", tInt, &value.Int{V: -5}, &none, 3),
		definesConst("LOWEST", tInt, &value.Int{V: math.MinInt64}, &none, 4),
		definesConst("HIGHEST", tInt, &value.Int{V: math.MaxInt64}, &none, 5),
		definesConst("ENABLED", tBool, &value.Bool{V: true}, &on, 7),
		definesConst("DISABLED", tBool, &value.Bool{V: false}, &on, 8),
		definesConst("TITLE", tString, &value.Str{V: "say \"hi\"\n\\"}, &on, 9),
		definesConst("DEFAULT_KIND", ir.TypeRef{Kind: types.Enum, Named: kind}, &value.Member{Index: 1}, &on, 10),
		definesConst("PLAIN", tInt, &value.Int{V: 7}, nil, 12),
	}
	return &ir.Package{
		Name: definesPkg, Dir: "demo/defs", Types: []ir.Type{kind, plain, extreme, legacy}, Consts: consts,
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Dir: "out/defs", Mode: ir.ModeTypes, Namespace: "demo::defs"}},
	}
}
