package cppgen_test

import (
	"fmt"
	"math"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

const densePkg = "demo.dense"

var tI8 = ir.TypeRef{Kind: types.Int, Bits: 8, Signed: true}

// numbered is an @codes enum of the codes, its members m0, m1, ... named and wired alike.
func numbered(name string, codes *ir.TypeRef, values ...int64) *ir.Enum {
	e := &ir.Enum{Pkg: densePkg, Name: name, Codes: codes}
	for i, c := range values {
		m := fmt.Sprintf("m%d", i)
		e.Members = append(e.Members, &ir.EnumMember{Name: m, Wire: m, Index: i, Code: c})
	}
	return e
}

// denseEnums are the @codes enums of CODEGEN.md §5.2's dense form and its edges (DECISIONS 430): codes dense in declaration order from the signed minimum of a narrow type, from 6000, at the top and the bottom of the 64-bit range, from 0, one member, with wires that differ from the names; and the enums that keep the search: codes that wrap, codes with gaps, no member.
func denseEnums() []*ir.Enum {
	spelled := numbered("Spelled", &tU8, 1, 2)
	spelled.Members[0].Name, spelled.Members[0].Wire = "a", "x"
	spelled.Members[1].Name, spelled.Members[1].Wire = "b", "y"
	return []*ir.Enum{
		numbered("Level", &tI8, -128, -127, -126),
		numbered("Era", &tU16, 6000, 6001, 6002),
		numbered("Wide", &tInt, math.MaxInt64-1, math.MaxInt64),
		numbered("Floor", &tInt, math.MinInt64, math.MinInt64+1),
		numbered("Start", &tU8, 0, 1, 2, 3),
		numbered("Single", &tU8, 1),
		spelled,
		numbered("Gap", &tInt, math.MaxInt64, math.MinInt64),
		numbered("Skip", &tU8, 1, 3),
		numbered("Void", &tU8),
	}
}

// densePackage is demo.dense, a baked package of denseEnums.
func densePackage() *ir.Package {
	p := &ir.Package{
		Name: densePkg, Dir: "demo/dense",
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Dir: "demo/dense/out", Mode: ir.ModeBaked, Namespace: "demo::dense"}},
	}
	for _, e := range denseEnums() {
		p.Types = append(p.Types, e)
	}
	return p
}

// denseUsePackage is demo.denseuse, a baked package whose constexpr lookups take demo.dense's Era (dense from 6000) and Floor (dense from the minimum), so that its header reads detail::EraOrdinal and detail::FloorOrdinal of another package.
func denseUsePackage(dense *ir.Package) *ir.Package {
	enum := func(name string) []*ir.Param {
		for _, t := range dense.Types {
			if e := t.(*ir.Enum); e.Name == name {
				return []*ir.Param{{Name: "e", Type: ir.TypeRef{Kind: types.Enum, Named: e}}}
			}
		}
		return nil
	}
	return &ir.Package{
		Name: "demo.denseuse", Dir: "demo/denseuse",
		Fns: []*ir.ExportFn{
			lookup("weightOf", "", tInt, enum("Era"), num(4), num(1), num(9)),
			lookup("floorOf", "", tInt, enum("Floor"), num(-7), num(8)),
		},
		Imports: []*ir.PackageRef{{Name: dense.Name, Dir: dense.Dir, Emits: dense.Emits}},
		Emits:   []*ir.Emit{{Target: ir.TargetCpp, Dir: "demo/denseuse/out", Mode: ir.ModeBaked, Namespace: "demo::denseuse"}},
	}
}
