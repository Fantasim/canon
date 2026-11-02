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
	//go:embed text/enum_retired.txt
	retiredText string
	//go:embed text/enum_from_code.txt
	fromCodeText string
	//go:embed text/enum_dense_ordinal.txt
	denseOrdinalText string
)

// search is a lower-bound binary search over sorted keys: the function's signature, the lines
// before the search (its tables, its key), the keys (whose size bounds it), the format of the
// key at an index (the keys' own element when empty), the key, what a hit and a miss return.
type search struct {
	sig, prelude, keys, at, key, hit, miss string
}

// text is the search as a function definition of lower_bound.txt.
func (s search) text() string {
	at := s.at
	if at == "" {
		at = fmt.Sprintf(indexFormat, s.keys, indexHole)
	}
	return fmt.Sprintf(lowerBoundText, s.sig, s.prelude, s.keys, s.key, s.hit, s.miss, fmt.Sprintf(at, midVar), fmt.Sprintf(at, loVar))
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

func rawWire(m enumMember) string     { return m.wire }
func retiredFlag(m enumMember) string { return strconv.FormatBool(m.retired) }
func rawCanon(m enumMember) string    { return m.canon }
func codeText(m enumMember) string    { return intLit(m.code) }

// views are a string table's items: past sizedTableMin entries each a sized std::string_view, which
// no compiler measures in constant evaluation (clang's step limit), else literals (DECISIONS 392).
func views(raw []string) []string {
	out := make([]string, len(raw))
	for i, s := range raw {
		if len(raw) >= sizedTableMin {
			out[i] = fmt.Sprintf(viewFormat, quote(s), len(s))
			continue
		}
		out[i] = viewLit(s)
	}
	return out
}

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

// closedDefinitions are a closed enum's or a kind enum's tables, ToName and ToWire indexing them by ordinal, <E>FromWire searching the positions sorted by wire, <E>FromCode searching the codes through detail::<E>Ordinal (CODEGEN.md §5.2, DECISIONS 430).
func (g *gen) closedDefinitions(s enumSpec) {
	sigs, h := g.signatures(s, false), g.pl.EnumHelpers(s.name)
	g.enumTables(s)
	pos := fmt.Sprintf(staticCastFormat, cppSize, enumArg)
	if s.codes != nil {
		pos = fmt.Sprintf(helperFormat, detailPrefix+h.Ordinal, enumArg)
	}
	names, wires := detailPrefix+h.Names, detailPrefix+h.Wires
	if s.plainWires {
		wires = names
	}
	g.c.printf(byOrdinalText, sigs.toName, pos, names)
	g.c.printf(byOrdinalText, sigs.toWire, pos, wires)
	g.c.printf(retiredText, sigs.retired, len(s.members), retiredTable, indentBlock(braced(column(s.members, inOrder(len(s.members)), retiredFlag))), pos)
	order := detailPrefix + h.ByWire
	hit := fmt.Sprintf(staticCastFormat, s.name, fmt.Sprintf(indexFormat, order, loVar))
	if s.codes != nil {
		hit = fmt.Sprintf(indexFormat, h.Members, fmt.Sprintf(indexFormat, order, loVar))
	}
	g.c.write(byWireSearch(sigs.fromWire, wireParam, wires, order, hit))
	if s.codes != nil {
		g.c.printf(fromCodeText, sigs.fromCode, detailPrefix+h.Ordinal, s.name, h.Members)
	}
}

// byWireSearch is the search of the positions order (sorted by wire) for the wire key among wires, what a hit returns, and no value otherwise (CODEGEN.md §5.2, §5.3; DECISIONS 430).
func byWireSearch(sig, key, wires, order, hit string) string {
	return search{
		sig: sig, keys: order, at: fmt.Sprintf(byWireAtFormat, wires, order), key: key, hit: hit, miss: cppNullopt,
	}.text()
}

// enumTables write a closed enum's, a kind enum's or an id enum's names (not an id enum's), its wires unless they are its names, and its positions sorted by wire, once in the .gen.cpp: in detail's unnamed namespace, local to this file (CODEGEN.md §5.2, §5.3; DECISIONS 430).
func (g *gen) enumTables(s enumSpec) {
	h, all := g.pl.EnumHelpers(s.name), inOrder(len(s.members))
	g.c.line(detailOpen)
	g.c.line(anonOpen)
	if !s.id {
		g.c.printf(localTableFormat, cppStringView, len(all), h.Names, braced(views(column(s.members, all, rawCanon))))
	}
	if !s.plainWires {
		g.c.printf(localTableFormat, cppStringView, len(all), h.Wires, braced(views(column(s.members, all, rawWire))))
	}
	g.c.printf(localTableFormat, underlying(nil, len(all)), len(all), h.ByWire, braced(intTexts(byWire(s.members))))
	g.c.line(anonClose)
	g.c.line(detailClose)
	g.c.blank()
}

// wireRanks are the places of the members in the order of their wires: rank[member] is the member's index in order.
func wireRanks(order []int) []int {
	rank := make([]int, len(order))
	for place, member := range order {
		rank[member] = place
	}
	return rank
}

// openDefinitions are an opened enum's ToName, IsKnown, <E>FromWire and, with @codes, ToCode and <E>FromCode: searches of its wires sorted by bytes, held once with their parallel names (only when one differs from its wire) and codes in an unnamed namespace, or of its codes; ToName is ToWire when every name is its wire (CODEGEN.md §5.2, DECISIONS 383, 430).
func (g *gen) openDefinitions(s enumSpec) {
	sigs, h := g.signatures(s, true), g.pl.EnumHelpers(s.name)
	g.openWireTables(s, h)
	wires, key := detailPrefix+h.Wires, fmt.Sprintf(wireKeyLineFormat, ir.CppToWire)
	if s.plainWires {
		g.c.printf(openPlainNameFormat, sigs.toName, ir.CppToWire)
	} else {
		g.c.write(search{sig: sigs.toName, prelude: key, keys: wires, key: keyParam, hit: fmt.Sprintf(indexFormat, detailPrefix+h.WireNames, loVar), miss: keyParam}.text())
	}
	g.c.write(search{
		sig: sigs.retired, prelude: key + table(cppBool, retiredTable, column(s.members, byWire(s.members), retiredFlag)),
		keys: wires, key: keyParam, hit: fmt.Sprintf(indexFormat, retiredTable, loVar), miss: strconv.FormatBool(false),
	}.text())
	g.c.write(search{sig: sigs.known, prelude: key, keys: wires, key: keyParam, hit: strconv.FormatBool(true), miss: strconv.FormatBool(false)}.text())
	g.c.write(search{
		sig: sigs.fromWire, keys: wires, key: wireParam,
		hit: fmt.Sprintf(helperFormat, s.name, fmt.Sprintf(stringOfFormat, wireParam)), miss: cppNullopt,
	}.text())
	if s.codes == nil {
		return
	}
	g.c.write(search{sig: sigs.toCode, prelude: key, keys: wires, key: keyParam, hit: fmt.Sprintf(indexFormat, detailPrefix+h.WireCodes, loVar), miss: cppNullopt}.text())
	g.openFromCode(s, sigs.fromCode, h)
}

// openFromCode is an opened enum's <E>FromCode: a search of its codes ascending, beside the places of their members' wires in k<E>Wires, which it reads (DECISIONS 383, 430).
func (g *gen) openFromCode(s enumSpec, sig string, h ir.CppEnumHelpers) {
	byCodes, rank := byCode(s.members), wireRanks(byWire(s.members))
	places := make([]int, len(byCodes))
	for i, at := range byCodes {
		places[i] = rank[at]
	}
	wire := fmt.Sprintf(indexFormat, detailPrefix+h.Wires, fmt.Sprintf(indexFormat, wireAtTable, loVar))
	g.c.write(search{
		sig: sig, prelude: table(intType(*s.codes), keysTable, column(s.members, byCodes, codeText)) + table(underlying(nil, len(places)), wireAtTable, intTexts(places)),
		keys: keysTable, key: codeField, hit: fmt.Sprintf(helperFormat, s.name, fmt.Sprintf(stringOfFormat, wire)), miss: cppNullopt,
	}.text())
}

// openWireTables write an opened enum's wires sorted by bytes, with their names (only when one differs from its wire) and codes, once in the .gen.cpp: in detail's unnamed namespace, local to this file (CODEGEN.md §5.2).
func (g *gen) openWireTables(s enumSpec, h ir.CppEnumHelpers) {
	order := byWire(s.members)
	g.c.line(detailOpen)
	g.c.line(anonOpen)
	g.c.printf(localTableFormat, cppStringView, len(order), h.Wires, braced(views(column(s.members, order, rawWire))))
	if !s.plainWires {
		g.c.printf(localTableFormat, cppStringView, len(order), h.WireNames, braced(views(column(s.members, order, rawCanon))))
	}
	if s.codes != nil {
		g.c.printf(localTableFormat, intType(*s.codes), len(order), h.WireCodes, braced(column(s.members, order, codeText)))
	}
	g.c.line(anonClose)
	g.c.line(detailClose)
	g.c.blank()
}

// helperSigs are an enum's helper signatures: ToName, ToWire (closed), IsKnown (opened), IsRetired, <E>FromWire, ToCode (opened, @codes) and <E>FromCode (@codes); "" for one it has not.
type helperSigs struct {
	toName, toWire, known, retired, fromWire, toCode, fromCode string
}

// all are the signatures it has, in the order the header declares them and the .gen.cpp defines them (CODEGEN.md §5.2).
func (h helperSigs) all() []string {
	var out []string
	for _, sig := range []string{h.toName, h.toWire, h.known, h.retired, h.fromWire, h.toCode, h.fromCode} {
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
		out.retired = fmt.Sprintf(openRetiredSigFormat, s.name)
	} else {
		out.toWire, out.retired = fmt.Sprintf(nameSigFormat, ir.CppToWire, s.name), fmt.Sprintf(retiredSigFormat, s.name)
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
