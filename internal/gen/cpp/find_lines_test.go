package cppgen_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	linesPkg    = "find.lines"
	linesStride = 7919 // prime: row i has code (i*stride) mod n, none in key order
)

// linesCase is one container of the test: its lookups' names and the tables each one reads, written once.
type linesCase struct {
	name    string
	lookups []string
	tables  []string
}

var linesCases = []linesCase{
	{name: "table", lookups: []string{"Slots::FindByCode", "Slots::FindByTag"}, tables: []string{"kSlotsFindByCodeKeys", "kSlotsFindByCodeRows", "kSlotsFindByTagKeys", "kSlotsFindByTagRows"}},
	{name: "keyed", lookups: []string{"Items::Find", "Items::FindByTag"}, tables: []string{"kItemsFindKeys", "kItemsFindRows", "kItemsFindByTagKeys", "kItemsFindByTagRows"}},
}

// linesPackage is a baked package of two containers of n rows: a table Slots with @stable fields code (UInt16, a permutation of the rows) and tag (String), and a keyed list Items by key (Int), @stable tag (String).
func linesPackage(n int) *ir.Package {
	code, tag := field("code", "code", "", tU16), field("tag", "tag", "", tString)
	code.Stable, tag.Stable = true, true
	slot := &ir.Record{Pkg: linesPkg, Name: "Slot", Fields: []*ir.Field{code, tag}}
	itemTag := field("tag", "tag", "", tString)
	itemTag.Stable = true
	item := &ir.Record{Pkg: linesPkg, Name: "Item", Fields: []*ir.Field{field("key", "key", "", tInt), itemTag}}
	ids, slots, items := make([]string, n), make([]*value.Record, n), make([]*value.Record, n)
	for i := range n {
		ids[i] = fmt.Sprintf("slot_%05d", i)
		tagText := str(fmt.Sprintf("t%05d", (i*linesStride)%n))
		slots[i] = &value.Record{T: bulkTypeOf(linesPkg, "Slot", slot), Fields: []value.Value{num(int64(i*linesStride) % int64(n)), tagText}, Ident: &value.Identity{Coll: &types.Collection{Kind: types.CollLet, Pkg: linesPkg, Name: "slots"}, Key: value.Key{S: ids[i]}}}
		items[i] = &value.Record{T: bulkTypeOf(linesPkg, "Item", item), Fields: []value.Value{num(int64(i*linesStride) % int64(n)), tagText}, Ident: &value.Identity{Coll: &types.Collection{Kind: types.CollLet, Pkg: linesPkg, Name: "items"}, Key: intKey(int64(i*linesStride) % int64(n))}}
	}
	slotElem := ir.TypeRef{Kind: types.Record, Named: slot}
	return &ir.Package{
		Name: linesPkg, Dir: "find/lines", Types: []ir.Type{slot, item},
		Values: []*ir.Value{
			{Name: "slots", IDs: ids, Type: ir.TypeRef{Kind: types.Table, Elem: &slotElem}, V: &value.Table{Entries: slots}},
			findList("items", item, "key", items...),
		},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "find/lines/out", Mode: ir.ModeBaked, Namespace: "find::lines"}},
	}
}

// lookupBodies are the line counts of the bodies of the generated functions named in names, in that order; a missing name is an error.
func lookupBodies(t *testing.T, files []ir.File, names []string) []int {
	t.Helper()
	got := map[string]int{}
	for _, f := range bodies(t, files) {
		got[f.Name] = f.Body
	}
	out := make([]int, len(names))
	for i, name := range names {
		body, ok := got[name]
		if !ok {
			t.Fatalf("no function %s in the generated source", name)
		}
		out[i] = body
	}
	return out
}

// CODEGEN.md §2.11, §5.9, DECISIONS 432, 435: Find of a keyed list and FindBy<F> of a table or keyed list hold a constant number of lines whatever the rows; their sorted keys and rows' positions are k<C>Find[By<F>]Keys and ...Rows tables of the .gen.cpp's unnamed namespace, written once.
func TestFindConstantLines(t *testing.T) {
	const small, large = 3, 3000
	for _, tc := range linesCases {
		t.Run(tc.name, func(t *testing.T) {
			got := map[int][]int{}
			for _, n := range []int{small, large} {
				files := generate(t, linesPackage(n))
				got[n] = lookupBodies(t, files, tc.lookups)
				source := fileOf(t, files, ".gen.cpp")
				for _, table := range tc.tables {
					if strings.Count(source, " "+table+" = ") != 1 {
						t.Errorf("n=%d: %s is not written once", n, table)
					}
				}
			}
			if !slices.Equal(got[small], got[large]) {
				t.Errorf("%v: %v lines for %d rows, %v for %d: want a constant number", tc.lookups, got[small], small, got[large], large)
			}
		})
	}
}
