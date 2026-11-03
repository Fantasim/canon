package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// lookupCodes are the codes of a three-member enum: dense from 5 in declaration order, or sparse and out of it.
var lookupCodes = map[bool][]int64{true: {5, 6, 7}, false: {30, 10, 20}}

// codesEnum is an enum of package pkg with @codes (UInt16) and its members' codes; dense picks lookupCodes.
func codesEnum(pkg, name string, dense bool) *ir.Enum {
	codes := ir.TypeRef{Kind: types.Int, Bits: 16}
	e := &ir.Enum{Pkg: pkg, Name: name, Codes: &codes}
	for i, code := range lookupCodes[dense] {
		e.Members = append(e.Members, &ir.EnumMember{Name: string(rune('a' + i)), Wire: string(rune('a' + i)), Index: i, Code: code})
	}
	return e
}

// lookupOver is a package fn weigh<Name>(e Name) Int whose three cells are its members' positions.
func lookupOver(e *ir.Enum, name string) *ir.ExportFn {
	twin := &types.EnumType{Pkg: e.Pkg, Name: e.Name}
	table := &ir.LookupTable{Domains: [][]value.Value{nil}}
	for i, m := range e.Members {
		twin.Members = append(twin.Members, &types.Member{Name: m.Name, Wire: m.Wire, Index: i, Code: m.Code})
		table.Domains[0] = append(table.Domains[0], &value.Member{Enum: twin, Index: i})
		table.Cells = append(table.Cells, &value.Int{V: int64(i), T: types.IntType})
	}
	return &ir.ExportFn{
		Name: name, Kind: ir.FnLookup, Result: ir.TypeRef{Kind: types.Int, Bits: 64, Signed: true},
		Params: []*ir.Param{{Name: "e", Type: ir.TypeRef{Kind: types.Enum, Named: e}}}, Table: table,
	}
}

// CODEGEN.md §3.5, §5.10, DECISIONS 435: the tables a lookup searches an enum's index in are in the plan, and only the plan's: the plan of a package with lookups over own and imported enums, dense and sparse, holds exactly the names gen/go writes (a dense closed enum has no table).
func TestGoNamePlanMatchesLookups(t *testing.T) {
	sparse, dense := codesEnum("p", "Sparse", false), codesEnum("p", "Dense", true)
	own := goPackage(&ir.Package{
		Types: []ir.Type{sparse, dense},
		Fns:   []*ir.ExportFn{lookupOver(sparse, "weighSparse"), lookupOver(dense, "weighDense")},
	})
	far := codesEnum("far", "Far", false)
	imported := goPackage(&ir.Package{
		Imports: []*ir.PackageRef{{Name: "far", Dir: "far", Emits: []*ir.Emit{{
			Target: ir.TargetGo, Out: "out/go", Dir: "far/out/go", GoImport: "example.com/far", Mode: ir.ModeBaked, GoPackage: "far",
		}}}},
		Fns: []*ir.ExportFn{lookupOver(far, "weighFar")},
	})
	for name, p := range map[string]*ir.Package{"own enums": own, "imported enum": imported} { //canon:unordered each package compared alone
		comparePlan(t, name, p, p.Emits[0])
	}
	pl := ir.PlanGoNames(own, own.Emits[0])
	if _, ok := pl.LookupTables(sparse); !ok {
		t.Error("a sparse enum has no lookup tables in the plan")
	}
	if _, ok := pl.LookupTables(dense); ok {
		t.Error("a dense closed enum has lookup tables in the plan")
	}
	if t2, _ := ir.PlanGoNames(imported, imported.Emits[0]).LookupTables(far); t2.Keys != "lookupKeys_far_Far" || t2.At != "lookupAt_far_Far" {
		t.Errorf("an imported enum's tables are %+v", t2)
	}
}
