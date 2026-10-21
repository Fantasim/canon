package cppgen_test

import (
	"fmt"
	"math"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	smallPkg = "demo.bulk"
	smallN   = 8 // cppgen's bulkMinElems: the fewest elements read from an array (DECISIONS 392)
)

// bulkSmall is the IR of the baked-bulk fixture, shared by its builders.
type bulkSmall struct {
	tier          *ir.Enum
	item, crew    *ir.Record
	reward        *ir.Variant
	itemT, crewT  *types.RecordType
	coins, absent *types.CaseType
	tTier         ir.TypeRef
}

// smallRef is a ref to an entry of items.
func (s *bulkSmall) smallRef() ir.TypeRef {
	k := tString
	return ir.TypeRef{Kind: types.Ref, Key: &k, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: smallPkg, Value: "items", Elem: s.item}}
}

func (s *bulkSmall) types() {
	members := make([]*ir.EnumMember, smallN)
	for i := range members {
		members[i] = &ir.EnumMember{Name: fmt.Sprintf("t%d", i), Wire: fmt.Sprintf("t%d", i), Index: i}
	}
	s.tier = &ir.Enum{Pkg: smallPkg, Name: "Tier", Members: members}
	s.tTier = ir.TypeRef{Kind: types.Enum, Named: s.tier}
	s.reward = &ir.Variant{Pkg: smallPkg, Name: "Prize", Tag: "kind", Cases: []*ir.Case{
		{Name: "coins", Wire: "coins", Fields: []*ir.Field{field("amount", "amount", "", tInt)}}, {Name: "none", Wire: "none"},
	}}
	s.coins, s.absent = &types.CaseType{Name: "coins", Index: 0, Fields: []*types.Field{{Name: "amount"}}}, &types.CaseType{Name: "none", Index: 1}
	s.item = &ir.Record{Pkg: smallPkg, Name: "Item"}
	note := field("note", "note", "", tString)
	note.Optional = true
	code := field("code", "code", "", tInt)
	code.Stable = true
	s.item.Fields = []*ir.Field{
		field("name", "name", "", tString), code, field("tags", "tags", "", listOf(tString)), field("tier", "tier", "", s.tTier),
		note, field("next", "next", "", s.smallRef()), field("prize", "prize", "", ir.TypeRef{Kind: types.Variant, Named: s.reward}),
	}
	wait := field("wait", "wait", "", tDuration)
	wait.Optional = true
	optString := ir.TypeRef{Kind: types.Optional, Elem: &tString}
	s.item.Fields = append(s.item.Fields, field("ratio", "ratio", "", tFloat), field("weight", "weight", "", tFloat32),
		field("big", "big", "", tInt), wait, field("opts", "opts", "", listOf(optString)))
	s.item.Methods = []*ir.ExportFn{
		{Name: "twice", Kind: ir.FnPrecomputed, Result: tInt},
		{Name: "bonus", Kind: ir.FnLookup, Result: tInt, Params: []*ir.Param{{Name: "t", Type: s.tTier}}},
	}
	s.crew = &ir.Record{Pkg: smallPkg, Name: "Crew", Fields: []*ir.Field{field("n", "n", "", tInt), field("name", "name", "", tString)}}
	s.itemT, s.crewT = bulkTypeOf(smallPkg, "Item", s.item), bulkTypeOf(smallPkg, "Crew", s.crew)
}

// bulkTypeOf is the checker's record type of rec in package pkg, its fields named as rec's.
func bulkTypeOf(pkg, name string, rec *ir.Record) *types.RecordType {
	rt := bulkType(name, rec)
	rt.Pkg = pkg
	return rt
}

// items are smallN rows: i1 retired, odd rows with a note and coins, each next pointing at the following row.
func (s *bulkSmall) items() ([]string, []*value.Record) {
	ids, rows := make([]string, smallN), make([]*value.Record, smallN)
	twice, bonus := s.item.Methods[0], s.item.Methods[1]
	tiers := make([]value.Value, smallN)
	for i := range tiers {
		tiers[i] = member(i)
	}
	for i := range rows {
		ids[i] = fmt.Sprintf("i%d", i)
		var note value.Value = &value.None{}
		prize := &value.Record{T: s.absent}
		if i%2 == 1 {
			note, prize = str(fmt.Sprintf("odd %d", i)), &value.Record{T: s.coins, Fields: []value.Value{num(int64(i) * 10)}}
		}
		coll := &types.Collection{Kind: types.CollLet, Pkg: smallPkg, Name: "items"}
		rows[i] = &value.Record{T: s.itemT, Ident: &value.Identity{Coll: coll, Key: value.Key{S: ids[i]}, Retired: i == 1}, Fields: append([]value.Value{
			str(fmt.Sprintf("Item %d", i)), num(int64(100 + i)), list(str(fmt.Sprintf("t%d", i))), member(i % smallN),
			note, key(fmt.Sprintf("i%d", (i+1)%smallN)), prize,
		}, probes(i)...)}
		cells := make([]value.Value, smallN)
		for c := range cells {
			cells[c] = num(int64(i*smallN + c))
		}
		twice.Instances = append(twice.Instances, &ir.Instance{Recv: rows[i], Result: num(int64(2 * i))})
		bonus.Instances = append(bonus.Instances, &ir.Instance{Recv: rows[i], Table: &ir.LookupTable{Domains: [][]value.Value{tiers}, Cells: cells}})
	}
	return ids, rows
}

// probes are row i's float, float32, extreme integer, optional Duration and list with none
// elements: -0.0 and int64's limits in rows 0 and 7, the last row's list empty.
func probes(i int) []value.Value {
	ratio, big := flt(float64(i)/4), num(int64(i))
	var wait value.Value = &value.None{}
	opts := list(str("o"), &value.None{})
	switch i {
	case 0:
		ratio, big, opts = flt(math.Copysign(0, -1)), num(math.MinInt64), list(&value.None{})
	case smallN - 1:
		big, opts = num(math.MaxInt64), list()
	}
	if i%2 == 1 {
		wait = dur(int64(i) * 1000)
	}
	return []value.Value{ratio, flt(float64(i) + 0.5), big, wait, opts}
}

// values are the table, a list, a map, a list of refs and a keyed list, each of smallN elements.
func (s *bulkSmall) values() []*ir.Value {
	ids, rows := s.items()
	names, weights, picks := &value.List{}, &value.Map{}, &value.List{}
	crews := &value.List{}
	coll := &types.Collection{Kind: types.CollLet, Pkg: smallPkg, Name: "crews"}
	for i := range smallN {
		names.Elems = append(names.Elems, str(fmt.Sprintf("name \"%d\"", i)))
		if i == smallN/2 {
			names.Elems[i] = str("a\tb\x007\\\x7f") // escapes, an octal one a digit follows: seven bytes
		}
		weights.Keys, weights.Vals = append(weights.Keys, str(fmt.Sprintf("w%d", i))), append(weights.Vals, num(int64(i*i)))
		picks.Elems = append(picks.Elems, key(fmt.Sprintf("i%d", smallN-1-i)))
		crews.Elems = append(crews.Elems, &value.Record{T: s.crewT, Fields: []value.Value{num(int64(10 * i)), str(fmt.Sprintf("crew %d", i))},
			Ident: &value.Identity{Coll: coll, Key: value.Key{I: int64(10 * i), IsInt: true}}})
	}
	itemElem, crewElem := ir.TypeRef{Kind: types.Record, Named: s.item}, ir.TypeRef{Kind: types.Record, Named: s.crew}
	k, v := tString, tInt
	strs := listOf(tString)
	grid := list(list(num(1), num(2), num(3)), list(), list(num(4), num(5), num(6), num(7)))
	groups := &value.Map{Keys: []value.Value{str("g0"), str("g1"), str("g2")},
		Vals: []value.Value{list(str("a"), str("b")), list(), list(str("c"), str("d"), str("e"))}}
	return []*ir.Value{
		{Name: "grid", Type: listOf(listOf(tInt)), V: grid},
		{Name: "groups", Type: ir.TypeRef{Kind: types.Map, Key: &k, Elem: &strs}, V: groups},
		{Name: "items", IDs: ids, Type: ir.TypeRef{Kind: types.Table, Elem: &itemElem}, V: &value.Table{Entries: rows}},
		{Name: "names", Type: listOf(tString), V: names},
		{Name: "weights", Type: ir.TypeRef{Kind: types.Map, Key: &k, Elem: &v}, V: weights},
		{Name: "picks", Type: listOf(s.smallRef()), V: picks},
		{Name: "crews", Type: ir.TypeRef{Kind: types.List, Elem: &crewElem, KeyedBy: &ir.KeyField{Name: "n", WirePath: []string{"n"}}}, V: crews},
	}
}

// bulkSmallPackage is a baked package whose every bulk construct holds smallN elements, the fewest
// read from arrays: a table's columns, a list, a map, refs, keys, cells and constants (DECISIONS 392).
func bulkSmallPackage() *ir.Package {
	s := &bulkSmall{}
	s.types()
	labels := make([]value.Value, smallN)
	ranks := &value.Map{}
	tierNames := &value.List{}
	for i := range labels {
		labels[i] = str(fmt.Sprintf("Tier %d", i))
		if i%2 == 1 {
			labels[i] = &value.None{}
		}
		ranks.Keys, ranks.Vals = append(ranks.Keys, str(fmt.Sprintf("t%d", i))), append(ranks.Vals, num(int64(smallN-i)))
		tierNames.Elems = append(tierNames.Elems, str(fmt.Sprintf("T%d", i)))
	}
	k, v := tString, tInt
	strs := listOf(tString)
	return &ir.Package{
		Name: smallPkg, Dir: "demo/bulk",
		Types:  []ir.Type{s.tier, s.reward, s.item, s.crew},
		Values: s.values(),
		Consts: []*ir.Const{
			{Name: "TIER_NAMES", Type: listOf(tString), V: tierNames},
			{Name: "TIER_RANKS", Type: ir.TypeRef{Kind: types.Map, Key: &k, Elem: &v}, V: ranks},
			{Name: "GROUPS", Type: ir.TypeRef{Kind: types.Map, Key: &k, Elem: &strs}, V: constGroups()},
			{Name: "GRID", Type: listOf(listOf(tInt)), V: list(list(), list(num(1), num(2), num(3), num(4), num(5)), list())},
		},
		Fns:   []*ir.ExportFn{lookup("labelOf", "", ir.TypeRef{Kind: types.Optional, Elem: &k}, []*ir.Param{{Name: "t", Type: s.tTier}}, labels...)},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/bulk/out", Mode: ir.ModeBaked, Namespace: "demo::bulk"}},
	}
}

// constGroups are smallN groups of i%3 names, group 5's first name holding a NUL: a nested header
// constant whose initializer reads detail::kGROUPSElems arrays (DECISIONS 392).
func constGroups() *value.Map {
	m := &value.Map{}
	for i := range smallN {
		names := &value.List{}
		for j := range i % 3 {
			names.Elems = append(names.Elems, str(fmt.Sprintf("n%d%d", i, j)))
		}
		if i == smallN-3 {
			names.Elems[0] = str("a\x00b")
		}
		m.Keys, m.Vals = append(m.Keys, str(fmt.Sprintf("c%d", i))), append(m.Vals, names)
	}
	return m
}
