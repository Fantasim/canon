package cppgen

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// fillDependent writes a dependent value, or each element of a list of them, into the branch the record's own discriminant selects (CODEGEN.md §5.6).
func (g *gen) fillDependent(a at, key string, fields []*ir.Field, f *ir.Field, r *value.Record) {
	v := g.fieldOf(r, f.Name)
	d, br := g.bakedBranch(fields, *ir.HeldApp(f.Type), r)
	if d == nil {
		return
	}
	bt := d.Branches[br].Type
	target := key
	if f.Optional {
		a.line(emplaceStmtFormat, key)
		target = fmt.Sprintf(derefFormat, key)
	}
	one := func(lhs string, e value.Value) {
		if d.Pkg != g.p.Name { // another package's: its branch hook (CODEGEN.md §5.14)
			args := []string{g.element(bt, e)}
			if ir.DefinesRef(bt) {
				args = append(args, g.defineLit(bt, e))
			}
			a.line(assignFormat, lhs, g.makeCall(d.Pkg, g.pl.MakeBranchName(d, d.Branches[br]), args))
			return
		}
		a.line(emplaceBranchFormat, lhs, br, g.element(bt, e))
		if ir.DefinesRef(bt) {
			a.line(assignFormat, lhs+memberAccess+g.pl.Dependent(d).DefineValue, g.defineLit(bt, e))
		}
	}
	l, isList := v.(*value.List)
	if !isList {
		one(target, v)
		return
	}
	a.line(resizeFormat, target, len(l.Elems))
	for i, e := range l.Elems {
		one(fmt.Sprintf(indexFormat, target, strconv.Itoa(i)), e)
	}
}

// defineLit is the value of the define a key e of ref type t names, from the package's define table (CODEGEN.md §5.8).
func (g *gen) defineLit(t ir.TypeRef, e value.Value) string {
	d := ir.DefinesOf(g.p, ir.DefineTarget(t))
	r, isRef := e.(*value.Ref)
	i := -1
	if d != nil && isRef {
		i = slices.Index(d.Names, r.Key.S)
	}
	if i < 0 || i >= len(d.Values) {
		g.malformed(defineMissing, g.at)
		return cppInvalid
	}
	return intLit(d.Values[i])
}

// bakedBranch is the dependent type app applies and the branch r's discriminant selects, read down ir.DiscFields' path; nil where stage E refuses (E8019 DependentType, E3801).
func (g *gen) bakedBranch(fields []*ir.Field, app ir.TypeRef, r *value.Record) (*ir.Dependent, int) {
	d, ok := app.Named.(*ir.Dependent)
	path := ir.DiscFields(fields, app)
	if !ok || path == nil {
		g.malformed(dependentBadPath, g.at)
		return nil, 0
	}
	disc := value.Value(r)
	for _, f := range path {
		rec, isRecord := disc.(*value.Record)
		if !isRecord {
			g.malformed(dependentBadPath, g.at)
			return nil, 0
		}
		disc = g.fieldOf(rec, f.Name)
	}
	m := -1
	switch x := disc.(type) {
	case *value.Member:
		m = x.Index
	case *value.Bool:
		m = boolOrdinal(x.V)
	}
	if m < 0 || m >= len(d.ByMember) || d.ByMember[m] < 0 || d.ByMember[m] >= len(d.Branches) {
		g.malformed(dependentNoBranch, g.at)
		return nil, 0
	}
	return d, d.ByMember[m]
}

// boolOrdinal is a Bool's position in its domain: false, then true (CODEGEN.md §5.6, §5.10).
func boolOrdinal(b bool) int {
	if b {
		return 1
	}
	return 0
}

// fillDefine writes a define ref's values beside its keys, from the package's define table (CODEGEN.md §5.8).
func (g *gen) fillDefine(a at, lhs string, f *ir.Field, v value.Value) {
	if _, member, ok := g.pl.DefineValue(f); ok {
		g.fillDefineTo(a, lhs+memberAccess+member, f, v)
	}
}

// fillDefineTo writes the values of define ref f's keys v into dst.
func (g *gen) fillDefineTo(a at, dst string, f *ir.Field, v value.Value) {
	lookup := func(e value.Value) string { return g.defineLit(f.Type, e) }
	l, isList := v.(*value.List)
	if !isList {
		a.line(assignFormat, dst, lookup(v))
		return
	}
	items := make([]string, len(l.Elems))
	for i, e := range l.Elems {
		items[i] = lookup(e)
	}
	a.line(assignFormat, dst, g.storage(defineValueType(f.Type))+openBrace+strings.Join(items, listSep)+closeBrace)
}
