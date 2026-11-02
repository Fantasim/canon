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

// bakedFinds define, in the .gen.cpp, the Find and FindBy<F> of every container the header declares: a table's Find through its id enum, any other lookup a binary search of the keys sorted here, beside the positions of their rows (CODEGEN.md §5.9, DECISIONS 432).
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

func (g *gen) containerFinds(v *ir.Value) {
	s, rows := g.containerSpec(v), g.rows(v)
	if kf := g.keyFieldOf(v); kf != nil {
		g.findDefinition(s, findName, kf.Type, g.fieldValues(rows, kf.Name))
	} else {
		g.tableFind(s, v)
	}
	for _, f := range s.stable {
		g.findDefinition(s, g.pl.FindBy(f).Func, f.Type, g.fieldValues(rows, f.Name))
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

// findDefinition is the lookup fn of container s over vals, the key of each row of type t: the keys sorted in C++'s order and the positions of their rows, as constant arrays local to the function, and the search of lower_bound.txt (CODEGEN.md §5.9, DECISIONS 432).
func (g *gen) findDefinition(s containerSpec, fn string, t ir.TypeRef, vals []value.Value) {
	keys, rows := g.sortedKeys(t, vals)
	elem, key, prelude := g.storage(t), keyParam, ""
	if g.foreignID(t) {
		key, prelude = wireParam, foreignWireLine
	}
	if g.stringKeyed(t) {
		elem = cppStringView
	}
	g.c.write(search{
		sig:     fmt.Sprintf(findDefFormat, s.elem, s.name, fn, lookupType(g.storage(t))),
		prelude: prelude + table(elem, keysTable, keys) + table(cppUint32, rowsTable, intTexts(rows)),
		keys:    keysTable, key: key, hit: fmt.Sprintf(rowHitFormat, loVar), miss: cppNullptr,
	}.text())
}

// foreignID reports a key type held as the id enum of another package's table (CODEGEN.md §5.3).
func (g *gen) foreignID(t ir.TypeRef) bool {
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
			return enumRank(e, x.Index)
		}
	case *value.Ref:
		return g.refRank(t, x.Key)
	}
	g.malformed(fmt.Sprintf(valueFormat, v), g.at)
	return keyRank{}
}

// enumRank is member i of e by its C++ underlying value: its code under @codes, else its position (CODEGEN.md §5.2).
func enumRank(e *ir.Enum, i int) keyRank {
	if e.Codes != nil && i >= 0 && i < len(e.Members) {
		return keyRank{n: e.Members[i].Code}
	}
	return keyRank{n: int64(i)}
}

// refRank is the place of a ref's key k: by entry position when it is held as a table's id enum, else by its key's order (CODEGEN.md §5.8, §5.9).
func (g *gen) refRank(t ir.TypeRef, k value.Key) keyRank {
	if _, held := g.idEnum(t); held {
		return g.idRank(t, k)
	}
	if e, i := keyMember(t, k); i >= 0 {
		return enumRank(e, i)
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
