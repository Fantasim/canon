package cppgen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// class is a generated C++ class of a record, a variant, a variant case with fields, or a dependent type.
type class struct {
	rec       *ir.Record
	variant   *ir.Variant
	cs        *ir.Case      // with variant: a case class
	dependent *ir.Dependent // a dependent type's class (CODEGEN.md §5.6)
}

// key identifies a class across the sort: the record, the case, the dependent type, or the variant.
func (c class) key() any {
	switch {
	case c.rec != nil:
		return c.rec
	case c.cs != nil:
		return c.cs
	case c.dependent != nil:
		return c.dependent
	}
	return c.variant
}

// shape is what a record and a case class have in common: fields and export fns.
func (c class) shape() (fields []*ir.Field, fns []*ir.ExportFn) {
	switch {
	case c.rec != nil:
		return c.rec.Fields, c.rec.Methods
	case c.cs != nil:
		return c.cs.Fields, c.cs.Methods
	}
	return nil, nil
}

// name is the class's C++ name.
func (g *gen) className(c class) string {
	switch {
	case c.rec != nil:
		return g.typeName(c.rec)
	case c.cs != nil:
		return g.caseName(c.variant, c.cs)
	case c.dependent != nil:
		return g.typeName(c.dependent)
	}
	return g.typeName(c.variant)
}

// canonName is the class as messages and conformance failures name it: Potion, Reward.item.
func (c class) canonName() string {
	switch {
	case c.rec != nil:
		return c.rec.Name
	case c.cs != nil:
		return c.variant.Name + qnameSep + c.cs.Name
	case c.dependent != nil:
		return c.dependent.Name
	}
	return c.variant.Name
}

// declared is every class in declaration order: a variant, then its case classes; a dependent type is a class too (CODEGEN.md §5.6).
func (g *gen) declared() []class {
	var out []class
	for _, t := range g.p.Types {
		switch x := t.(type) {
		case *ir.Record:
			out = append(out, class{rec: x})
		case *ir.Variant:
			out = append(out, variantClasses(x)...)
		case *ir.Dependent:
			out = append(out, class{dependent: x})
		}
	}
	return out
}

// variantClasses are a variant's class, then its cases with fields (§5.5; log-2026-09-24, round 3).
func variantClasses(v *ir.Variant) []class {
	out := []class{{variant: v}}
	for _, c := range v.Cases {
		if len(c.Fields) > 0 {
			out = append(out, class{variant: v, cs: c})
		}
	}
	return out
}

// sortClasses puts a class before the first class holding it by value, depth first (CODEGEN.md §2.7).
func (g *gen) sortClasses() {
	all := g.declared()
	byKey := map[any]class{}
	for _, c := range all {
		byKey[c.key()] = c
	}
	state := map[any]int{}
	var visit func(c class)
	visit = func(c class) {
		state[c.key()] = visiting
		for _, d := range g.deps(c) {
			target, ok := byKey[d.to]
			switch {
			case !ok, state[d.to] == visited:
			case state[d.to] == visiting && d.strong:
				g.malformed(recursiveTypes, c.canonName()) // E8019 RecursiveVariantCase, RecordCycleThroughMethod
			case state[d.to] != visiting:
				visit(target)
			}
		}
		state[c.key()] = visited
		g.classes = append(g.classes, c)
	}
	for _, c := range all {
		if state[c.key()] == 0 {
			visit(c)
		}
	}
}

// dep is a class another one holds: strong when held directly, weak through a list.
type dep struct {
	to     any
	strong bool
}

func (g *gen) deps(c class) []dep {
	if c.variant != nil && c.cs == nil {
		var out []dep
		for _, cs := range c.variant.Cases {
			if len(cs.Fields) > 0 {
				out = append(out, dep{to: cs, strong: true})
			}
		}
		return out
	}
	fields, fns := c.shape()
	var out []dep
	for _, f := range fields {
		out = typeDeps(f.Type, !g.boxed[f], out)
	}
	for _, fn := range fns {
		if fn.Kind != ir.FnTranslated {
			out = typeDeps(fn.Result, true, out)
		}
	}
	return out
}

// typeDeps adds the classes t holds, a dependent type's included; through a list or a map they are weak.
func typeDeps(t ir.TypeRef, strong bool, out []dep) []dep {
	switch {
	case t.Kind == types.Record, t.Kind == types.Variant, t.Kind == types.TypeApp:
		return append(out, dep{to: t.Named, strong: strong})
	case t.Elem == nil:
		return out
	case t.Kind == types.List, t.Kind == types.Map, t.Kind == types.Table:
		return typeDeps(*t.Elem, false, out)
	case t.Kind == types.Optional:
		return typeDeps(*t.Elem, strong, out)
	default:
		return out
	}
}

// indexFns lists the translated fns; stored methods are getters (CODEGEN.md §5.10, E8013).
func (g *gen) indexFns() {
	for _, fn := range g.p.Fns {
		if fn.Kind != ir.FnTranslated {
			g.malformed(packageStoredFn, fn.Name) // E8013 package
			continue
		}
		g.pkgFns = append(g.pkgFns, fn)
	}
	g.fieldlessMethods()
	for _, c := range g.declared() {
		fields, fns := c.shape()
		for _, fn := range fns {
			if fn.Kind == ir.FnTranslated {
				g.methods = append(g.methods, &method{fn: fn, class: c, fields: fields})
			}
		}
	}
}

// fieldlessMethods is a defensive guard: E8019 already refuses export fns of a fieldless case.
func (g *gen) fieldlessMethods() {
	for _, t := range g.p.Types {
		v, ok := t.(*ir.Variant)
		for i := 0; ok && i < len(v.Cases); i++ {
			if c := v.Cases[i]; len(c.Fields) == 0 && len(c.Methods) > 0 {
				g.fail(fmt.Errorf("%w: export fns of %s without fields", ErrMalformed, v.Name+qnameSep+c.Name))
			}
		}
	}
}

// declareNames checks the names of the emit's namespace (CODEGEN.md §3.5).
func (g *gen) declareNames() {
	for _, t := range g.p.Types {
		if e, ok := t.(*ir.Enum); ok {
			g.declareEnum(g.typeName(e), e.Name)
		}
	}
	for _, c := range g.declared() {
		g.declare(g.className(c), c.canonName())
		if c.variant != nil && c.cs == nil {
			g.declareEnum(g.kindName(c.variant), c.variant.Name)
		}
	}
	for _, c := range g.p.Consts {
		g.declare(g.pl.ConstName(c), c.Name)
	}
	for _, v := range g.values {
		g.declare(g.pl.SchemaName(v), v.Name)
		if v.Type.Kind != types.Record {
			g.declare(g.pl.ContainerName(v), v.Name)
		}
	}
	for _, fn := range g.pkgFns {
		g.declare(g.pl.FnName(fn), fn.Name)
	}
	if g.reloads() > 0 {
		g.declare(g.pl.SnapshotName(), snapshotOrigin)
		g.declare(g.pl.StoreName(), snapshotOrigin)
	}
}

// declareEnum declares an enum and its helpers that are not overloads.
func (g *gen) declareEnum(name, origin string) {
	helpers := g.pl.EnumHelpers(name)
	g.declare(name, origin)
	g.declare(helpers.Members, origin)
	g.declare(helpers.FromWire, origin)
}

func (g *gen) reloads() int {
	n := 0
	for _, v := range g.values {
		if v.Reload {
			n++
		}
	}
	return n
}

// forwards declares every class of the header, in declaration order (CODEGEN.md §2.7).
func (g *gen) forwards() {
	var names []string
	for _, c := range g.declared() {
		names = append(names, g.className(c))
	}
	for _, v := range g.values {
		if v.Type.Kind != types.Record {
			names = append(names, g.pl.ContainerName(v))
		}
	}
	if g.reloads() > 0 {
		names = append(names, g.pl.SnapshotName(), g.pl.StoreName())
	}
	for _, n := range names {
		g.h.printf(forwardFormat, n)
	}
	if len(names) > 0 {
		g.h.blank()
	}
}
