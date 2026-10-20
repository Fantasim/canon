package cppgen

import (
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// builtDefault writes, before field member's key is read, the function-local static const its default holding a record or variant is built into once, by the baked filler's statements, and returns its name; "" for a default a literal writes (CODEGEN.md §5.13; DECISIONS 386).
func (g *gen) builtDefault(l leaf, member string) string {
	if l.def == nil || !ir.HoldsRecordValue(l.def) {
		return ""
	}
	storage, kept := g.storage(l.t), fmt.Sprintf(defaultKeptFormat, member)
	g.c.linef(1, defaultStaticFormat, storage, kept)
	g.c.linef(depthTwo, defaultLocalFormat, storage, defaultBuilt)
	g.fillValue(at{&g.c, depthTwo}, defaultBuilt, l.t, false, l.def)
	g.c.linef(depthTwo, returnFormat, defaultBuilt)
	g.c.linef(1, defaultCloseLine)
	return kept
}

// assignDefault writes the constant default l takes where its key, or an object of its path, is absent into l.dst: its literal, or a copy of its built static const (builtDefault).
func (g *gen) assignDefault(depth int, l leaf) {
	if l.kept == "" {
		g.c.linef(depth, assignFormat, l.dst, g.defaultLit(l.t, l.def))
		return
	}
	kept := l.kept
	if l.box != "" {
		kept = fmt.Sprintf(makeUniqueCopyFormat, l.box, kept)
	}
	g.c.linef(depth, assignFormat, l.dst, kept)
}

// defaultWriters befriends, in types mode, each class of this package whose members a built default writes (a record, a variant, a case, a dependent type, a row class of another package's record) with the decoder writing it, as the baked filler's Access is (CODEGEN.md §5.13, §7.2; DECISIONS 386).
func (g *gen) defaultWriters() {
	if !g.types() {
		return
	}
	for _, c := range g.declared() {
		if c.dependent != nil || c.variant != nil && c.cs == nil {
			continue
		}
		fields, _ := c.shape()
		for _, f := range fields {
			if def := g.fieldDefault(f); def != nil && f.Input == nil && ir.HoldsRecordValue(def) {
				g.befriend(g.className(c), f.Type, def)
			}
		}
	}
}

// befriend makes friend a friend of each class of this package whose members filling v, of type t, writes: another package's classes are built through their hooks.
func (g *gen) befriend(friend string, t ir.TypeRef, v value.Value) {
	for t.Kind == types.Optional && t.Elem != nil {
		t = *t.Elem
	}
	switch x := v.(type) {
	case *value.List:
		for _, e := range x.Elems {
			g.befriendElem(friend, t, e)
		}
	case *value.Map:
		for _, e := range x.Vals {
			g.befriendElem(friend, t, e)
		}
	case *value.Table:
		g.befriendRows(friend, g.nestedRecord(t), x)
	case *value.Record:
		g.befriendRecord(friend, t, x)
	}
}

// befriendElem is befriend on an element or map value of a list or map of type t.
func (g *gen) befriendElem(friend string, t ir.TypeRef, e value.Value) {
	if t.Elem != nil {
		g.befriend(friend, *t.Elem, e)
	}
}

// befriendRows befriends the rows of a table of rec: rec's class when it is this package's, else this package's row class of it (CODEGEN.md §5.9).
func (g *gen) befriendRows(friend string, rec *ir.Record, tab *value.Table) {
	if rec.Pkg != g.p.Name {
		if !slices.Contains(g.rowFriends[rec], friend) {
			g.rowFriends[rec] = append(g.rowFriends[rec], friend)
		}
		return
	}
	for _, r := range tab.Entries {
		g.befriendRecord(friend, ir.TypeRef{Kind: types.Record, Named: rec}, r)
	}
}

// befriendRecord befriends a record of this package, or a variant of it and the case r holds, then what their fields' values write.
func (g *gen) befriendRecord(friend string, t ir.TypeRef, r *value.Record) {
	var fields []*ir.Field
	switch x := t.Named.(type) {
	case *ir.Record:
		if x.Pkg != g.p.Name {
			return
		}
		g.addFriend(x, friend)
		fields = x.Fields
	case *ir.Variant:
		ct := caseOf(r)
		if x.Pkg != g.p.Name || ct == nil || ct.Index < 0 || ct.Index >= len(x.Cases) {
			return
		}
		g.addFriend(x, friend)
		g.addFriend(x.Cases[ct.Index], friend)
		fields = x.Cases[ct.Index].Fields
	}
	for _, f := range fields {
		fv, ok := ir.FieldValue(r, f.Name)
		if !ok || absent(fv) {
			continue
		}
		if d := heldDependent(f.Type); d != nil && d.Pkg == g.p.Name {
			g.addFriend(d, friend)
		}
		g.befriend(friend, f.Type, fv)
	}
}

// addFriend adds friend to the decoders befriended by class key, once.
func (g *gen) addFriend(key any, friend string) {
	if !slices.Contains(g.defaultFriends[key], friend) {
		g.defaultFriends[key] = append(g.defaultFriends[key], friend)
	}
}

// heldDependent is the dependent type a field holds, itself or through lists (ir.HeldApp), nil for none.
func heldDependent(t ir.TypeRef) *ir.Dependent {
	if app := ir.HeldApp(t); app != nil {
		d, _ := app.Named.(*ir.Dependent)
		return d
	}
	return nil
}

// caseOf is the case type of a variant value, nil for none.
func caseOf(r *value.Record) *types.CaseType {
	if r.T == nil {
		return nil
	}
	ct, _ := r.T.Base().(*types.CaseType)
	return ct
}
