package gogen

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// leafDisc is what a decoder holds of the discriminant of field f's type application app: the discriminant itself, read down ir.DiscFields' path, or, for a field decided through a ref, the ref's key, which f's build-time table looks the branch up by (CODEGEN.md §5.6, DECISIONS 350); the table's variable is "" for the first.
func (g *gen) leafDisc(owner *body, f *ir.Field, app ir.TypeRef) (string, string) {
	if f == nil || f.Branches == nil {
		return g.dependentDisc(owner, app), ""
	}
	path := ir.DiscRefFields(owner.fields, app)
	name := g.names.BranchesName(f)
	if path == nil || name == "" {
		g.fail(newDetail(errDependentDisc, g.at, dependentDiscFormat, g.at))
		return "", ""
	}
	return g.pathExpr(path, true), name
}

// pathExpr reads path from out's storage, a record of another package through its getter (CODEGEN.md §5.6); with key, the last field is a ref read as its key.
func (g *gen) pathExpr(path []*ir.Field, key bool) string {
	steps := []string{g.lc.Out}
	for i, f := range path {
		in, ok := path[max(i-1, 0)].Type.Named.(*ir.Record)
		foreign := i > 0 && ok && in.Pkg != g.p.Name
		sl := g.names.Slot(f)
		getter, store := sl.Getter, sl.Store
		if key && i == len(path)-1 {
			getter, store = sl.KeyGetter, sl.KeyStore
		}
		if foreign {
			steps = append(steps, getter+callSuffix)
			continue
		}
		steps = append(steps, store)
	}
	return strings.Join(steps, dot)
}

// branchLookup looks the discriminant of l's branch up in its table by the ref's key l.disc, refusing a key the table lacks with the text of a member no branch covers (CODEGEN.md §5.6, DECISIONS 350); it returns the local holding it.
func (g *gen) branchLookup(b *strings.Builder, l leaf, loc location) string {
	disc, ok := g.temp(tempValue), g.temp(tempOK)
	fmt.Fprintf(b, parsedFormat, disc, ok, l.table+lbracket+l.disc+rbracket, g.errAt(loc, noBranchText, ""))
	return disc
}

// branchTables writes each build-time table the decoders read: a map from the ref's key to the first discriminant member of the arm it selects, which the dependent type's decoder switches on (CODEGEN.md §5.6, DECISIONS 350).
func (g *gen) branchTables() {
	for _, bt := range g.names.BranchTables() {
		g.branchTable(bt)
	}
}

// branchTable writes one table, its entries in the IR's key order.
func (g *gen) branchTable(bt ir.NamedBranches) {
	defer g.enter(bt.Name)()
	path := ir.DiscRefFields(bt.Fields, *ir.HeldApp(bt.Field.Type))
	d, ok := ir.HeldApp(bt.Field.Type).Named.(*ir.Dependent)
	if path == nil || !ok {
		g.fail(newDetail(errDependentDisc, g.at, dependentDiscFormat, g.at))
		return
	}
	ref := path[len(path)-1].Type
	discType, labels := g.discLabels(d)
	if labels == nil {
		g.fail(newDetail(errDependentNoDisc, d.QName(), dependentNoDiscFormat, d.QName()))
		return
	}
	g.printf(branchTableOpenFormat, bt.Name, g.keyType(ref), discType)
	for i, k := range bt.Field.Branches.Keys {
		arm := d.Branches[bt.Field.Branches.Branches[i]]
		g.printf(branchEntryFormat, g.keyLit(ref, k), labels[arm.Members[0]])
	}
	g.printf(closeBrace)
}

// recordBranch is the dependent type of slot s and the branch the record value r selects for its value v: through its build-time table when a ref decides it, else down ir.DiscFields' path (CODEGEN.md §5.6). An empty list needs none (ir.EmptyListDependent, DECISIONS 307).
func (g *gen) recordBranch(owner *body, s *slot, r *value.Record, v value.Value) (*ir.Dependent, int) {
	if d, ok := ir.EmptyListDependent(s.T, v); ok {
		return d, 0
	}
	if s.src != nil && s.src.Branches != nil {
		return g.refBranch(owner, s.src, r)
	}
	return g.bakedBranch(owner, r, *ir.HeldApp(s.T))
}

// refBranch is the dependent type of field f and the branch the record value r selects through f's build-time table (CODEGEN.md §5.6, DECISIONS 350); nil when r holds no key the table knows, which a checked value never does.
func (g *gen) refBranch(owner *body, f *ir.Field, r *value.Record) (*ir.Dependent, int) {
	d, ok := ir.HeldApp(f.Type).Named.(*ir.Dependent)
	br, found := ir.RefBranch(owner.fields, f, r)
	if !ok || !found {
		g.fail(newDetail(errDependentNever, g.at, dependentNeverFormat, g.at)) // E3801, E3802 at verification
		return nil, 0
	}
	return d, br
}
