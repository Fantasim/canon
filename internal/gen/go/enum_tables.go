package gogen

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// enumView is the data of the enum templates: an enum's type, the names of its methods and of its tables (DECISIONS 393).
type enumView struct {
	Name, Method, Table, Parse, FromCode, Code, String, Wire, Known, Arg, Codes, Zero string
	Strconv, Slices, Strings                                                          string
	Names, Wires, Keys, ByWire, CodeKeys, Members, Numbers, ByCode                    string
	Retired, Flags, Value, Search, FlagRetired                                        string // Retired(): its name, the flag string, the receiver's index value, the sorted table it is searched in
	Index, CodeIndex, Wide, First, Found, ByWireElem, Item                            string // a value's index, a code's, the type both are compared as, the first dense code, the member Parse finds at the by-wire index i, the by-wire element type, what Members yields for m
	Sparse, Named                                                                     bool   // the codes are searched in their sorted table; an opened enum has a names table
}

// enumView names the tables and methods of s; a table's name is the plan's (DECISIONS 393, 430).
func (g *gen) enumView(s *enumSpec) enumView {
	table := func(role string) string { return g.names.TableName(role, s.name) }
	zero, value := zeroLit, selfName
	if s.opaque() {
		zero, value = fmt.Sprintf(idZeroFormat, s.name), selfPosition
	}
	v := enumView{
		Name: s.name, Parse: g.names.ParseName(s.name), FromCode: g.names.FromCodeName(s.name), Arg: s.parseArg, Codes: s.codes, Zero: zero,
		Slices: g.use(slicesPkg, slicesPkg),
		Names:  table(ir.GoTableNames), Wires: table(ir.GoTableWires), Keys: table(ir.GoTableKeys), ByWire: table(ir.GoTableByWire), CodeKeys: table(ir.GoTableCodes),
		Members: table(ir.GoTableMembers), Numbers: table(ir.GoTableNumbers), ByCode: table(ir.GoTableByCode),
		Retired: ir.GoRetired, Value: value, FlagRetired: string(flagRetired), ByWireElem: smallestUint(len(s.consts)),
		Wide: goUint, Index: fmt.Sprintf(convFormat, goUint, value), First: s.first, Sparse: s.codes != "" && s.first == "", Named: !s.wiresAreNames,
		Item: memberItem(s, table(ir.GoTableKeys)),
	}
	if s.first != "" {
		v.Wide = goUint64
		v.Index = fmt.Sprintf(denseIndexFormat, goUint64, goInt64, value, s.first)
		v.CodeIndex = fmt.Sprintf(denseIndexFormat, goUint64, goInt64, codeParam, s.first)
	}
	return v
}

// memberItem is what <E>Members yields for the element m of its table: m itself, or for an opened enum, whose table holds indexes, the key at m (DECISIONS 430).
func memberItem(s *enumSpec, keys string) string {
	if s.open {
		return fmt.Sprintf(keyedItemFormat, keys, tempMember)
	}
	return tempMember
}

// firstLiteral is the first dense code as an operand: a negative one in parentheses.
func firstLiteral(first int64) string {
	if first < 0 {
		return fmt.Sprintf(negFirstFormat, first)
	}
	return strconv.FormatInt(first, decimal)
}

// writeTable writes `var name = [...]elem{items}`: on one line when short, else rows of tableRowItems (go/format keeps the rows).
func (g *gen) writeTable(name, elem string, items []string) {
	body := strings.Join(items, listSep)
	if len(items) > tableRowItems {
		rows := make([]string, 0, len(items)/tableRowItems+1)
		for row := range slices.Chunk(items, tableRowItems) {
			rows = append(rows, strings.Join(row, listSep))
		}
		body = tableOpen + strings.Join(rows, tableRowEnd) + tableRowEnd
	}
	g.printf(tableFormat, name, elem, body)
}

// order is the indexes 0..n-1 sorted by cmpAt, equal ones in index order, so the first declared wins a search.
func order(n int, cmpAt func(a, b int) int) []int {
	idx := identity(n)
	slices.SortStableFunc(idx, cmpAt)
	return idx
}

// identity is the indexes 0..n-1.
func identity(n int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	return idx
}

// pick is items at the indexes idx, in that order.
func pick[T any](items []T, idx []int) []T {
	out := make([]T, len(idx))
	for i, at := range idx {
		out[i] = items[at]
	}
	return out
}

// quoted is every item as a Go string literal.
func quoted(items []string) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = strconv.Quote(item)
	}
	return out
}

// byText is the member indexes sorted by their text in byte order: the order of a lookup table.
func byText(texts []string) []int {
	return order(len(texts), func(a, b int) int { return strings.Compare(texts[a], texts[b]) })
}

// byWire is the positions 0..n-1 of texts sorted by text in byte order, equal texts by the declaration index of their position (decl), so the first declared wins a search (DECISIONS 406, 430).
func byWire(texts []string, decl []int) []int {
	return order(len(texts), func(a, b int) int {
		return cmp.Or(strings.Compare(texts[a], texts[b]), cmp.Compare(decl[a], decl[b]))
	})
}

// positionsOf inverts the permutation idx: at[d] is where the item with index d stands in the order idx lists.
func positionsOf(idx []int) []int {
	at := make([]int, len(idx))
	for pos, d := range idx {
		at[d] = pos
	}
	return at
}

// indexLiterals is every index as a Go integer literal.
func indexLiterals(idx []int) []string {
	out := make([]string, len(idx))
	for i, at := range idx {
		out[i] = strconv.Itoa(at)
	}
	return out
}

// found is the member Parse returns for the by-wire index i: an opaque type's literal holding the position, a dense enum's code (the first plus the position), a sparse one's entry of its sorted codes (DECISIONS 430).
func (s *enumSpec) found(v enumView) string {
	switch {
	case s.opaque():
		return fmt.Sprintf(foundOpaqueFormat, s.name, v.ByWire)
	case s.first != "":
		return fmt.Sprintf(foundDenseFormat, s.name, goInt64, v.ByWire, s.first)
	}
	return fmt.Sprintf(foundSparseFormat, v.CodeKeys, v.ByWire)
}

// byCode is the member indexes sorted by code.
func (s *enumSpec) byCode() []int {
	return order(len(s.consts), func(a, b int) int { return cmp.Compare(s.consts[a].code, s.consts[b].code) })
}

// constNames is the constant of every member.
func (s *enumSpec) constNames() []string {
	out := make([]string, len(s.consts))
	for i, c := range s.consts {
		out[i] = c.name
	}
	return out
}

// memberRefs is what a lookup returns for every member: its constant, or for an opaque type the literal holding its position.
func (s *enumSpec) memberRefs() []string {
	if !s.opaque() {
		return s.constNames()
	}
	out := make([]string, len(s.consts))
	for i, c := range s.consts {
		out[i] = fmt.Sprintf(idLiteralFormat, c.value)
	}
	return out
}

// writeRetired writes Retired: a string of flags, one byte per member in the order of the table the receiver is searched in (the sorted keys, or the codes), indexed by that position, or by the value itself when it is the index (DECISIONS 395).
func (g *gen) writeRetired(s *enumSpec, v enumView, search string, idx []int) {
	flags := make([]byte, len(idx))
	for i, at := range idx {
		flags[i] = flagLive
		if s.consts[at].retired {
			flags[i] = flagRetired
		}
	}
	v.Flags, v.Search = strconv.Quote(string(flags)), search
	g.exec(tmplEnumRetired, v)
}

// writeLiveMembers writes <E>LiveMembers beside <E>Members: the members not retired in declaration order: liveness is decided here, so a duplicate wire cannot hide a member.
func (g *gen) writeLiveMembers(s *enumSpec, v enumView) {
	live := make([]byte, len(s.consts))
	for i, c := range s.consts {
		live[i] = flagRetired
		if !c.retired {
			live[i] = flagLive
		}
	}
	g.printf(liveMembersFormat, g.names.LiveMembersName(s.name), g.use(iterPkg, iterPkg), s.name, v.Members, g.names.MembersName(s.name), strconv.Quote(string(live)), flagRetired, v.Item)
}
