package cppgen_test

import (
	"math"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const findPkg = "demo.finder"

// finder is the IR of the baked-find fixture (DECISIONS 432): one container of each key kind a baked Find searches, its rows in an order that is none of the sorted ones.
type finder struct {
	kind, entry, number, toneRow, rankRow, kindRow, linkRow, slot *ir.Record
	tone, rank                                                    *ir.Enum
}

// findColl is the collection of the value table, a let of this package.
func findColl(table string) *types.Collection {
	return &types.Collection{Kind: types.CollLet, Pkg: findPkg, Name: table}
}

// findRow is an entry of table keyed by k, its fields given.
func findRow(rec *ir.Record, table string, k value.Key, fields ...value.Value) *value.Record {
	return &value.Record{T: bulkTypeOf(findPkg, rec.Name, rec), Fields: fields, Ident: &value.Identity{Coll: findColl(table), Key: k}}
}

// findList is a keyed list of rec by field name, its rows given.
func findList(name string, rec *ir.Record, by string, rows ...*value.Record) *ir.Value {
	elem := ir.TypeRef{Kind: types.Record, Named: rec}
	vs := make([]value.Value, len(rows))
	for i, r := range rows {
		vs[i] = r
	}
	return &ir.Value{Name: name, Type: ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: by, WirePath: []string{by}}}, V: list(vs...)}
}

// findRef is a ref into value table of this package, keyed by key.
func findRef(table string, elem ir.Type, key ir.TypeRef, keyed bool) ir.TypeRef {
	return ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: findPkg, Value: table, Elem: elem, Keyed: keyed}}
}

func (f *finder) types() {
	f.tone = &ir.Enum{Pkg: findPkg, Name: "Tone", Members: members([]string{"first", "z", "second", "y", "third", "x"}, nil)}
	f.rank = &ir.Enum{Pkg: findPkg, Name: "Rank", Codes: &tU8, Members: members([]string{"high", "high", "low", "low", "mid", "mid"}, []int64{9, 1, 4})}
	label := field("label", "label", "", tString)
	f.kind = &ir.Record{Pkg: findPkg, Name: "Kind", Fields: []*ir.Field{label}}
	code, alias := field("code", "code", "", tU16), field("alias", "alias", "", tString)
	code.Stable, alias.Stable = true, true
	f.entry = &ir.Record{Pkg: findPkg, Name: "Entry", Fields: []*ir.Field{field("name", "name", "", tString), code, alias}}
	f.number = &ir.Record{Pkg: findPkg, Name: "Number", Fields: []*ir.Field{field("n", "n", "", tInt), label}}
	f.toneRow = &ir.Record{Pkg: findPkg, Name: "ToneRow", Fields: []*ir.Field{field("tone", "tone", "", ir.TypeRef{Kind: types.Enum, Named: f.tone}), label}}
	f.rankRow = &ir.Record{Pkg: findPkg, Name: "RankRow", Fields: []*ir.Field{field("rank", "rank", "", ir.TypeRef{Kind: types.Enum, Named: f.rank}), label}}
	f.kindRow = &ir.Record{Pkg: findPkg, Name: "KindRow", Fields: []*ir.Field{field("kind", "kind", "", findRef("kinds", f.kind, tString, false)), label}}
	f.linkRow = &ir.Record{Pkg: findPkg, Name: "LinkRow", Fields: []*ir.Field{field("link", "link", "", findRef("names", f.entry, tString, true)), label}}
	f.slot = &ir.Record{Pkg: findPkg, Name: "Slot", Fields: []*ir.Field{field("name", "name", "", tString)}}
}

// entries are the rows of names: keys of one byte, a prefix pair, a NUL inside, a byte past 0x7f, the codes (UInt16) and aliases the FindBy lookups search.
func (f *finder) entries() []*value.Record {
	entry := func(name string, code int64, alias string) *value.Record {
		return findRow(f.entry, "names", value.Key{S: name}, str(name), num(code), str(alias))
	}
	return []*value.Record{
		entry("prefix", 900, "x9"), entry("a\x00b", 7, "x1"), entry("é", 65535, "é2"), entry("pre", 0, "y"),
		entry("a", 300, "x10"), entry("Z", 12, "p"), entry("ab", 4000, "pp"), entry("_", 256, "q"),
	}
}

func intKey(n int64) value.Key { return value.Key{I: n, IsInt: true} }

// numbers are the rows of numbers: negatives, zero, plus and minus 2^62 and the int64 limits.
func (f *finder) numbers() []*value.Record {
	var rows []*value.Record
	for _, n := range []int64{7, -5, 0, 1 << 62, math.MinInt64, -(1 << 62), math.MaxInt64, 6, -6} {
		rows = append(rows, findRow(f.number, "numbers", intKey(n), num(n), str("n")))
	}
	return rows
}

func (f *finder) values() []*ir.Value {
	kindElem := ir.TypeRef{Kind: types.Record, Named: f.kind}
	ids := []string{"zeta", "alpha", "mid", "beta"}
	kinds := make([]*value.Record, len(ids))
	for i, id := range ids {
		kinds[i] = findRow(f.kind, "kinds", value.Key{S: id}, str(id))
	}
	kinds[1].Ident.Retired = true
	byEnum := func(name string, rec *ir.Record, e *ir.Enum, order ...int) *ir.Value {
		var rows []*value.Record
		for _, i := range order {
			rows = append(rows, findRow(rec, name, value.Key{S: e.Members[i].Name}, member(i), str("m")))
		}
		return findList(name, rec, rec.Fields[0].Name, rows...)
	}
	byKind := func(order ...string) *ir.Value {
		var rows []*value.Record
		for _, id := range order {
			rows = append(rows, findRow(f.kindRow, "byKind", value.Key{S: id}, key(id), str("k")))
		}
		return findList("byKind", f.kindRow, "kind", rows...)
	}
	var links []*value.Record
	for _, name := range []string{"ab", "Z", "é", "a"} {
		links = append(links, findRow(f.linkRow, "links", value.Key{S: name}, key(name), str("l")))
	}
	return []*ir.Value{
		{Name: "kinds", IDs: ids, Type: ir.TypeRef{Kind: types.Table, Elem: &kindElem}, V: &value.Table{Entries: kinds}},
		findList("names", f.entry, "name", f.entries()...),
		findList("numbers", f.number, "n", f.numbers()...),
		byEnum("tones", f.toneRow, f.tone, 2, 0, 1),
		byEnum("ranks", f.rankRow, f.rank, 0, 1, 2),
		byKind("mid", "zeta", "beta"),
		findList("links", f.linkRow, "link", links...),
		findList("empties", f.slot, "name"),
	}
}

// finderPackage is demo.finder, a baked package whose containers are keyed by a String (bytes that sort apart from their positions), an Int (negatives, 2^62, the limits), an enum whose wire order is the reverse of its positions, a @codes enum whose codes are 9, 1, 4, a ref held as a table's id enum, a ref into a keyed list and a table, with FindBy over UInt16 and String and a keyed list of no row (CODEGEN.md §5.9, DECISIONS 432).
func finderPackage() *ir.Package {
	f := &finder{}
	f.types()
	return &ir.Package{
		Name: findPkg, Dir: "demo/finder",
		Types:  []ir.Type{f.tone, f.rank, f.kind, f.entry, f.number, f.toneRow, f.rankRow, f.kindRow, f.linkRow, f.slot},
		Values: f.values(),
		Emits:  []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/finder/out", Mode: ir.ModeBaked, Namespace: "demo::finder"}},
	}
}
