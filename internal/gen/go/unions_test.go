package gogen_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const unionsGoldens = "testdata/unions"

// unionStatus is enum Status { open, closed }, the checker arm of a literal union.
func unionStatus(pkg string) *ir.Enum {
	return &ir.Enum{Pkg: pkg, Name: "Status", Members: []*ir.EnumMember{
		{Name: "open", Wire: "open", Index: 0}, {Name: "closed", Wire: "closed", Index: 1},
	}}
}

// unionItemType is the table entry's type twin, the shape g.fieldValue needs to read a baked
// literal's field by name (fixture_values_test.go's w.record does the same for JSON fixtures).
func unionItemType(pkg string) *types.RecordType {
	return &types.RecordType{Pkg: pkg, Name: "Item", Fields: []*types.Field{{Name: "status", Index: 0, Wire: "status", WirePath: []string{"status"}}}}
}

// unionPkg is one table whose field is a literal union over Status, "unknown" (CODEGEN.md §4.1).
func unionPkg(name string, mode ir.Mode) *ir.Package {
	e := unionStatus(name)
	enumT := ir.TypeRef{Kind: types.Enum, Named: e}
	litT := ir.TypeRef{Kind: types.LitUnion, Elem: &enumT, Literals: []string{"unknown"}}
	item := &ir.Record{Pkg: name, Name: "Item", Fields: []*ir.Field{
		{Name: "status", WirePath: []string{"status"}, Type: litT},
	}}
	itemT := unionItemType(name)
	table := ir.TypeRef{Kind: types.Table, Elem: &ir.TypeRef{Kind: types.Record, Named: item}}
	entry := func(id, status string) *value.Record {
		return &value.Record{T: itemT, Ident: &value.Identity{Key: value.Key{S: id}}, Fields: []value.Value{&value.Str{V: status}}}
	}
	p := &ir.Package{Name: name, Dir: name, Types: []ir.Type{e, item}}
	p.Values = []*ir.Value{{
		Name: "items", Type: table, IDs: []string{"a", "b"},
		V: &value.Table{Entries: []*value.Record{entry("a", "open"), entry("b", "unknown")}},
	}}
	p.Emits = []*ir.Emit{{
		Target: ir.TargetGo, Out: "out/go/", Dir: name + "/out/go",
		GoImport: dataModule + "/" + name + "/out/go", Mode: mode, GoPackage: name,
	}}
	return p
}

// CODEGEN.md §4.1.
func TestUnionsGolden(t *testing.T) {
	files := generateData(t, unionPkg("genunionbaked", ir.ModeBaked), unionPkg("genuniondata", ir.ModeData))
	checkGoldens(t, files, sortedPaths(files), unionsGoldens)
}

// WIRE.md §5.9: the checker's arm and the literal arm both read back as their wire text.
func TestUnionsBakedCompiles(t *testing.T) {
	files := generateData(t, unionPkg("genunionbaked", ir.ModeBaked))
	runData(t, files, "genunionbaked/out/go", "testdata/smoke/unions_baked_test.go", nil)
}
