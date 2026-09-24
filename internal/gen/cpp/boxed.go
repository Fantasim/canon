package cppgen

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// boxes marks the optional fields whose type holds their class: std::unique_ptr (CODEGEN.md §7.2).
func (g *gen) boxes() {
	byKey := map[any]class{}
	for _, c := range g.declared() {
		byKey[c.key()] = c
	}
	boxed := map[*ir.Field]bool{}
	for _, c := range g.declared() {
		fields, _ := c.shape()
		for _, f := range fields {
			if f.Optional && holdsClass(f.Type) && g.reaches(byKey, f.Type.Named, c.key(), map[any]bool{}) {
				boxed[f] = true
			}
		}
	}
	g.boxed = boxed
}

func holdsClass(t ir.TypeRef) bool { return t.Kind == types.Record || t.Kind == types.Variant }

// reaches reports that from holds to by value: through fields, optional or not, and cases.
func (g *gen) reaches(byKey map[any]class, from, to any, seen map[any]bool) bool {
	if from == to {
		return true
	}
	c, ok := byKey[from]
	if !ok || seen[from] {
		return false
	}
	seen[from] = true
	for _, d := range g.deps(c) {
		if d.strong && g.reaches(byKey, d.to, to, seen) {
			return true
		}
	}
	return false
}
