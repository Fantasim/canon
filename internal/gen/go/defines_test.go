package gogen_test

import (
	"errors"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// definesPkg is a table hits of Hit refing the define table monsters four ways (CODEGEN.md §5.8); baked holds rows a, b.
func definesPkg(pkg string, mode ir.Mode) *ir.Package {
	mon := ir.TypeRef{Kind: types.Ref, Key: &strT, Ref: &ir.RefTarget{Coll: types.CollDefines, Pkg: pkg, Value: "monsters", Local: true}}
	hit := &ir.Record{Pkg: pkg, Name: "Hit", Fields: []*ir.Field{
		wired("monster", "monster", "", mon), opt(wired("alt", "alt", "", mon)),
		wired("all", "all", "", listT(mon)), opt(wired("maybe", "maybe", "", listT(mon))),
	}}
	hitT := &types.RecordType{Pkg: pkg, Name: "Hit"}
	for i, n := range []string{"monster", "alt", "all", "maybe"} {
		hitT.Fields = append(hitT.Fields, &types.Field{Name: n, Index: i})
	}
	key := func(k string) value.Value { return &value.Ref{Key: value.Key{S: k}} }
	keys := func(ks ...string) value.Value {
		l := &value.List{}
		for _, k := range ks {
			l.Elems = append(l.Elems, key(k))
		}
		return l
	}
	row := func(id string, fields ...value.Value) *value.Record {
		return &value.Record{T: hitT, Ident: &value.Identity{Key: value.Key{S: id}}, Fields: fields}
	}
	elem := ir.TypeRef{Kind: types.Record, Named: hit}
	hits := &ir.Value{Name: "hits", Schema: pkg + ".Hit@00000001", IDs: []string{"a", "b"}, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}}
	if mode == ir.ModeBaked {
		hits.V = &value.Table{Entries: []*value.Record{
			row("a", key("MI_A"), key("MI_B"), keys("MI_A", "MI_B"), &value.None{}),
			row("b", key("MI_B"), &value.None{}, keys(), keys("MI_A")),
		}}
	}
	e := &ir.Emit{Target: ir.TargetGo, Out: "out/go/", Dir: pkg + "/out/go", GoImport: dataModule + "/" + pkg + "/out/go", Mode: mode, GoPackage: pkg}
	return &ir.Package{
		Name: pkg, Dir: pkg, Types: []ir.Type{hit}, Values: []*ir.Value{hits}, Emits: []*ir.Emit{e},
		Defines: []*ir.DefineTable{{Pkg: pkg, Value: "monsters", Names: []string{"MI_A", "MI_B"}, Values: []int64{7, 9}}},
	}
}

// CODEGEN.md §5.8, decision 222: a baked emit writes each define's value beside its key.
func TestDefinesBakedCompiles(t *testing.T) {
	runData(t, generateData(t, definesPkg("defsbaked", ir.ModeBaked)), "defsbaked/out/go", "testdata/smoke/defines_baked_test.go", nil)
}

// CODEGEN.md §5.8: the data loader looks each key up in defines<Table> and fails on a name it lacks.
func TestDefinesDataCompiles(t *testing.T) {
	data := map[string][]byte{
		"good/hits.json": []byte(`{"$schema": "defsdata.Hit@00000001", "rows": [
  {"$id": "a", "monster": "MI_A", "alt": "MI_B", "all": ["MI_A", "MI_B"], "maybe": null},
  {"$id": "b", "monster": "MI_B", "alt": null, "all": [], "maybe": ["MI_A"]}
]}
`),
		"bad/hits.json": []byte(`{"$schema": "defsdata.Hit@00000001", "rows": [
  {"$id": "a", "monster": "MI_A", "alt": null, "all": ["MI_B", "MI_C"], "maybe": null}
]}
`),
	}
	runData(t, generateData(t, definesPkg("defsdata", ir.ModeData)), "defsdata/out/go", "testdata/smoke/defines_data_test.go", data)
}

// CODEGEN.md §5.8: a ref into a define table the IR does not hold is malformed IR.
func TestDefineTableMissing(t *testing.T) {
	p := definesPkg("defs", ir.ModeBaked)
	p.Defines = nil
	if err := generateErr(p, nil); !errors.Is(err, gogen.ErrMalformed) {
		t.Errorf("got %v, want ErrMalformed", err)
	}
}
