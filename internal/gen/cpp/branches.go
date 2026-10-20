package cppgen

import (
	"cmp"
	_ "embed"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

//go:embed text/branch_lookup.txt
var branchLookupText string

// leafDisc is what a decoder passes for the discriminant of field f's type application app: the discriminant, down ir.DiscFields' path, or, for a field decided through a ref, the ref's key with the table that key looks the branch up in (CODEGEN.md §5.6, DECISIONS 350); the table is "" for the first.
func (g *gen) leafDisc(fields []*ir.Field, f *ir.Field, app ir.TypeRef) (string, string) {
	if f.Branches == nil {
		return g.discExpr(fields, app), ""
	}
	path, name := ir.DiscRefFields(fields, app), g.pl.BranchesName(f)
	if path == nil || name == "" {
		g.malformed(dependentBadPath, g.at)
		return "", ""
	}
	return g.pathExpr(path), name
}

// openRefKey reports a ref whose key is an enum the emit opens: its branch table is keyed by the wire, in wire byte order, not by the class (CODEGEN.md §5.6, DECISIONS 362).
func (g *gen) openRefKey(ref ir.TypeRef) bool {
	e, ok := ir.RefKeyEnum(ref)
	return ok && g.open(e)
}

// pathExpr reads path from what the decoder has already decoded, each field through its getter, a reader's first from its local (CODEGEN.md §2.8, §5.6).
func (g *gen) pathExpr(path []*ir.Field) string {
	calls := []string{outVar}
	if g.dst == "" {
		m, err := g.member(path[0].Name)
		g.fail(err)
		calls, path = []string{m}, path[1:]
	}
	for _, f := range path {
		calls = append(calls, g.getterName(f)+callSuffix)
	}
	return strings.Join(calls, memberAccess)
}

// branchCall searches l's table for the ref's key l.disc and calls call with the discriminant it holds, or refuses the value at key with the text of a member no branch covers (CODEGEN.md §5.6, DECISIONS 350).
func (g *gen) branchCall(depth int, l leaf, call func(depth int, disc string), key string) {
	searched := fmt.Sprintf(branchKeyFormat, depth)
	if g.wireTables[l.table] { // keyed by the wire (CODEGEN.md §5.6)
		searched = fmt.Sprintf(helperFormat, ir.CppToWire, searched)
	}
	text := fmt.Sprintf(branchLookupText, depth, l.table, l.disc, fmt.Sprintf(noBranchAtFormat, key), searched)
	for line := range strings.SplitSeq(strings.TrimSuffix(text, newline), newline) {
		g.c.linef(depth, lineFormat, line)
	}
	found := fmt.Sprintf(branchFoundFormat, l.table, depth)
	if d, ok := l.t.Named.(*ir.Dependent); ok && g.openDisc(d) { // the table holds the address of the member
		found = derefStar + found
	}
	call(depth+depthTwo, found)
	g.c.linef(depth+1, closeBrace)
	g.c.linef(depth, closeBrace)
}

// branchTables writes each build-time table the decoders read, in detail: the ref's keys in the order the lookup compares them, each with the first discriminant member of the arm it selects, which Decode<Alias> switches on (CODEGEN.md §5.6, DECISIONS 350).
func (g *gen) branchTables() {
	for _, bt := range g.pl.BranchTables() {
		g.branchTable(bt)
	}
}

// branchTable writes one table.
func (g *gen) branchTable(bt ir.NamedBranches) {
	leave := g.enter(bt.Name)
	defer leave()
	app := ir.HeldApp(bt.Field.Type)
	path := ir.DiscRefFields(bt.Fields, *app)
	d, ok := app.Named.(*ir.Dependent)
	if path == nil || !ok {
		g.malformed(dependentBadPath, g.at)
		return
	}
	ref := path[len(path)-1].Type
	labels := g.discLabels(d)
	if d.Disc.Kind == types.Bool {
		labels = []string{strconv.FormatBool(false), strconv.FormatBool(true)} // a bool table holds bools: no narrowing from 0 and 1
	}
	entries := g.branchEntries(ref, bt.Field.Branches)
	if g.openRefKey(ref) {
		g.wireTables[bt.Name] = true
	}
	format, address := branchesOpenFormat, ""
	if g.openDisc(d) { // an opened enum's member is a static const: the table holds its address, so it stays a constant expression
		format, address = branchesOpenAddressFormat, addressOf
	}
	g.c.printf(format, g.branchKeyType(ref), g.global(*d.Disc), len(entries), bt.Name)
	for _, e := range entries {
		g.c.linef(1, defineEntryFormat, e.key, address+labels[d.Branches[e.branch].Members[0]])
	}
	g.c.line(definesClose)
	g.c.blank()
}

// branchEntry is one key of a table as C++ writes it, with its branch and its rank in C++'s order; wire is the key's wire text, the order of a table keyed by the wire.
type branchEntry struct {
	key, wire string
	branch    int
	rank      int64
}

// branchEntries are t's keys, ranked as the lookup's `<` compares the ref's key: strings by bytes and integers by value (the IR's order), an enum class by its enumerator's value, its @codes code or else its member index (CODEGEN.md §5.2).
func (g *gen) branchEntries(ref ir.TypeRef, t *ir.BranchTable) []branchEntry {
	out := make([]branchEntry, len(t.Keys))
	e, isEnum := ir.RefKeyEnum(ref)
	byWire := isEnum && g.open(e)
	for i, k := range t.Keys {
		out[i] = branchEntry{key: quote(k.S), branch: t.Branches[i], rank: int64(i)}
		switch {
		case k.IsInt:
			out[i].key = intLit(k.I)
		case isEnum:
			m := memberIndex(e, k.S)
			if m < 0 {
				g.malformed(dependentBadPath, g.at)
				continue
			}
			out[i].key, out[i].rank = g.qualifiedOwn(e)+scopeSep+g.pl.Enumerator(e.Members[m]), enumeratorValue(e, m)
			if byWire {
				out[i].wire = e.Members[m].Wire
				out[i].key = quote(out[i].wire)
			}
		}
	}
	if byWire {
		slices.SortStableFunc(out, func(a, b branchEntry) int { return cmp.Compare(a.wire, b.wire) })
		return out
	}
	slices.SortStableFunc(out, func(a, b branchEntry) int { return cmp.Compare(a.rank, b.rank) })
	return out
}

// enumeratorValue is the value the C++ enum class gives member m of e: its code under @codes, else its index (enum writes them so).
func enumeratorValue(e *ir.Enum, m int) int64 {
	if e.Codes != nil {
		return e.Members[m].Code
	}
	return int64(m)
}

// branchKeyType is a table's key type: std::string_view for a string key, else the key's own.
func (g *gen) branchKeyType(ref ir.TypeRef) string {
	if e, ok := ir.RefKeyEnum(ref); ok && !g.open(e) {
		return g.qualifiedOwn(e)
	}
	if ref.Key != nil && ref.Key.Kind == types.Int {
		return g.storage(*ref.Key)
	}
	return cppStringView
}

// recordBranch is the dependent type of field f and the branch the record value r selects for its value v: through its build-time table when a ref decides it (DECISIONS 350), else down ir.DiscFields' path. An empty list needs none (ir.EmptyListDependent, DECISIONS 307).
func (g *gen) recordBranch(fields []*ir.Field, f *ir.Field, r *value.Record, v value.Value) (*ir.Dependent, int) {
	if d, ok := ir.EmptyListDependent(f.Type, v); ok {
		return d, 0
	}
	app := *ir.HeldApp(f.Type)
	if f.Branches == nil {
		return g.bakedBranch(fields, app, r)
	}
	d, ok := app.Named.(*ir.Dependent)
	br, found := ir.RefBranch(fields, f, r)
	if !ok || !found {
		g.malformed(dependentNoBranch, g.at)
		return nil, 0
	}
	return d, br
}
