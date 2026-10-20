package cppgen

import (
	"cmp"
	_ "embed"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

var (
	//go:embed text/lower_bound.txt
	lowerBoundText string
	//go:embed text/enum_by_ordinal.txt
	byOrdinalText string
	//go:embed text/enum_from_code.txt
	fromCodeText string
)

// search is a lower-bound binary search over sorted keys: the function's signature, the lines
// before the search (its tables, its key), the keys, the key, what a hit and a miss return.
type search struct {
	sig, prelude, keys, key, hit, miss string
}

// text is the search as a function definition of lower_bound.txt.
func (s search) text() string {
	return fmt.Sprintf(lowerBoundText, s.sig, s.prelude, s.keys, s.key, s.hit, s.miss)
}

// table is a static constexpr local array of a function the .gen.cpp defines, one item per line past maxInline.
func table(typ, name string, items []string) string {
	return fmt.Sprintf(tableLineFormat, typ, len(items), name, indentBlock(braced(items)))
}

// byWire are the member positions stable-sorted by wire bytes: of equal wires, the first declared comes first (CODEGEN.md §5.2).
func byWire(members []enumMember) []int {
	return sortedPositions(members, func(a, b enumMember) int { return strings.Compare(a.wire, b.wire) })
}

// byCode are the member positions sorted by code, ascending (CODEGEN.md §5.2).
func byCode(members []enumMember) []int {
	return sortedPositions(members, func(a, b enumMember) int { return cmp.Compare(a.code, b.code) })
}

func sortedPositions(members []enumMember, by func(a, b enumMember) int) []int {
	order := inOrder(len(members))
	slices.SortStableFunc(order, func(a, b int) int { return by(members[a], members[b]) })
	return order
}

// column is one text of each member, in the given order.
func column(members []enumMember, order []int, text func(enumMember) string) []string {
	out := make([]string, len(order))
	for i, at := range order {
		out[i] = text(members[at])
	}
	return out
}

// inOrder are the positions of n members in declaration order.
func inOrder(n int) []int {
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	return order
}

func quotedWire(m enumMember) string  { return quote(m.wire) }
func quotedCanon(m enumMember) string { return quote(m.canon) }
func codeText(m enumMember) string    { return intLit(m.code) }

// enumDefinitions define, after detail, every enum's helpers in enum order: tables, never a chain or a switch over the members (CODEGEN.md §2.7, §5.2, §7.1; DECISIONS 383).
func (g *gen) enumDefinitions() {
	for _, he := range g.helperEnums() {
		leave := g.enter(he.origin)
		if he.open {
			g.openDefinitions(he.spec)
		} else {
			g.closedDefinitions(he.spec)
		}
		leave()
	}
}

// closedDefinitions are ToName and ToWire indexing arrays by ordinal, <E>FromWire searching the wires, <E>FromCode searching the codes through detail::<E>Ordinal (CODEGEN.md §5.2).
func (g *gen) closedDefinitions(s enumSpec) {
	sigs, h := g.signatures(s, false), g.pl.EnumHelpers(s.name)
	pos := fmt.Sprintf(staticCastFormat, cppSize, enumArg)
	if s.codes != nil {
		pos = fmt.Sprintf(helperFormat, detailPrefix+h.Ordinal, enumArg)
	}
	all := inOrder(len(s.members))
	g.c.printf(byOrdinalText, sigs.toName, len(s.members), namesTable, indentBlock(braced(column(s.members, all, quotedCanon))), pos)
	g.c.printf(byOrdinalText, sigs.toWire, len(s.members), wiresTable, indentBlock(braced(column(s.members, all, quotedWire))), pos)
	order := byWire(s.members)
	values := column(s.members, order, func(m enumMember) string { return s.name + scopeSep + m.name })
	g.c.write(search{
		sig: sigs.fromWire, prelude: table(cppStringView, keysTable, column(s.members, order, quotedWire)) + table(s.name, valuesTable, values),
		keys: keysTable, key: wireParam, hit: fmt.Sprintf(indexFormat, valuesTable, loVar), miss: cppNullopt,
	}.text())
	if s.codes != nil {
		g.c.printf(fromCodeText, sigs.fromCode, detailPrefix+h.Ordinal, s.name, h.Members)
	}
}

// openDefinitions are an opened enum's ToName, IsKnown, <E>FromWire and, with @codes, ToCode and <E>FromCode: searches of its wires sorted by bytes, held once with their parallel names and codes in an unnamed namespace, or of its codes (CODEGEN.md §5.2, DECISIONS 383).
func (g *gen) openDefinitions(s enumSpec) {
	sigs, h := g.signatures(s, true), g.pl.EnumHelpers(s.name)
	g.openWireTables(s, h)
	wires, key := detailPrefix+h.Wires, fmt.Sprintf(wireKeyLineFormat, ir.CppToWire)
	g.c.write(search{sig: sigs.toName, prelude: key, keys: wires, key: keyParam, hit: fmt.Sprintf(indexFormat, detailPrefix+h.WireNames, loVar), miss: keyParam}.text())
	g.c.write(search{sig: sigs.known, prelude: key, keys: wires, key: keyParam, hit: strconv.FormatBool(true), miss: strconv.FormatBool(false)}.text())
	g.c.write(search{
		sig: sigs.fromWire, keys: wires, key: wireParam,
		hit: fmt.Sprintf(helperFormat, s.name, fmt.Sprintf(stringOfFormat, wireParam)), miss: cppNullopt,
	}.text())
	if s.codes == nil {
		return
	}
	g.c.write(search{sig: sigs.toCode, prelude: key, keys: wires, key: keyParam, hit: fmt.Sprintf(indexFormat, detailPrefix+h.WireCodes, loVar), miss: cppNullopt}.text())
	byCodes := byCode(s.members)
	g.c.write(search{
		sig: sigs.fromCode, prelude: table(intType(*s.codes), keysTable, column(s.members, byCodes, codeText)) + table(cppStringView, wiresTable, column(s.members, byCodes, quotedWire)),
		keys: keysTable, key: codeField,
		hit: fmt.Sprintf(helperFormat, s.name, fmt.Sprintf(stringOfFormat, fmt.Sprintf(indexFormat, wiresTable, loVar))), miss: cppNullopt,
	}.text())
}

// openWireTables write an opened enum's wires sorted by bytes, with their names and codes, once in the .gen.cpp: in detail's unnamed namespace, local to this file (CODEGEN.md §5.2).
func (g *gen) openWireTables(s enumSpec, h ir.CppEnumHelpers) {
	order := byWire(s.members)
	g.c.line(detailOpen)
	g.c.line(anonOpen)
	g.c.printf(localTableFormat, cppStringView, len(order), h.Wires, braced(column(s.members, order, quotedWire)))
	g.c.printf(localTableFormat, cppStringView, len(order), h.WireNames, braced(column(s.members, order, quotedCanon)))
	if s.codes != nil {
		g.c.printf(localTableFormat, intType(*s.codes), len(order), h.WireCodes, braced(column(s.members, order, codeText)))
	}
	g.c.line(anonClose)
	g.c.line(detailClose)
	g.c.blank()
}

// helperSigs are an enum's helper signatures: ToName, ToWire (closed), IsKnown (opened), <E>FromWire, ToCode (opened, @codes) and <E>FromCode (@codes); "" for one it has not.
type helperSigs struct {
	toName, toWire, known, fromWire, toCode, fromCode string
}

// all are the signatures it has, in the order the header declares them and the .gen.cpp defines them (CODEGEN.md §5.2).
func (h helperSigs) all() []string {
	var out []string
	for _, sig := range []string{h.toName, h.toWire, h.known, h.fromWire, h.toCode, h.fromCode} {
		if sig != "" {
			out = append(out, sig)
		}
	}
	return out
}

// signatures are enum s's helper signatures, s opened or closed (CODEGEN.md §5.2).
func (g *gen) signatures(s enumSpec, open bool) helperSigs {
	h := g.pl.EnumHelpers(s.name)
	out := helperSigs{toName: fmt.Sprintf(nameSigFormat, ir.CppToName, s.name), fromWire: fmt.Sprintf(fromWireSigFormat, s.name, h.FromWire)}
	if open {
		out.toName, out.known = fmt.Sprintf(openNameSigFormat, s.name), fmt.Sprintf(openKnownSigFormat, s.name)
	} else {
		out.toWire = fmt.Sprintf(nameSigFormat, ir.CppToWire, s.name)
	}
	if s.codes != nil {
		code := intType(*s.codes)
		out.fromCode = fmt.Sprintf(fromCodeSigFormat, s.name, h.FromCode, code)
		if open {
			out.toCode = fmt.Sprintf(openToCodeSigFormat, code, s.name)
		}
	}
	return out
}
