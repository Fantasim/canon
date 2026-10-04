package gogen_test

import (
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// defineDependentPkg is dependentPkg whose branch two refs the load.defines table monsters (DECISIONS 298).
func defineDependentPkg(mode ir.Mode) *ir.Package {
	p := dependentPkg()
	d := p.Types[1].(*ir.Dependent)
	d.Branches[1].Type = ir.TypeRef{Kind: types.Ref, Key: &strT, Ref: &ir.RefTarget{Coll: types.CollDefines, Pkg: "dep", Value: "monsters", Local: true}}
	d.ByMember = []int{0, 1, ir.NoBranch}
	p.Defines = []*ir.DefineTable{{Pkg: "dep", Value: "monsters", Names: []string{"MI_A", "MI_B"}, Values: []int64{7, 9}}}
	p.Emits[0].Mode = mode
	if mode != ir.ModeBaked {
		return p
	}
	thingT := &types.RecordType{Pkg: "dep", Name: "Thing", Fields: []*types.Field{{Name: "k"}, {Name: "p"}}}
	coll := &types.Collection{Kind: types.CollLet, Pkg: "dep", Name: "things"}
	row := func(id string, k int, v value.Value) *value.Record {
		return &value.Record{T: thingT, Fields: []value.Value{&value.Member{Index: k}, v}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: id}}}
	}
	p.Values[0].IDs = []string{"a", "b"}
	p.Values[0].V = &value.Table{Entries: []*value.Record{row("a", 0, &value.Int{V: 42}), row("b", 1, &value.Ref{Key: value.Key{S: "MI_B"}})}}
	return p
}

// CODEGEN.md §5.6, §5.8, DECISIONS 298: a define branch's As<Branch>Value, its key looked up as the loader reads it (a missing define is the load error), and baked.
func TestDependentDefineGoldens(t *testing.T) {
	files := generateData(t, defineDependentPkg(ir.ModeData))
	checkGoldens(t, files, sortedPaths(files), filepath.Join(dataGoldens, "dependentdefine"))
	baked := generateData(t, defineDependentPkg(ir.ModeBaked))
	checkGoldens(t, baked, sortedPaths(baked), filepath.Join(dataGoldens, "dependentdefinebaked"))
}

// CODEGEN.md §5.6, §5.8, DECISIONS 298: the data loader reads a define branch's value and refuses a define the table lacks, with §5.8's text.
func TestDependentDefineDataRuns(t *testing.T) {
	data := map[string][]byte{
		"good/things.json": []byte(`{"$schema": "dep.Thing@00000001", "rows": [{"$id": "a", "k": "one", "p": 42}, {"$id": "b", "k": "two", "p": "MI_B"}]}`),
		"bad/things.json":  []byte(`{"$schema": "dep.Thing@00000001", "rows": [{"$id": "c", "k": "two", "p": "MI_Z"}]}`),
	}
	runData(t, generateData(t, defineDependentPkg(ir.ModeData)), "dep/out/go", "testdata/smoke/dependent_define_test.go", data)
}

// CODEGEN.md §5.6, DECISIONS 298: a baked define branch holds its key and its value.
func TestDependentDefineBakedRuns(t *testing.T) {
	runData(t, generateData(t, defineDependentPkg(ir.ModeBaked)), "dep/out/go", "testdata/smoke/dependent_define_baked_test.go", nil)
}
