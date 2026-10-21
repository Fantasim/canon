package cppgen_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	bulkPkg   = "big.bulk"
	bulkNames = 11500 // the TID_* header of the handoff: 11 487 names
	bulkItems = 7400  // the item table of the handoff: 7 306 rows
	bulkMap   = 3000
	bulkGroup = 5000 // a map of string lists, nested in arrays
	bulkGrid  = 100  // rows and columns of a list of int lists
	bulkEvery = 7    // one item in bulkEvery is rare, has a note and gives coins
)

// bulkTypes are the records, enum and variant of the bulk fixture.
type bulkTypes struct {
	header, item   *ir.Record
	grade          *ir.Enum
	reward         *ir.Variant
	headerT, itemT *types.RecordType
	coins, nothing *types.CaseType
}

// bulkName is the i-th name of the big header, as the handoff's TID_* defines read.
func bulkName(i int) string { return fmt.Sprintf("TID_GAME_TEXT_%05d", i) }

// bulkItem is the id of the i-th item row.
func bulkItem(i int) string { return fmt.Sprintf("II_ITEM_%05d", i) }

// bulkRow is entry id of table, its fields given.
func bulkRow(t *types.RecordType, table, id string, fields ...value.Value) *value.Record {
	coll := &types.Collection{Kind: types.CollLet, Pkg: bulkPkg, Name: table}
	return &value.Record{T: t, Fields: fields, Ident: &value.Identity{Coll: coll, Key: value.Key{S: id}}}
}

// bulkType is the checker's record type of rec, its fields named as rec's.
func bulkType(name string, rec *ir.Record) *types.RecordType {
	rt := &types.RecordType{Pkg: bulkPkg, Name: name}
	for i, f := range rec.Fields {
		rt.Fields = append(rt.Fields, &types.Field{Name: f.Name, Index: i})
	}
	return rt
}

// bulkHeaders are the rows of headers: a small one, the TID_* one, and an empty one.
func (b *bulkTypes) bulkHeaders() (ids []string, rows []*value.Record) {
	names, values := make([]value.Value, bulkNames), make([]value.Value, bulkNames)
	for i := range names {
		names[i], values[i] = str(bulkName(i)), num(int64(i)*3)
	}
	small := bulkRow(b.headerT, "headers", "small", str("small.h"), list(str("A"), str("B")), list(num(1), num(2)), list())
	big := bulkRow(b.headerT, "headers", "big", str("defineText.h"), &value.List{Elems: names}, &value.List{Elems: values}, list(str("TID_ALIAS")))
	empty := bulkRow(b.headerT, "headers", "empty", str("empty.h"), list(), list(), list())
	return []string{"small", "big", "empty"}, []*value.Record{small, big, empty}
}

// bulkItemRows are the rows of items: a scalar of every kind, two tags, a ref, an optional
// note, an enum and a variant, which arrays cannot hold.
func (b *bulkTypes) bulkItemRows() (ids []string, rows []*value.Record) {
	ids, rows = make([]string, bulkItems), make([]*value.Record, bulkItems)
	headers := []string{"small", "big", "empty"}
	for i := range rows {
		ids[i] = bulkItem(i)
		var note value.Value = &value.None{}
		reward := &value.Record{T: b.nothing}
		if i%bulkEvery == 0 {
			note = str(fmt.Sprintf("note %d", i))
			reward = &value.Record{T: b.coins, Fields: []value.Value{num(int64(i))}}
		}
		rows[i] = bulkRow(b.itemT, "items", ids[i], str(fmt.Sprintf("Item %d", i)), num(int64(i%120)), num(int64(i)*10),
			boolean(i%bulkEvery == 0), dur(int64(i)*100), list(str(fmt.Sprintf("t%d", i%5)), str("u")),
			key(headers[i%len(headers)]), note, member(i%len(b.grade.Members)), reward)
	}
	return ids, rows
}

func (b *bulkTypes) types() {
	b.header = &ir.Record{Pkg: bulkPkg, Name: "Header", Fields: []*ir.Field{
		field("file", "file", "", tString), field("names", "names", "", listOf(tString)),
		field("values", "values", "", listOf(tInt)), field("aliases", "aliases", "", listOf(tString)),
	}}
	b.grade = &ir.Enum{Pkg: bulkPkg, Name: "Grade", Members: []*ir.EnumMember{
		{Name: "low", Wire: "low", Index: 0}, {Name: "mid", Wire: "mid", Index: 1}, {Name: "high", Wire: "high", Index: 2},
	}}
	b.reward = &ir.Variant{Pkg: bulkPkg, Name: "Reward", Tag: "kind", Cases: []*ir.Case{
		{Name: "coins", Wire: "coins", Fields: []*ir.Field{field("amount", "amount", "", tInt)}},
		{Name: "nothing", Wire: "nothing"},
	}}
	b.coins = &types.CaseType{Name: "coins", Index: 0, Fields: []*types.Field{{Name: "amount"}}}
	b.nothing = &types.CaseType{Name: "nothing", Index: 1}
	stringKey := tString
	headerRef := ir.TypeRef{Kind: types.Ref, Key: &stringKey, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: bulkPkg, Value: "headers", Elem: b.header}}
	note := field("note", "note", "", tString)
	note.Optional = true
	b.item = &ir.Record{Pkg: bulkPkg, Name: "Item", Fields: []*ir.Field{
		field("name", "name", "", tString), field("level", "level", "", tInt), field("price", "price", "", tInt),
		field("rare", "rare", "", tBool), field("cooldown", "cooldown", "", tDuration), field("tags", "tags", "", listOf(tString)),
		field("header", "header", "", headerRef), note, field("grade", "grade", "", ir.TypeRef{Kind: types.Enum, Named: b.grade}),
		field("reward", "reward", "", ir.TypeRef{Kind: types.Variant, Named: b.reward}),
	}}
	b.headerT, b.itemT = bulkType("Header", b.header), bulkType("Item", b.item)
}

// bulkPackage is a baked package whose data would grow the generated C++ (DECISIONS 392): a
// list of bulkNames strings in one row, a table of bulkItems rows, a map of bulkMap entries.
func bulkPackage() *ir.Package {
	b := &bulkTypes{}
	b.types()
	headerIDs, headers := b.bulkHeaders()
	itemIDs, items := b.bulkItemRows()
	headerElem, itemElem := ir.TypeRef{Kind: types.Record, Named: b.header}, ir.TypeRef{Kind: types.Record, Named: b.item}
	key, val := tString, tInt
	weights := &value.Map{}
	for i := range bulkMap {
		weights.Keys = append(weights.Keys, str(bulkItem(i)))
		weights.Vals = append(weights.Vals, num(int64(i)+1))
	}
	groups, grid := bulkNested()
	strs, ints := listOf(tString), listOf(tInt)
	return &ir.Package{
		Name: bulkPkg, Dir: "big/bulk",
		Types: []ir.Type{b.grade, b.header, b.reward, b.item},
		Values: []*ir.Value{
			{Name: "headers", IDs: headerIDs, Type: ir.TypeRef{Kind: types.Table, Elem: &headerElem}, V: &value.Table{Entries: headers}},
			{Name: "items", IDs: itemIDs, Type: ir.TypeRef{Kind: types.Table, Elem: &itemElem}, V: &value.Table{Entries: items}},
			{Name: "weights", Type: ir.TypeRef{Kind: types.Map, Key: &key, Elem: &val}, V: weights},
			{Name: "groups", Type: ir.TypeRef{Kind: types.Map, Key: &key, Elem: &strs}, V: groups},
			{Name: "grid", Type: listOf(ints), V: grid},
		},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "big/bulk/out", Mode: ir.ModeBaked, Namespace: "big::bulk"}},
	}
}

// bulkNested are a map of bulkGroup string lists, the i-th of i%4 names, and a bulkGrid square of ints.
func bulkNested() (groups *value.Map, grid *value.List) {
	groups, grid = &value.Map{}, &value.List{}
	for i := range bulkGroup {
		names := &value.List{}
		for j := range i % 4 {
			names.Elems = append(names.Elems, str(fmt.Sprintf("m%d_%d", i, j)))
		}
		groups.Keys, groups.Vals = append(groups.Keys, str(fmt.Sprintf("group%04d", i))), append(groups.Vals, names)
	}
	for r := range bulkGrid {
		row := &value.List{}
		for c := range bulkGrid {
			row.Elems = append(row.Elems, num(int64(r*bulkGrid+c)))
		}
		grid.Elems = append(grid.Elems, row)
	}
	return groups, grid
}
