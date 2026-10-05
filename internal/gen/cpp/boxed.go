package cppgen

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// boxes marks the optional fields whose type holds their class: std::unique_ptr (CODEGEN.md §7.2); those of the other packages' classes the emit builds too, whose hooks take them as their owner holds them (§5.14).
func (g *gen) boxes() {
	boxed := map[*ir.Field]bool{}
	for _, c := range append(g.declared(), g.foreignClasses()...) {
		fields, _ := c.shape()
		for _, f := range fields {
			if f.Optional && holdsClass(f.Type) && g.reaches(f.Type.Named, c.key(), map[any]bool{}) {
				boxed[f] = true
			}
		}
	}
	g.boxed = boxed
}

func holdsClass(t ir.TypeRef) bool { return t.Kind == types.Record || t.Kind == types.Variant }

// reaches reports that from holds to by value: through fields, optional or not, and cases.
func (g *gen) reaches(from, to any, seen map[any]bool) bool {
	if from == to {
		return true
	}
	if seen[from] {
		return false
	}
	seen[from] = true
	for _, next := range g.strongDeps(from) {
		if g.reaches(next, to, seen) {
			return true
		}
	}
	return false
}

// strongDeps are the classes a record, case or variant of any package holds directly.
func (g *gen) strongDeps(key any) []any {
	var c class
	switch x := key.(type) {
	case *ir.Record:
		c = class{rec: x}
	case *ir.Case:
		c = class{cs: x}
	case *ir.Variant:
		c = class{variant: x}
	default:
		return nil
	}
	var out []any
	for _, d := range g.deps(c) {
		if d.strong {
			out = append(out, d.to)
		}
	}
	return out
}

// foreignClasses are the other packages' records, variants, cases and dependent types the emit builds through their make hooks (CODEGEN.md §2.8).
func (g *gen) foreignClasses() []class {
	u := g.pl.Foreign()
	var out []class
	for _, c := range u.Built() {
		switch x := c.(type) {
		case *ir.Record:
			out = append(out, class{rec: x})
		case *ir.Variant:
			out = append(out, class{variant: x})
		case *ir.Case:
			out = append(out, class{variant: u.VariantOf(x), cs: x})
		case *ir.Dependent:
			out = append(out, class{dependent: x})
		}
	}
	return out
}
