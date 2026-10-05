package cppgen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// checkDependent refuses a dependent type without a Bool or enum discriminant or whose member map disagrees with its branches, and one stage E refuses (E8019 DependentType): every arm Never (CODEGEN.md §4.4, §5.6).
func (g *gen) checkDependent(d *ir.Dependent) {
	members, ok := discMembers(d)
	switch {
	case !ok:
		g.malformed(dependentNoDisc, d.Name)
	case len(d.ByMember) != members || slices.ContainsFunc(d.ByMember, func(b int) bool { return b < ir.NoBranch || b >= len(d.Branches) }):
		g.malformed(dependentBadArm, d.Name)
	case len(d.Branches) == 0:
		g.malformed(dependentNoBranch, d.Name)
	}
}

// discMembers is the number of members of d's discriminant, false and true for a Bool; false when it is neither a Bool nor an enum.
func discMembers(d *ir.Dependent) (int, bool) {
	if d.Disc == nil {
		return 0, false
	}
	if e, ok := d.Disc.Named.(*ir.Enum); ok && d.Disc.Kind == types.Enum {
		return len(e.Members), true
	}
	return boolMembers, d.Disc.Kind == types.Bool
}

// branchEnums are the branch enums of the dependent types, after the kind enums (CODEGEN.md §2.7, §5.6): no helpers, never on the wire.
func (g *gen) branchEnums() {
	for _, t := range g.p.Types {
		d, ok := t.(*ir.Dependent)
		if !ok {
			continue
		}
		n := g.pl.Dependent(d)
		g.h.printf(enumOpenFormat, n.Branch, underlying(nil, len(n.Branches)))
		for _, b := range n.Branches {
			g.h.linef(1, enumMemberFormat, b.Enumerator)
		}
		g.h.line(closeClass)
		g.h.blank()
	}
}

// dependentClass holds the branches in a std::variant indexed like the branch enum; each As<Branch> reads its alternative by index, so two branches of one storage type stay apart (CODEGEN.md §4.2, §4.3, §5.6).
func (g *gen) dependentClass(d *ir.Dependent) {
	n := g.pl.Dependent(d)
	g.doc(0, d.Doc)
	g.h.printf(classOpenFormat, n.Class)
	g.h.linef(1, branchGetterFormat, n.Branch, n.GetBranch, n.Branch)
	alts := make([]string, len(d.Branches))
	for i, b := range d.Branches {
		alts[i] = g.storage(b.Type)
		if g.byValue(b.Type) {
			g.h.linef(1, branchValueFormat, alts[i], n.Branches[i].As, i, alts[i])
		} else {
			g.h.linef(1, asGetterFormat, alts[i], n.Branches[i].As, i)
		}
		if n.Branches[i].AsValue != "" {
			g.h.linef(1, branchDefineFormat, n.Branches[i].AsValue, i, n.DefineValue)
		}
	}
	g.h.blank()
	g.h.line(privateLabel)
	g.h.linef(1, friendAccessFormat, g.pl.AccessName())
	g.h.linef(1, friendAccessFormat, g.pl.MakeStruct(g.p.Name))
	if !g.baked() {
		g.h.linef(1, friendDependentFormat, n.Decode, g.global(*d.Disc), g.qualifiedOwn(d))
	}
	g.h.blank()
	g.h.linef(1, variantMemberFormat, strings.Join(alts, listSep))
	if n.DefineValue != "" {
		g.h.linef(1, memberFormat, cppInt64, n.DefineValue, initZero)
	}
	g.h.line(closeClass)
	g.h.blank()
}

// dependentDecodeDecl declares detail's Decode<Alias> (CODEGEN.md §2.7 step 4, §5.6).
func (g *gen) dependentDecodeDecl(d *ir.Dependent) {
	n := g.pl.Dependent(d)
	g.h.printf(dependentDeclFormat, n.Decode, g.global(*d.Disc), g.qualifiedOwn(d))
}

// dependentDecoder is Decode<Alias>: the discriminant, read first by the holder, picks the branch through a switch generated from the match; a member whose arm is Never takes no value (CODEGEN.md §5.6; WIRE.md §5.9).
func (g *gen) dependentDecoder(d *ir.Dependent, name string) {
	n := g.pl.Dependent(d)
	g.c.printf(dependentOpenFormat, name, g.global(*d.Disc), g.qualifiedOwn(d))
	g.c.linef(1, switchFormat, g.switchArg(d))
	labels := g.discLabels(d)
	for i := range d.Branches {
		var mine []string
		for m, arm := range d.ByMember {
			if arm == i {
				mine = append(mine, labels[m])
			}
		}
		g.dependentCase(mine, i, d, n.DefineValue)
	}
	g.c.linef(1, defaultBreak)
	g.c.linef(1, closeBrace)
	g.c.linef(1, noBranchLine)
	g.c.linef(1, returnFalse)
	g.c.line(closeBrace)
	g.c.blank()
}

// switchArg is the switch's argument: disc, an integer for a Bool (-Wswitch-bool).
func (g *gen) switchArg(d *ir.Dependent) string {
	if d.Disc.Kind == types.Bool {
		return fmt.Sprintf(staticCastFormat, cppInt64, discParam)
	}
	return discParam
}

// discLabels are the case labels of the discriminant's members, in member order (DECISIONS 80).
func (g *gen) discLabels(d *ir.Dependent) []string {
	e, ok := d.Disc.Named.(*ir.Enum)
	if !ok {
		return []string{zeroInt, oneInt}
	}
	name := g.qualifiedOwn(e)
	labels := make([]string, len(e.Members))
	for i, m := range e.Members {
		labels[i] = name + scopeSep + g.pl.Enumerator(m)
	}
	return labels
}

// dependentCase decodes the value as branch i's type into a local, a define key's value into defineValue (DECISIONS 298), then emplaces alternative i; a reader of another package's dependent type builds it through the branch's make hook (CODEGEN.md §2.8, §5.14).
func (g *gen) dependentCase(labels []string, i int, dep *ir.Dependent, defineValue string) {
	if len(labels) == 0 {
		return
	}
	b := dep.Branches[i]
	last := len(labels) - 1
	for _, l := range labels[:last] {
		g.c.linef(1, caseLabelFormat, l)
	}
	g.c.linef(1, caseOpenFormat, labels[last])
	tmp, init := fmt.Sprintf(tempFormat, depthTwo), elemInit(b.Type)
	if _, id := g.idEnum(b.Type); id {
		init = initBraces
	}
	g.c.linef(depthTwo, localFormat, g.global(b.Type), tmp, init)
	g.decodeValue(depthTwo, sourceVar, keyParam, leaf{t: b.Type, dst: tmp})
	reader := dep.Pkg != g.p.Name
	args := []string{fmt.Sprintf(moveFormat, tmp)}
	if d := ir.DefinesOf(g.p, ir.DefineTarget(b.Type)); d != nil {
		dst := outPrefix + defineValue
		if reader {
			dst = defineValue
			g.c.linef(depthTwo, memberFormat, cppInt64, dst, initZero)
			args = append(args, dst)
		}
		g.c.linef(depthTwo, defineCallFormat, g.pl.DefinesName(d), quote(ir.DefineTableName(d)), keyParam, tmp, dst)
	} else if ir.DefinesRef(b.Type) {
		g.malformed(defineMissing, g.at)
	}
	if reader {
		g.c.linef(depthTwo, assignFormat, outVar, g.makeCall(dep.Pkg, g.pl.MakeBranchName(dep, b), args))
	} else {
		g.c.linef(depthTwo, emplaceMoveFormat, i, tmp)
	}
	g.c.linef(depthTwo, returnOk)
	g.c.linef(1, closeBrace)
}

// global is t's storage named from the global namespace, so that Decode<Alias>'s parameters (v, key, disc, dec, out) hide no enum it names.
func (g *gen) global(t ir.TypeRef) string {
	base := t
	for base.Kind == types.Ref && base.Key != nil {
		base = *base.Key
	}
	if base.Kind == types.Enum && base.Named != nil {
		return g.qualifiedOwn(base.Named)
	}
	return g.storage(t)
}

// decodeDependent calls the dependent type's Decode<Alias>, or this package's reader of another package's (CODEGEN.md §2.8, §5.6); a dependent value outside a field or its list's elements is refused at stage E (E8019 DependentType).
func (g *gen) decodeDependent(depth int, src, key string, l leaf) {
	d, ok := l.t.Named.(*ir.Dependent)
	if !ok || l.disc == "" {
		g.malformed(dependentElsewhere, g.at)
		return
	}
	call := g.pl.Dependent(d).Decode
	if d.Pkg != g.p.Name {
		g.qualifier(d.Pkg) // its header is included
		call = g.pl.ReaderName(d)
	}
	g.c.linef(depth, dependentCallFormat, call, src, key, l.disc, l.dst)
}

// discExpr is the discriminant of a field's type application, read from what the decoder has already decoded: ir.DiscFields' path from the class's fields, each through its getter (TYPES.md §11.1; WIRE.md §5.5.1: decoding follows declaration order); stage E refuses any other (E8019 DependentType).
func (g *gen) discExpr(fields []*ir.Field, app ir.TypeRef) string {
	path := ir.DiscFields(fields, app)
	if path == nil {
		g.malformed(dependentBadPath, g.at)
		return ""
	}
	calls := []string{outVar}
	if g.dst == "" { // a reader's local holds the first field (CODEGEN.md §2.8)
		m, err := g.member(path[0].Name)
		g.fail(err)
		calls, path = []string{m}, path[1:]
	}
	for _, f := range path {
		calls = append(calls, g.getterName(f)+callSuffix)
	}
	return strings.Join(calls, memberAccess)
}
