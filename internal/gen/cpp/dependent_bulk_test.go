package cppgen_test

import (
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// bakedDependentBulkPackage is bakedDependentPackage with lists of smallN elements: dependent
// values, dependent values whose branch is a define ref, and define refs (DECISIONS 392).
func bakedDependentBulkPackage() *ir.Package {
	p := bakedDependentPackage()
	event := p.Types[6].(*ir.Record)
	multi := p.Types[3].(*ir.Dependent)
	monsterRef := multi.Branches[1].Type
	event.Fields = append(event.Fields, field("multis", "multis", "", listOf(appOf(multi, "kind3"))), field("monsters", "monsters", "", listOf(monsterRef)))
	rec := p.Values[0].V.(*value.Record)
	t := recordType("Event", "active", "payload", "kind3", "multi", "et", "deep", "many", "multis", "monsters")
	many, keys := &value.List{}, &value.List{}
	for i := range smallN {
		many.Elems = append(many.Elems, member(i%2))
		keys.Elems = append(keys.Elems, key(fmt.Sprintf("MI_%c", 'A'+rune(i%2))))
	}
	rec.T = t
	rec.Fields[6] = many
	rec.Fields = append(rec.Fields, keys, keys)
	return p
}

// bakedDependentBulkWant is what testdata/main/baked_dependent_bulk_main.cpp prints.
const bakedDependentBulkWant = "many=8:0,1 multis=8:MI_A=7,MI_B=9 monsters=8:7,9\n"

// CODEGEN.md §5.6, §5.8, DECISIONS 392: a list of 8 dependent values, of define refs and of dependent define refs is filled from arrays and reads back.
func TestBakedDependentBulkCompilesAndRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, generate(t, bakedDependentBulkPackage()), "baked_dependent_bulk_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"demo.gen.cpp", "main.cpp"}) {
		if out != bakedDependentBulkWant {
			t.Errorf("got:\n%s\nwant:\n%s", out, bakedDependentBulkWant)
		}
	}
}
