package cppgen_test

import (
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	chainEnumPkg = "demo.chainenum"
	chainBPkg    = "chain.b"
	chainAPkg    = "chain.a"
)

// chainEnumPackage is demo.chainenum, a baked package with two chains that end in an enum-keyed list, each three lists deep: Tone has no @codes and its member names sort in an order that is not its positions'; Rank has @codes out of declaration order and a byte order of its own, so no rank of the keys but the right one finds them (CODEGEN.md §5.2, §5.9, DECISIONS 432, 456).
func chainEnumPackage() *ir.Package {
	tone := &ir.Enum{Pkg: chainEnumPkg, Name: "Tone", Members: members([]string{"zed", "z", "alpha", "y", "mid", "x"}, nil)}
	rank := &ir.Enum{Pkg: chainEnumPkg, Name: "Rank", Codes: &tU8, Members: members([]string{"zulu", "zulu", "alpha", "alpha", "mid", "mid"}, []int64{4, 9, 1})}
	p := &ir.Package{
		Name: chainEnumPkg, Dir: "demo/chainenum", Types: []ir.Type{tone, rank},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/chainenum/out", Mode: ir.ModeBaked, Namespace: "demo::chainenum"}},
	}
	chainEnumLists(p, tone, [3]string{"tone", "pick", "over"}, [3][]int{{2, 0, 1}, {2, 0}, {0, 2}})
	chainEnumLists(p, rank, [3]string{"rank", "rpick", "rover"}, [3][]int{{1, 2, 0}, {0, 2, 1}, {1, 2, 0}})
	return p
}

// chainEnumLists adds to p the lists of one enum chain: n[0] keyed by e, n[1] by a ref into it, n[2] by a ref into that, each holding the members of rows in the order given.
func chainEnumLists(p *ir.Package, e *ir.Enum, n [3]string, rows [3][]int) {
	label := field("label", "label", "", tString)
	named := func(s string) string { return strings.ToUpper(s[:1]) + s[1:] + "Row" }
	baseRef := chainRef(chainEnumPkg, n[0]+"s", ir.TypeRef{Kind: types.Enum, Named: e}, true)
	pickRef := chainRef(chainEnumPkg, n[1]+"s", baseRef, true)
	baseRow := &ir.Record{Pkg: chainEnumPkg, Name: named(n[0]), Fields: []*ir.Field{field(n[0], n[0], "", ir.TypeRef{Kind: types.Enum, Named: e}), label}}
	pickRow := &ir.Record{Pkg: chainEnumPkg, Name: named(n[1]), Fields: []*ir.Field{field(n[1], n[1], "", baseRef), label}}
	overRow := &ir.Record{Pkg: chainEnumPkg, Name: named(n[2]), Fields: []*ir.Field{field(n[2], n[2], "", pickRef), label}}
	baseRef.Ref.Elem, pickRef.Ref.Elem = baseRow, pickRow
	var base []value.Value
	for _, i := range rows[0] {
		base = append(base, chainRow(chainEnumPkg, n[0]+"s", baseRow, e.Members[i].Name, member(i), str("m")))
	}
	names := func(members []int) []string {
		out := make([]string, len(members))
		for i, m := range members {
			out[i] = e.Members[m].Name
		}
		return out
	}
	p.Types = append(p.Types, baseRow, pickRow, overRow)
	p.Values = append(p.Values,
		chainListOf(n[0]+"s", baseRow, base...),
		chainList(chainEnumPkg, n[1]+"s", pickRow, names(rows[1])),
		chainList(chainEnumPkg, n[2], overRow, names(rows[2])),
	)
}

// chainForeign is two baked packages: chain.b holds a table rows; chain.a a keyed list byRow keyed by `ref b.rows`, held as chain.b's id enum, and a keyed list over keyed by a ref into byRow.
type chainForeign struct {
	bEmit, aEmit     *ir.Emit
	row, byRow, over *ir.Record
}

func newChainForeign() *chainForeign {
	c := &chainForeign{
		bEmit: &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "chain/b", Mode: ir.ModeBaked, Namespace: "chain::b"},
		aEmit: &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "chain/a", Mode: ir.ModeBaked, Namespace: "chain::a"},
	}
	c.row = &ir.Record{Pkg: chainBPkg, Name: "Row", Fields: []*ir.Field{field("label", "label", "", tString)}}
	rowRef := chainRef(chainBPkg, "rows", tString, false)
	rowRef.Ref.Elem = c.row
	byRowRef := chainRef(chainAPkg, "byRow", rowRef, true)
	label := field("label", "label", "", tString)
	c.byRow = &ir.Record{Pkg: chainAPkg, Name: "ByRowRow", Fields: []*ir.Field{field("row", "row", "", rowRef), label}}
	c.over = &ir.Record{Pkg: chainAPkg, Name: "OverRow", Fields: []*ir.Field{field("over", "over", "", byRowRef), label}}
	byRowRef.Ref.Elem = c.byRow
	return c
}

// packageB is chain.b: a table whose ids are in no sorted order, wire or position.
func (c *chainForeign) packageB() *ir.Package {
	ids := []string{"zeta", "alpha", "mid", "beta"}
	rows := make([]*value.Record, len(ids))
	for i, id := range ids {
		rows[i] = chainRow(chainBPkg, "rows", c.row, id, str(id)).(*value.Record)
	}
	elem := ir.TypeRef{Kind: types.Record, Named: c.row}
	return &ir.Package{
		Name: chainBPkg, Dir: "chain/b", Types: []ir.Type{c.row}, Emits: []*ir.Emit{c.bEmit},
		Values: []*ir.Value{{Name: "rows", IDs: ids, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}, V: &value.Table{Entries: rows}}},
	}
}

// packageA is chain.a: byRow for beta, mid and zeta, not alpha, and over for zeta and beta, in orders that are neither the wires' nor the positions'.
func (c *chainForeign) packageA() *ir.Package {
	return &ir.Package{
		Name: chainAPkg, Dir: "chain/a", Types: []ir.Type{c.byRow, c.over}, Emits: []*ir.Emit{c.aEmit},
		Imports: []*ir.PackageRef{{Name: chainBPkg, Dir: "chain/b", Emits: []*ir.Emit{c.bEmit}}},
		Values: []*ir.Value{
			chainList(chainAPkg, "byRow", c.byRow, []string{"mid", "zeta", "beta"}),
			chainList(chainAPkg, "over", c.over, []string{"zeta", "beta"}),
		},
	}
}
