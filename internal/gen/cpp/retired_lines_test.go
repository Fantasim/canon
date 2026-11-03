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
	retiredPkg    = "ret.lines"
	retiredEach   = 3 // every third member, case and entry is retired
	codeStride    = 7 // the gap between the codes of the sparse enums
	isRetiredName = "IsRetired"
	fromCodeName  = "FromCode"
	wantIsRetired = 5 // Many, Dense, Sparse, ShapeKind and SlotId
	wantFromCode  = 2 // Dense and Sparse
)

var tI32 = ir.TypeRef{Kind: types.Int, Bits: 32, Signed: true}

// retiredEnum is an enum of n members, every retiredEach-th retired, whose codes are code(i) with @codes (nil codes: none).
func retiredEnum(name string, n int, codes *ir.TypeRef, code func(i int) int64) *ir.Enum {
	members := make([]*ir.EnumMember, n)
	for i := range members {
		w := fmt.Sprintf("%s_%05d", name, i)
		members[i] = &ir.EnumMember{Name: w, Wire: w, Index: i, Retired: i%retiredEach == 0}
		if codes != nil {
			members[i].Code = code(i)
		}
	}
	return &ir.Enum{Pkg: retiredPkg, Name: name, Codes: codes, Members: members}
}

// retiredVariant is a variant of n cases, every retiredEach-th retired, whose kind enum is ShapeKind.
func retiredVariant(n int) *ir.Variant {
	v := &ir.Variant{Pkg: retiredPkg, Name: "Shape", Tag: "kind"}
	for i := range n {
		w := fmt.Sprintf("shape_%05d", i)
		v.Cases = append(v.Cases, &ir.Case{Name: w, Wire: w, Retired: i%retiredEach == 0})
	}
	return v
}

// retiredPackage is a baked package with a closed enum, two closed @codes enums (dense and sparse), a variant and a table, each of n members, some retired: their IsRetired and FromCode bodies must not grow with n.
func retiredPackage(n int) *ir.Package {
	ids := make([]string, n)
	rec := &ir.Record{Pkg: retiredPkg, Name: "Slot", Fields: []*ir.Field{field("n", "n", "", tInt)}}
	rt := bulkTypeOf(retiredPkg, "Slot", rec)
	coll := &types.Collection{Kind: types.CollLet, Pkg: retiredPkg, Name: "slots"}
	rows := make([]*value.Record, n)
	for i := range rows {
		ids[i] = fmt.Sprintf("slot_%05d", i)
		rows[i] = &value.Record{T: rt, Fields: []value.Value{num(int64(i))}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: ids[i]}, Retired: i%retiredEach == 0}}
	}
	elem := ir.TypeRef{Kind: types.Record, Named: rec}
	return &ir.Package{
		Name: retiredPkg, Dir: "ret/lines",
		Types: []ir.Type{
			retiredEnum("Many", n, nil, nil),
			retiredEnum("Dense", n, &tI32, func(i int) int64 { return int64(i) }),
			retiredEnum("Sparse", n, &tI32, func(i int) int64 { return int64(i * codeStride) }),
			retiredVariant(n), rec,
		},
		Values: []*ir.Value{{Name: "slots", IDs: ids, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}, V: &value.Table{Entries: rows}}},
		Emits:  []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "ret/lines/out", Mode: ir.ModeBaked, Namespace: "ret::lines"}},
	}
}

// openedPackage is a types package whose enum Wide is opened, with @codes running against its wires, of n members, some retired.
func openedPackage(n int) *ir.Package {
	wide := retiredEnum("Wide", n, &tI32, func(i int) int64 { return int64(n-i) * codeStride })
	return &ir.Package{
		Name: retiredPkg, Dir: "ret/lines", Types: []ir.Type{wide},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "ret/lines/out", Mode: ir.ModeTypes, Namespace: "ret::lines", Open: []string{"Wide"}}},
	}
}

// helperBodies are the line counts, ascending, of the functions of the generated files whose name ends with suffix, as fnsize measures them.
func helperBodies(t *testing.T, files []ir.File, suffix string) []int {
	t.Helper()
	var out []int
	for _, f := range bodies(t, files) {
		if strings.HasSuffix(f.Name, suffix) {
			out = append(out, f.Body)
		}
	}
	slices.Sort(out)
	return out
}

// retiredCase is one package of the test: the tables it must write once, and how many IsRetired and FromCode definitions it has.
type retiredCase struct {
	name                string
	build               func(n int) *ir.Package
	tables              []string
	isRetired, fromCode int
}

var retiredCases = []retiredCase{
	{
		name: "closed", build: retiredPackage, isRetired: wantIsRetired, fromCode: wantFromCode,
		tables: []string{"kManyRetired", "kDenseRetired", "kSparseRetired", "kShapeKindRetired", "kSlotIdRetired"},
	},
	{name: "opened", build: openedPackage, isRetired: 1, fromCode: 1, tables: []string{"kWideRetired", "kWideCodes", "kWideCodeAt"}},
}

// CODEGEN.md §2.11, §5.2, §5.3, DECISIONS 395, 430, 435: IsRetired and <E>FromCode of a closed enum, a @codes enum (dense and sparse), an opened @codes enum, a kind enum and an id enum hold a constant number of lines whatever the members; their flags and codes are k<E>Retired, k<E>Codes and k<E>CodeAt tables of the .gen.cpp's unnamed namespace.
func TestIsRetiredConstantLines(t *testing.T) {
	const small, large = 3, 3000
	for _, tc := range retiredCases {
		t.Run(tc.name, func(t *testing.T) {
			got := map[int]map[string][]int{}
			for _, n := range []int{small, large} {
				files := generate(t, tc.build(n))
				got[n] = map[string][]int{}
				for _, name := range []string{isRetiredName, fromCodeName} {
					got[n][name] = helperBodies(t, files, name)
				}
				source := fileOf(t, files, ".gen.cpp")
				for _, table := range tc.tables {
					if strings.Count(source, " "+table+" = ") != 1 {
						t.Errorf("n=%d: %s is not written once", n, table)
					}
				}
			}
			for name, want := range map[string]int{isRetiredName: tc.isRetired, fromCodeName: tc.fromCode} { //canon:unordered a check per name
				if n := len(got[small][name]); n != want {
					t.Errorf("%s: %d definitions, want %d", name, n, want)
				}
				if !slices.Equal(got[small][name], got[large][name]) {
					t.Errorf("%s: %v lines for %d members, %v for %d: want a constant number", name, got[small][name], small, got[large][name], large)
				}
			}
		})
	}
}
