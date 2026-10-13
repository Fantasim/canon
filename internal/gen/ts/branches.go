package tsgen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// branchDisc is the discriminant a reader passes for field f decided through a ref: its table's entry for the ref's key, which the reader already holds (CODEGEN.md §5.6, DECISIONS 350). A key the table lacks gives undefined, which the dependent type's reader refuses as a member no branch covers.
func (g *gen) branchDisc(rd *reading, f *ir.Field, app ir.TypeRef) string {
	path := ir.DiscRefFields(rd.fields, app)
	tables := g.tables
	i := slices.IndexFunc(tables, func(b ir.NamedBranches) bool { return b.Field == f })
	if len(path) == 0 || rd.locals[path[0]] == "" || i < 0 {
		g.failf(ErrMalformed, malformedNoDisc, g.at)
		return ""
	}
	if !slices.ContainsFunc(g.branches, func(b ir.NamedBranches) bool { return b.Field == f }) {
		g.branches = append(g.branches, tables[i])
	}
	parts := []string{rd.locals[path[0]]}
	for _, pf := range path[1:] {
		parts = append(parts, fieldProp(pf))
	}
	return fmt.Sprintf(branchGetFormat, tables[i].Name, strings.Join(parts, dot))
}

// branchTables writes each table the readers read, in the order they first read one: a Map from the ref's key to the first discriminant member of the arm it selects, which the dependent type's reader switches on (CODEGEN.md §5.6, DECISIONS 350).
func (g *gen) branchTables() {
	for i := 0; i < len(g.branches); i++ {
		g.add(g.branchTable(g.branches[i]))
	}
}

// branchTable is one table's declaration, not exported.
func (g *gen) branchTable(bt ir.NamedBranches) string {
	defer g.enter(bt.Name)()
	app := ir.HeldApp(bt.Field.Type)
	path := ir.DiscRefFields(bt.Fields, *app)
	d, ok := app.Named.(*ir.Dependent)
	if path == nil || !ok {
		g.failf(ErrMalformed, malformedNoDisc, g.at)
		return ""
	}
	ref, field := path[len(path)-1].Type, path[len(path)-1]
	discType, labels := g.discLabels(d)
	if labels == nil {
		g.failf(ErrMalformed, malformedNoDisc, d.QName())
		return ""
	}
	keyType := g.tsType(ref, field.BigInt)
	entries := make([]string, len(bt.Field.Branches.Keys))
	for i, k := range bt.Field.Branches.Keys {
		arm := d.Branches[bt.Field.Branches.Branches[i]]
		entries[i] = fmt.Sprintf(branchEntryFormat, g.branchKey(ref, field.BigInt, k), labels[arm.Members[0]])
	}
	name := g.declare(bt.Name, g.at)
	return fmt.Sprintf(branchTableFormat, name, keyType, discType, keyType, discType, strings.Join(entries, ""))
}

// branchKey is a key of a ref as TypeScript holds it: an enum member's wire, a number or a bigint, else the string (CODEGEN.md §4.1).
func (g *gen) branchKey(ref ir.TypeRef, big bool, k value.Key) string {
	if !k.IsInt {
		if e, ok := keyEnum(ref); ok {
			if i := slices.IndexFunc(e.Members, func(m *ir.EnumMember) bool { return m.Name == k.S }); i >= 0 {
				return quote(e.Members[i].Wire)
			}
			g.failf(ErrMalformed, malformedNoDisc, g.at)
		}
		return quote(k.S)
	}
	if bigAt(ref, big) {
		return strconv.FormatInt(k.I, decimal) + bigSuffix
	}
	return strconv.FormatInt(k.I, decimal)
}

// keyEnum is the enum keying a ref into a keyed list keyed by one.
func keyEnum(ref ir.TypeRef) (*ir.Enum, bool) {
	if ref.Key == nil || ref.Key.Kind != types.Enum {
		return nil, false
	}
	e, ok := ref.Key.Named.(*ir.Enum)
	return e, ok
}

// refBranch is the branch the record of o selects for its field f through f's build-time table (CODEGEN.md §5.6, DECISIONS 350); -1 when it holds no key the table knows, which a checked value never does.
func (g *gen) refBranch(d *ir.Dependent, o *owner) int {
	br, ok := ir.RefBranch(o.fields, o.field, o.rec)
	if !ok {
		g.failf(ErrMalformed, malformedBranch, d.QName())
		return -1
	}
	return br
}
