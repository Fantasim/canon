package cppgen

import (
	"cmp"
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

//go:embed text/baked_table_find.txt
var bakedTableFindText string

// keyRank is a key's place in C++'s order: a string by its bytes (s), anything else by its number (n): an integer's value, an enum's underlying value, a table id's entry position.
type keyRank struct {
	s string
	n int64
}

func (a keyRank) compare(b keyRank) int {
	return cmp.Or(cmp.Compare(a.n, b.n), strings.Compare(a.s, b.s))
}

// bakedFinds define, in the .gen.cpp, the Find and FindBy<F> of every container the header declares: a table's Find through its id enum, any other lookup a binary search of the keys sorted here, beside the positions of their rows, both held in the unnamed namespace of detail (CODEGEN.md §5.9, DECISIONS 432, 435).
func (g *gen) bakedFinds() {
	for _, v := range g.values {
		if !ir.IsContainer(v) {
			continue
		}
		leave := g.enter(v.Name)
		g.containerFinds(v)
		leave()
	}
}

// lookup is one lookup fn of a container: the key type, the sorted keys of its rows and the positions of those rows, written as the tables it reads.
type lookup struct {
	fn     string
	t      ir.TypeRef
	tables ir.CppFindTables
	elem   string
	keys   []string
	rows   []string
}

// containerFinds writes the Find of keyed list v, FindBy<F> of each of its @stable fields, and the tables they read (CODEGEN.md §5.9, DECISIONS 432, 435); a container without a key field finds through its table (tableFind).
func (g *gen) containerFinds(v *ir.Value) {
	s, rows := g.containerSpec(v), g.rows(v)
	var lookups []lookup
	kf := g.keyFieldOf(v)
	if kf != nil {
		lookups = append(lookups, g.lookupOf(findName, g.pl.FindTables(v), kf.Type, g.fieldValues(rows, kf.Name)))
	}
	for _, f := range s.stable {
		lookups = append(lookups, g.lookupOf(g.pl.FindBy(f).Func, g.pl.FindByTables(v, f), f.Type, g.fieldValues(rows, f.Name)))
	}
	g.lookupTables(lookups)
	if kf == nil {
		g.tableFind(s, v)
	}
	for _, l := range lookups {
		g.findDefinition(s, l)
	}
}

// fieldValues are field name of each row.
func (g *gen) fieldValues(rows []*value.Record, name string) []value.Value {
	out := make([]value.Value, len(rows))
	for i, r := range rows {
		out[i] = g.fieldOf(r, name)
	}
	return out
}

// tableFind is a table's Find: its id enum's <Rec>IdFromWire, then the row at that id (CODEGEN.md §5.3, §5.9).
func (g *gen) tableFind(s containerSpec, v *ir.Value) {
	rec, ok := v.Type.Elem.Named.(*ir.Record)
	if !ok {
		return // containerSpec reported it
	}
	id := g.pl.IDName(rec)
	g.c.printf(bakedTableFindText, fmt.Sprintf(findDefFormat, s.elem, s.name, findName, cppStringView), id, g.pl.EnumHelpers(id).FromWire)
}

// lookupOf is the lookup fn over vals, the key of each row of type t: the keys sorted in C++'s order and the positions of their rows, held in the tables the plan names (CODEGEN.md §5.9, DECISIONS 432).
func (g *gen) lookupOf(fn string, tables ir.CppFindTables, t ir.TypeRef, vals []value.Value) lookup {
	keys, rows := g.sortedKeys(t, vals)
	elem := g.storage(t)
	if g.stringKeyed(t) {
		elem = cppStringView
	}
	return lookup{fn: fn, t: t, tables: tables, elem: elem, keys: keys, rows: intTexts(rows)}
}

// lookupTables write the sorted keys and the rows' positions of lookups, once in the .gen.cpp: in detail's unnamed namespace, local to this file, so a lookup keeps a constant number of lines whatever the rows (CODEGEN.md §2.11, §5.9; DECISIONS 435).
func (g *gen) lookupTables(lookups []lookup) {
	if len(lookups) == 0 {
		return
	}
	g.c.line(detailOpen)
	g.c.line(anonOpen)
	for _, l := range lookups {
		g.c.printf(localTableFormat, l.elem, len(l.keys), l.tables.Keys, braced(l.keys))
		g.c.printf(localTableFormat, cppUint32, len(l.rows), l.tables.Rows, braced(l.rows))
	}
	g.c.line(anonClose)
	g.c.line(detailClose)
	g.c.blank()
}

// findDefinition is the lookup l of container s: the search of lower_bound.txt over its keys table, returning the row at the position beside the key found (CODEGEN.md §5.9, DECISIONS 432).
func (g *gen) findDefinition(s containerSpec, l lookup) {
	key, prelude := keyParam, ""
	if g.foreignID(l.t) {
		key, prelude = wireParam, foreignWireLine
	}
	g.c.write(search{
		sig:     fmt.Sprintf(findDefFormat, s.elem, s.name, l.fn, lookupType(g.storage(l.t))),
		prelude: prelude,
		keys:    detailPrefix + l.tables.Keys, key: key, hit: fmt.Sprintf(rowHitFormat, detailPrefix+l.tables.Rows, loVar), miss: cppNullptr,
	}.text())
}

// foreignID reports a key type held as the id enum of another package's table (CODEGEN.md §5.3).
func (g *gen) foreignID(t ir.TypeRef) bool {
	t = ir.RefLeaf(t)
	_, held := g.idEnum(t)
	return held && t.Ref.Pkg != g.p.Name
}

// stringKeyed reports a key searched as a string: a String, or a ref held as another package's id enum, whose entry positions this IR does not hold: its wires are sorted by bytes and the key goes through the owner's ToWire, an id outside the enum giving no wire (CODEGEN.md §5.9, DECISIONS 432).
func (g *gen) stringKeyed(t ir.TypeRef) bool {
	return g.storage(t) == cppString || g.foreignID(t)
}

// sortedKeys are the keys vals, each of type t, written as the array of a lookup holds them, in C++'s order, and the position of each one's row (CODEGEN.md §5.9, DECISIONS 432).
func (g *gen) sortedKeys(t ir.TypeRef, vals []value.Value) (keys []string, rows []int) {
	order := inOrder(len(vals))
	ranks := make([]keyRank, len(vals))
	for i, v := range vals {
		ranks[i] = g.rankOf(t, v)
	}
	slices.SortStableFunc(order, func(a, b int) int { return ranks[a].compare(ranks[b]) })
	keys = make([]string, len(order))
	wires := g.stringKeyed(t)
	for i, at := range order {
		keys[i] = ranks[at].s
		if !wires {
			keys[i] = g.element(t, vals[at])
		}
	}
	if wires {
		keys = views(keys)
	}
	return keys, order
}

// rankOf is the place of key v of type t in C++'s order: strings by bytes, integers by value, an enum by its underlying value, a ref as its key is held (CODEGEN.md §5.9).
func (g *gen) rankOf(t ir.TypeRef, v value.Value) keyRank {
	switch x := v.(type) { // no Duration: a key is a String, an integer, an enum or a ref (keyable)
	case *value.Str:
		return keyRank{s: x.V}
	case *value.Int:
		return keyRank{n: x.V}
	case *value.Member:
		if e, ok := t.Named.(*ir.Enum); ok {
			return g.enumRank(e, x.Index)
		}
	case *value.Ref:
		return g.refRank(t, x.Key)
	}
	g.malformed(fmt.Sprintf(valueFormat, v), g.at)
	return keyRank{}
}

// enumRank is member i of e by its C++ underlying value: its code under @codes, else its position (CODEGEN.md §5.2). An index outside the members of a @codes enum is malformed IR, which valid IR never reaches.
func (g *gen) enumRank(e *ir.Enum, i int) keyRank {
	if e.Codes == nil {
		return keyRank{n: int64(i)}
	}
	if i < 0 || i >= len(e.Members) {
		g.malformed(fmt.Sprintf(rankOutsideFormat, i, e.Name), g.at)
		return keyRank{}
	}
	return keyRank{n: e.Members[i].Code}
}

// refRank is the place of a ref's key k: by entry position when it is held as a table's id enum, else by its key's order; a ref into a list keyed by a ref is placed as that ref's key, down the chain (CODEGEN.md §5.8, §5.9, DECISIONS 432, 456).
func (g *gen) refRank(t ir.TypeRef, k value.Key) keyRank {
	t = ir.RefLeaf(t)
	if _, held := g.idEnum(t); held {
		return g.idRank(t, k)
	}
	if e, i := keyMember(t, k); i >= 0 {
		return g.enumRank(e, i)
	}
	if k.IsInt {
		return keyRank{n: k.I}
	}
	return keyRank{s: k.S}
}

// idRank is the entry position of key k in the table t targets, the underlying value of its id enum member; of another package's table it is the key's wire, found through ToWire (CODEGEN.md §5.3).
func (g *gen) idRank(t ir.TypeRef, k value.Key) keyRank {
	if g.foreignID(t) {
		return keyRank{s: k.S}
	}
	pos, ok := g.bk.index[t.Ref.Value][k]
	if !ok {
		g.malformed(bakedEntryKey, g.at)
	}
	return keyRank{n: int64(pos)}
}
