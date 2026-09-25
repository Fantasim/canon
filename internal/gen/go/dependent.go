package gogen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// dependentType writes a dependent type's struct and accessors, used or not (CODEGEN.md §5.6); its branch enum is in the enums section (§2.7, branchEnums).
func (g *gen) dependentType(d *ir.Dependent) {
	defer g.enter(d.QName())()
	n := g.names.Dependent(d)
	g.body.WriteString(docFor(n.Type, d.Doc))
	g.printf(variantFormat, n.Type, n.Branch, n.BranchStore, n.ValueStore, n.Method)
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

// dependentAccessor writes As<Branch>, zero unless the value holds it (CODEGEN.md §5.6, §5.8).
func (g *gen) dependentAccessor(n ir.GoDependent, b ir.GoBranch, branch *ir.Branch) {
	if b.AsValue != "" {
		g.fail(newDetail(ErrUnsupported, g.at, dependentValueFormat, g.at))
	}
	t := g.goType(branch.Type)
	g.printf(dependentAsFormat, n.Type, b.As, t, n.BranchStore, b.Member, n.ValueStore)
}

// decodeDependent writes decode<T> of a dependent type a decoded class holds: the untagged wire switches on the discriminant its caller already decoded (CODEGEN.md §5.6, §6.1).
func (g *gen) decodeDependent(d *ir.Dependent) {
	defer g.enter(d.QName())()
	g.temps = 0
	e, ok := dependentDiscEnum(d)
	if !ok {
		g.fail(newDetail(ErrUnsupported, d.QName(), dependentDiscKindFormat, d.QName()))
		return
	}
	lc, n := g.lc, g.names.Dependent(d)
	disc, discType, name := g.names.DataLocal(localDisc), g.qualify(e.Pkg, g.goName(e)), g.goName(d)
	g.printf(dependentFuncOpenFormat, g.decodeFunc(d), lc.Name, lc.Path, lc.Raw, g.rawType(), disc, discType, lc.Out, name)
	g.printf(switchTagFormat, disc)
	for i, br := range d.Branches {
		g.printf(caseFormat, dependentMemberConsts(g, e, br.Members))
		var b strings.Builder
		x := g.readValue(&b, leaf{t: br.Type}, lc.Raw, g.root())
		g.body.WriteString(b.String())
		g.printf(assignPairFormat, lc.Out+dot+n.BranchStore, lc.Out+dot+n.ValueStore, n.Branches[i].Member, x)
		g.body.WriteString(returnKw + nilLit + newline)
	}
	g.printf(unknownCaseFormat, g.errAt(g.root(), unknownCaseText, disc+dot+ir.GoString+callSuffix))
	g.body.WriteString(closeBrace + newline)
}

// dependentDiscEnum is the enum a dependent type's match reads; only an enum discriminant is generated yet (CODEGEN.md §5.6).
func dependentDiscEnum(d *ir.Dependent) (*ir.Enum, bool) {
	if d.Disc == nil || d.Disc.Kind != types.Enum {
		return nil, false
	}
	e, ok := d.Disc.Named.(*ir.Enum)
	return e, ok
}

// dependentMemberConsts are the enum member constants a branch's pattern covers, in member order.
func dependentMemberConsts(g *gen, e *ir.Enum, members []int) string {
	names := make([]string, len(members))
	for i, mi := range members {
		names[i] = g.qualify(e.Pkg, g.names.MemberName(e, e.Members[mi]))
	}
	return strings.Join(names, listSep)
}

// dependentDisc is a dependent field's discriminant; reason names the ErrUnsupported cause when it cannot resolve one (CODEGEN.md §5.6).
func (g *gen) dependentDisc(owner *body, t ir.TypeRef) (expr, reason string) {
	d, ok := t.Named.(*ir.Dependent)
	if !ok {
		return "", ""
	}
	if _, ok := dependentDiscEnum(d); !ok {
		return "", dependentDiscKindFormat
	}
	if len(d.DiscPath) > 0 {
		return "", dependentScrutineeFormat
	}
	if owner == nil || d.DiscParam < 0 || d.DiscParam >= len(t.Args) {
		return "", dependentArgFormat
	}
	src := t.Args[d.DiscParam]
	if src.From != types.ArgField || len(src.WirePath) != 1 {
		return "", dependentArgFormat
	}
	es := siblingSlot(owner, src.WirePath)
	if es == nil {
		return "", dependentArgFormat
	}
	if es.Optional {
		return "", dependentDiscOptionalFormat
	}
	return g.lc.Out + dot + es.Store, ""
}

// siblingSlot is the slot of owner's own field whose whole wire path is wire, nil for none.
func siblingSlot(owner *body, wire []string) *slot {
	for _, s := range owner.slots {
		if s.src != nil && slices.Equal(s.src.WirePath, wire) {
			return s
		}
	}
	return nil
}

// readDependentValue decodes an untagged dependent-type value, the branch slotLeaf's disc selects (CODEGEN.md §5.6).
func (g *gen) readDependentValue(b *strings.Builder, l leaf, raw string, loc location) string {
	d, ok := l.t.Named.(*ir.Dependent)
	if !ok {
		g.failf(ErrMalformed, "a dependent type at %s", g.at)
		return raw
	}
	if l.disc == "" {
		g.fail(newDetail(ErrUnsupported, g.at, dependentNestedFormat, g.at))
		return raw
	}
	v := g.temp(tempValue)
	fmt.Fprintf(b, dependentDecodeFormat, v, g.goName(d), g.lc.Err, g.decodeFunc(d), g.lc.Name, g.locExpr(loc), raw, l.disc)
	return v
}
