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
	Strconv, Slices                                                                   string
	Names, Wires, Keys, Found, CodeKeys, Members, Numbers, ByCode                     string
	Dense                                                                             bool // the value of a member is its index
}

// enumView names the tables and methods of s; a table's name is the plan's (DECISIONS 393).
func (g *gen) enumView(s *enumSpec) enumView {
	table := func(role string) string { return g.names.TableName(role, s.name) }
	zero := zeroLit
	if s.id {
		zero = fmt.Sprintf(idZeroFormat, s.name)
	}
	return enumView{
		Name: s.name, Parse: g.names.ParseName(s.name), FromCode: g.names.FromCodeName(s.name), Arg: s.parseArg, Codes: s.codes, Zero: zero,
		Slices: g.use(slicesPkg, slicesPkg),
		Names:  table(ir.GoTableNames), Wires: table(ir.GoTableWires), Keys: table(ir.GoTableKeys), Found: table(ir.GoTableFound), CodeKeys: table(ir.GoTableCodes),
		Members: table(ir.GoTableMembers), Numbers: table(ir.GoTableNumbers), ByCode: table(ir.GoTableByCode),
		Dense: s.codes == "",
	}
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
func pick(items []string, idx []int) []string {
	out := make([]string, len(idx))
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

// memberRefs is what a lookup returns for every member: its constant, or for an id the literal holding its position.
func (s *enumSpec) memberRefs() []string {
	if !s.id {
		return s.constNames()
	}
	out := make([]string, len(s.consts))
	for i, c := range s.consts {
		out[i] = fmt.Sprintf(idLiteralFormat, c.value)
	}
	return out
}
