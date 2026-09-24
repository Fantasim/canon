package cppgen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// severalHoldersFixture is Item, held by two @reload keyed lists, with a ref into "left" (CODEGEN.md §5.8, §5.11).
func severalHoldersFixture() *ir.Package {
	const pkg = "demo.holders"
	rec := &ir.Record{Pkg: pkg, Name: "Item", Fields: []*ir.Field{field("a", "a", "", tString)}}
	key := tString
	ref := ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: pkg, Value: "left", Elem: rec, Keyed: true}}
	rec.Fields = append(rec.Fields, field("peer", "peer", "", ref))
	elem := ir.TypeRef{Kind: types.Record, Named: rec}
	value := func(name string) *ir.Value {
		return &ir.Value{Name: name, Reload: true, Schema: "demo.holders.Item@00000001", Type: ir.TypeRef{
			Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "a", WirePath: []string{"a"}},
		}}
	}
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "demo/holders/out", Mode: ir.ModeData, Namespace: "demo::holders"}
	return &ir.Package{
		Name: pkg, Dir: "demo/holders", Types: []ir.Type{rec},
		Values: []*ir.Value{value("left"), value("right")}, Emits: []*ir.Emit{emit},
	}
}

// TestSeveralHoldersCompileAndRun: a good file resolves the ref, a bad key refuses the Reload (CODEGEN.md §5.8, §5.11).
func TestSeveralHoldersCompileAndRun(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, generate(t, severalHoldersFixture()), "severalholders_main.cpp")
	for _, sub := range []string{"good", "bad"} {
		entries, err := os.ReadDir(filepath.Join("testdata", "main", "severalholders", sub))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			from := filepath.Join("testdata", "main", "severalholders", sub, e.Name())
			copyFile(t, from, filepath.Join(dir, sub, e.Name()), same)
		}
	}
	outs := buildAndRun(t, dir, []string{"holders.gen.cpp", "main.cpp"}, dir)
	for _, out := range outs {
		if !strings.Contains(out, "failures: 0\n") {
			t.Errorf("driver output:\n%s", out)
		}
		if !strings.Contains(out, "no entry nope") {
			t.Errorf("bad key refusal:\n%s\nwant \"no entry nope\"", out)
		}
	}
}
