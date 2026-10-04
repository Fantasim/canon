package cppgen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// slot is a ref a class stores, a field or a stored fn's result: its key member, its type
// after the optional, whether it is a list of refs, and its table's cell count (0 for none).
type slot struct {
	member   string
	ref      string // the member holding the resolved entry, f_ref_ (CODEGEN.md §5.8)
	wire     string // the key a load error names
	t        ir.TypeRef
	optional bool
	list     bool
	cells    int
	paths    []string // with cells, each cell's key path in load errors (cellPaths)
}

// target is the ref's element type: t itself, or a list's element.
func (s slot) target() ir.TypeRef {
	if s.list {
		return *s.t.Elem
	}
	return s.t
}

// refSlot reports `ref T` or `[ref T]`; refs nested deeper are keys only (CODEGEN.md §5.8).
func refSlot(t ir.TypeRef) (list, ok bool) {
	switch {
	case t.Kind == types.Ref:
		return false, true
	case t.Kind == types.List && t.Elem != nil && t.Elem.Kind == types.Ref:
		return true, true
	default:
		return false, false
	}
}

// holdersOf lists, per class, the emitted values holding it by value (records, cases, lists).
func (g *gen) holdersOf() {
	byKey := map[any]class{}
	for _, c := range g.declared() {
		byKey[c.key()] = c
	}
	for _, v := range g.values {
		root := v.Type
		if root.Kind != types.Record && root.Elem != nil {
			root = *root.Elem
		}
		seen := map[any]bool{}
		g.hold(byKey, root.Named, v, seen)
	}
}

func (g *gen) hold(byKey map[any]class, key any, v *ir.Value, seen map[any]bool) {
	c, ok := byKey[key]
	if !ok || seen[key] {
		return
	}
	seen[key] = true
	g.holders[key] = append(g.holders[key], v)
	for _, d := range g.deps(c) {
		g.hold(byKey, d.to, v, seen)
	}
}

// resolvedTarget is the value a ref of c resolves into, or nil (CODEGEN.md §5.8, §5.11; log-2026-09-24 "ir name plans + support plan").
func (g *gen) resolvedTarget(t ir.TypeRef, c class) *ir.Value {
	if !g.pl.Resolves(t, c.key()) {
		return nil
	}
	return g.valueNamed(t.Ref.Value)
}

func (g *gen) valueNamed(name string) *ir.Value {
	for _, v := range g.values {
		if v.Name == name {
			return v
		}
	}
	return nil
}

// resolvedGetter writes a slot's entry getter, unset in a default-constructed record (log-2026-09-24).
func (g *gen) resolvedGetter(sc *scope, name string, acc access, s slot, target *ir.Value) string {
	storage := fmt.Sprintf(constPtrFormat, g.valueElem(target))
	m, init := s.ref, initNull
	typ, body := fmt.Sprintf(constRefFormat, g.valueElem(target)), fmt.Sprintf(derefReturnFormat, acc.cell(m))
	switch {
	case s.list && s.optional:
		vec := fmt.Sprintf(vectorFormat, storage)
		storage, init = fmt.Sprintf(optionalFormat, vec), ""
		typ, body = fmt.Sprintf(constPtrFormat, vec), fmt.Sprintf(returnPtrFormat, acc.cell(m))
	case s.list:
		storage, init = fmt.Sprintf(vectorFormat, storage), ""
		typ, body = fmt.Sprintf(constRefFormat, storage), fmt.Sprintf(returnFormat, acc.cell(m))
	case s.optional:
		typ, body = storage, fmt.Sprintf(returnFormat, acc.cell(m))
	}
	g.fail(sc.add(name, s.member))
	g.fail(sc.add(m, s.member))
	g.writeGetter(typ, name, acc, body)
	if s.cells > 0 {
		storage, init = fmt.Sprintf(arrayFormat, storage, s.cells), initBraces
	}
	return fmt.Sprintf(memberFormat, storage, m, init)
}

// resolved is a slot of a class that resolves into target at load.
type resolved struct {
	slot
	target *ir.Value
}

func (g *gen) noteSlot(c class, s slot, target *ir.Value) {
	g.slots[c.key()] = append(g.slots[c.key()], resolved{s, target})
}

// valueElem is the class of a table's or keyed list's entries.
func (g *gen) valueElem(v *ir.Value) string { return g.storage(*v.Type.Elem) }
