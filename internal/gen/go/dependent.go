package gogen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// dependentType writes a dependent type's struct and accessors, used or not (CODEGEN.md §5.6); its branch enum is in the enums section (§2.7, branchEnums). Stage E refuses one every arm of which is Never (E8019 DependentType).
func (g *gen) dependentType(d *ir.Dependent) {
	defer g.enter(d.QName())()
	if len(d.Branches) == 0 {
		g.fail(newDetail(errDependentNoBranch, g.at, dependentNoBranchFormat, g.at))
		return
	}
	n := g.names.Dependent(d)
	g.body.WriteString(docFor(n.Type, d.Doc))
	if n.DefineStore != "" {
		g.printf(defineVariantFormat, n.Type, n.Branch, n.BranchStore, n.ValueStore, n.Method, n.DefineStore)
	} else {
		g.printf(variantFormat, n.Type, n.Branch, n.BranchStore, n.ValueStore, n.Method)
	}
	for i, b := range n.Branches {
		g.dependentAccessor(n, b, d.Branches[i])
	}
}

// branchEnums writes the branch enum of every dependent type, in declaration order (CODEGEN.md §2.7, §5.6).
func (g *gen) branchEnums() {
	for _, t := range g.p.Types {
		if d, ok := t.(*ir.Dependent); ok {
			g.writeBranchEnum(g.names.Dependent(d))
		}
	}
}

// writeBranchEnum writes TBranch and its members in arm order, no String, Wire or Parse<…> (CODEGEN.md §5.6).
func (g *gen) writeBranchEnum(n ir.GoDependent) {
	g.printf(enumHeaderFormat, n.Branch, smallestUint(len(n.Branches)))
	for i, b := range n.Branches {
		g.printf(enumConstFormat, b.Member, n.Branch, strconv.Itoa(i))
	}
	g.printf(closeParenFormat)
}

// dependentAccessor writes As<Branch>, zero unless the value holds it, and a define branch's As<Branch>Value (CODEGEN.md §5.6, §5.8; DECISIONS 298).
func (g *gen) dependentAccessor(n ir.GoDependent, b ir.GoBranch, branch *ir.Branch) {
	t := g.goType(branch.Type)
	g.printf(dependentAsFormat, n.Type, b.As, t, n.BranchStore, b.Member, n.ValueStore)
	if b.AsValue != "" {
		g.printf(dependentAsValueFormat, n.Type, b.AsValue, n.BranchStore, b.Member, n.DefineStore)
	}
}

// branchDefines is the define table a define branch refs; one the IR does not hold is malformed.
func (g *gen) branchDefines(t ir.TypeRef) *ir.DefineTable {
	d := ir.DefinesOf(g.p, ir.DefineTarget(t))
	if d == nil {
		g.fail(newDetail(errDependentValue, g.at, dependentValueFormat, g.at))
	}
	return d
}

// decodeDependent writes decode<T> of a dependent type a decoded class holds: the untagged wire switches on the discriminant its caller already decoded, an enum or a Bool (CODEGEN.md §5.6, §6.1); a member no branch covers (a Never arm's) is refused with the same text gen/cpp writes (WIRE.md §5.9).
func (g *gen) decodeDependent(d *ir.Dependent) {
	defer g.enter(d.QName())()
	defer g.withRT(d.Pkg)()
	g.temps = 0
	discType, labels := g.discLabels(d)
	if labels == nil {
		g.fail(newDetail(errDependentNoDisc, d.QName(), dependentNoDiscFormat, d.QName()))
		return
	}
	lc, out := g.lc, g.lc.Out
	if d.Pkg != g.p.Name {
		out = lc.Dst // another package's dependent type: this package's reader builds it (CODEGEN.md §2.8)
	}
	disc := g.names.DataLocal(localDisc)
	g.printf(dependentFuncOpenFormat, g.decodeFunc(d), lc.Name, lc.Path, lc.Raw, g.rawType(), disc, discType, out, g.typeName(d))
	g.printf(switchTagFormat, disc)
	for i, br := range d.Branches {
		cases := make([]string, len(br.Members))
		for j, m := range br.Members {
			cases[j] = labels[m]
		}
		g.printf(caseFormat, strings.Join(cases, listSep))
		var b strings.Builder
		x := g.readValue(&b, leaf{t: br.Type}, lc.Raw, g.root())
		g.body.WriteString(b.String())
		g.body.WriteString(g.storeBranch(d, i, x, g.branchDefineRead(i, d, x)))
		g.body.WriteString(returnKw + nilLit + newline)
	}
	g.printf(unknownCaseFormat, g.errAt(g.root(), noBranchText, ""))
	g.body.WriteString(closeBrace + newline)
}

// branchDefineRead reads a define branch's value right after its key, "" for any other branch.
func (g *gen) branchDefineRead(i int, d *ir.Dependent, key string) string {
	if g.names.Dependent(d).Branches[i].AsValue == "" {
		return ""
	}
	var b strings.Builder
	v := g.readBranchDefine(&b, d.Branches[i].Type, key)
	g.body.WriteString(b.String())
	return v
}

// storeBranch stores branch i read as x (and its define value v) into out, or builds another package's through its branch's make hook (CODEGEN.md §5.14).
func (g *gen) storeBranch(d *ir.Dependent, i int, x, v string) string {
	lc, n := g.lc, g.names.Dependent(d)
	if d.Pkg != g.p.Name {
		args := []string{x}
		if v != "" {
			args = append(args, v)
		}
		return fmt.Sprintf(cellStoreFormat, pointer+lc.Dst, g.qualify(d.Pkg, g.names.MakeBranchName(d, d.Branches[i]))+callArgs(args))
	}
	var s string
	if v != "" {
		s = fmt.Sprintf(assignFormat, lc.Out, n.DefineStore, v)
	}
	return s + fmt.Sprintf(assignPairFormat, lc.Out+dot+n.BranchStore, lc.Out+dot+n.ValueStore, n.Branches[i].Member, x)
}

// readBranchDefine looks a define branch's key up once read into a local, with the load error of a define field (CODEGEN.md §5.8; DECISIONS 298).
func (g *gen) readBranchDefine(b *strings.Builder, t ir.TypeRef, key string) string {
	d := g.branchDefines(t)
	if d == nil {
		return zeroLit
	}
	lc, v := g.lc, g.temp(tempValue)
	prefix, k := g.splitLoc(g.root())
	fmt.Fprintf(b, defineReadFormat, v, lc.Err, g.helper(helperDefine), lc.Name, prefix, k,
		g.names.DefinesVar(d), strconv.Quote(ir.DefineTableName(d)), key)
	return v
}

// discLabels are the Go type of d's discriminant and each member's case label, in member order: false and true for a Bool, the member constants for an enum; nil labels for neither.
func (g *gen) discLabels(d *ir.Dependent) (string, []string) {
	if d.Disc != nil && d.Disc.Kind == types.Bool {
		return goBool, []string{strconv.FormatBool(false), strconv.FormatBool(true)}
	}
	e, ok := dependentDiscEnum(d)
	if !ok {
		return "", nil
	}
	labels := make([]string, len(e.Members))
	for i, m := range e.Members {
		labels[i] = g.qualify(e.Pkg, g.names.MemberName(e, m))
	}
	return g.qualify(e.Pkg, g.goName(e)), labels
}

// dependentDiscEnum is the enum a dependent type's match reads, if it reads one (CODEGEN.md §5.6).
func dependentDiscEnum(d *ir.Dependent) (*ir.Enum, bool) {
	if d.Disc == nil || d.Disc.Kind != types.Enum {
		return nil, false
	}
	e, ok := d.Disc.Named.(*ir.Enum)
	return e, ok
}

// dependentDisc is the discriminant of app, a field's type application, as the decoder already holds it: out's storage down ir.DiscFields' path, a record of another package read through its getter (CODEGEN.md §5.6). Stage E refuses any other (E8019 DependentType).
func (g *gen) dependentDisc(owner *body, app ir.TypeRef) string {
	path := g.discPath(owner, app)
	if path == nil {
		return ""
	}
	steps := []string{g.lc.Out}
	for i, f := range path {
		in, ok := path[max(i-1, 0)].Type.Named.(*ir.Record)
		if i > 0 && ok && in.Pkg != g.p.Name {
			steps = append(steps, g.names.Slot(f).Getter+callSuffix)
			continue
		}
		steps = append(steps, g.names.Slot(f).Store)
	}
	return strings.Join(steps, dot)
}

// discPath is ir.DiscFields of a dependent type's application from owner's fields; nil, refused as malformed, for any other.
func (g *gen) discPath(owner *body, app ir.TypeRef) []*ir.Field {
	if _, ok := app.Named.(*ir.Dependent); !ok {
		g.fail(newDetail(errDependentNested, g.at, dependentNestedFormat, g.at))
		return nil
	}
	path := ir.DiscFields(owner.fields, app)
	if path == nil {
		g.fail(newDetail(errDependentDisc, g.at, dependentDiscFormat, g.at))
	}
	return path
}

// readDependentValue decodes an untagged dependent-type value, the branch slotLeaf's disc selects (CODEGEN.md §5.6); stage E refuses one outside a field and its list's elements (E8019 DependentType).
func (g *gen) readDependentValue(b *strings.Builder, l leaf, raw string, loc location) string {
	d, ok := l.t.Named.(*ir.Dependent)
	if !ok || l.disc == "" {
		g.fail(newDetail(errDependentNested, g.at, dependentNestedFormat, g.at))
		return raw
	}
	v := g.temp(tempValue)
	fmt.Fprintf(b, dependentDecodeFormat, v, g.typeName(d), g.lc.Err, g.decodeFunc(d), g.lc.Name, g.locExpr(loc), raw, l.disc)
	return v
}

// assignDependent is a dependent field's storage in a baked literal: each value, the field's list elements included, in the branch its record's discriminant selects (CODEGEN.md §5.6).
func (g *gen) assignDependent(owner *body, r *value.Record, s *slot, v value.Value) []pair {
	return g.assignWith(s, v, func(s *slot, v value.Value) string {
		app := ir.HeldApp(s.T)
		d, br := g.bakedBranch(owner, r, *app)
		if d == nil {
			return nilLit
		}
		return g.dependentExpr(s.T, v, d, br)
	})
}

// bakedBranch is the dependent type app applies and the index of the branch the record value r's discriminant selects, read down ir.DiscFields' path; nil when stage E should have refused it.
func (g *gen) bakedBranch(owner *body, r *value.Record, app ir.TypeRef) (*ir.Dependent, int) {
	path := g.discPath(owner, app)
	if path == nil {
		return nil, 0
	}
	d, _ := app.Named.(*ir.Dependent)
	disc := value.Value(r)
	for _, f := range path {
		disc = g.fieldValue(as[value.Record](g, disc), f.Name)
	}
	m := -1
	switch x := disc.(type) {
	case *value.Member:
		m = x.Index
	case *value.Bool:
		m = boolIndex(x.V)
	}
	if m < 0 || m >= len(d.ByMember) || d.ByMember[m] < 0 || d.ByMember[m] >= len(d.Branches) {
		g.fail(newDetail(errDependentNever, g.at, dependentNeverFormat, g.at)) // E3801, E3802 at verification
		return nil, 0
	}
	return d, d.ByMember[m]
}

// boolIndex is a Bool discriminant's member index: false, then true (CODEGEN.md §5.6).
func boolIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}

// dependentExpr is &T{branch, value} holding v in branch br, or a list of them; the value is the branch's Go type exactly, so As<Branch>'s type assertion holds (CODEGEN.md §5.6).
func (g *gen) dependentExpr(t ir.TypeRef, v value.Value, d *ir.Dependent, br int) string {
	if t.Kind == types.List {
		return g.plainList(t, as[value.List](g, v), func(elem ir.TypeRef, x value.Value) string {
			return g.dependentExpr(elem, x, d, br)
		})
	}
	n, bt := g.names.Dependent(d), d.Branches[br].Type
	x := g.expr(bt, v)
	if d.Pkg != g.p.Name { // another package's dependent type: its branch's make hook (CODEGEN.md §5.6, §5.14)
		args := []string{x}
		if n.Branches[br].AsValue != "" {
			args = append(args, g.branchDefineValue(bt, v))
		}
		return addressOf(g.typeName(d), g.qualify(d.Pkg, g.names.MakeBranchName(d, d.Branches[br]))+callArgs(args))
	}
	if untypedLit(bt) {
		x = g.goType(bt) + lparen + x + rparen
	}
	parts := []pair{{n.BranchStore, n.Branches[br].Member}, {n.ValueStore, x}}
	if n.Branches[br].AsValue != "" {
		parts = append(parts, pair{n.DefineStore, g.branchDefineValue(bt, v)})
	}
	return ampersand + compositeLit(g.goName(d), parts)
}

// branchDefineValue is a baked define branch's value, found at generation time (CODEGEN.md §5.8).
func (g *gen) branchDefineValue(t ir.TypeRef, v value.Value) string {
	d := g.branchDefines(t)
	if d == nil {
		return zeroLit
	}
	if i, ok := slices.BinarySearch(d.Names, as[value.Ref](g, v).Key.S); ok {
		return strconv.FormatInt(d.Values[i], decimal)
	}
	g.fail(newDetail(errDependentValue, g.at, dependentValueFormat, g.at))
	return zeroLit
}

// untypedLit reports a branch type whose literal is an untyped constant of another default type: an integer, a float, an integer key.
func untypedLit(t ir.TypeRef) bool {
	if t.Kind == types.Ref {
		return !isTableRef(t.Ref) && t.Key != nil && t.Key.Kind == types.Int
	}
	return t.Kind == types.Int || t.Kind == types.Float
}
