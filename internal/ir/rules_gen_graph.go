package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// classGraph mirrors gen/cpp's class order (CODEGEN.md §2.7): refused are the classes closing a by-value cycle through a variant.
type classGraph struct {
	classes []any
	own     map[any]bool
	boxed   map[*Field]bool
	state   map[any]int
	stack   []any
	refused []any
	// refusedRecords close a cycle of records only, through a stored method's result.
	refusedRecords []any
}

// classDep is a class another one holds: strong when by value, weak through a list or a map.
type classDep struct {
	to     any
	strong bool
}

func newClassGraph(p *Package) *classGraph {
	g := &classGraph{classes: packageClasses(p), own: map[any]bool{}, boxed: map[*Field]bool{}, state: map[any]int{}}
	for _, c := range g.classes {
		g.own[c] = true
	}
	boxed := map[*Field]bool{}
	for _, c := range g.classes {
		fields, _ := classBody(c)
		for _, f := range fields {
			held := f.Type.Kind == types.Record || f.Type.Kind == types.Variant
			if f.Optional && held && g.reaches(f.Type.Named, c, map[any]bool{}) {
				boxed[f] = true
			}
		}
	}
	g.boxed = boxed
	return g
}

// deps are the classes c holds: a variant its cases with fields, a record or case its fields' and stored fns' classes.
func (g *classGraph) deps(c any) []classDep {
	var out []classDep
	if v, ok := c.(*Variant); ok {
		for _, cs := range casesWithFields(v) {
			out = append(out, classDep{to: cs, strong: true})
		}
		return out
	}
	fields, fns := classBody(c)
	for _, f := range fields {
		out = classDeps(f.Type, !g.boxed[f], out)
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated {
			out = classDeps(fn.Result, true, out)
		}
	}
	return out
}

// classDeps adds the class t holds: through a list or a map weakly, through an optional as t is.
func classDeps(t TypeRef, strong bool, out []classDep) []classDep {
	switch {
	case t.Kind == types.Record, t.Kind == types.Variant:
		return append(out, classDep{to: t.Named, strong: strong})
	case t.Elem == nil:
		return out
	case t.Kind == types.List, t.Kind == types.Map:
		return classDeps(*t.Elem, false, out)
	case t.Kind == types.Optional:
		return classDeps(*t.Elem, strong, out)
	}
	return out
}

// reaches reports that from holds to by value: through fields, optional or not, and cases.
func (g *classGraph) reaches(from, to any, seen map[any]bool) bool {
	if from == to {
		return true
	}
	if !g.own[from] || seen[from] {
		return false
	}
	seen[from] = true
	for _, d := range g.deps(from) {
		if d.strong && g.reaches(d.to, to, seen) {
			return true
		}
	}
	return false
}

// visit orders c after the classes it holds, noting c where it holds by value one still being visited.
func (g *classGraph) visit(c any) {
	g.state[c] = visiting
	g.stack = append(g.stack, c)
	for _, d := range g.deps(c) {
		switch {
		case !g.own[d.to], g.state[d.to] == visited:
		case g.state[d.to] == visiting && d.strong:
			g.closes(c, d.to)
		case g.state[d.to] == unvisited:
			g.visit(d.to)
		}
	}
	g.state[c] = visited
	g.stack = g.stack[:len(g.stack)-1]
}

// closes notes c, whose by-value dep to closes a cycle, by whether the cycle goes through a variant.
func (g *classGraph) closes(c, to any) {
	cycle := g.stack[slices.Index(g.stack, to):]
	list := &g.refusedRecords
	if slices.ContainsFunc(cycle, func(x any) bool { _, ok := x.(*Variant); return ok }) {
		list = &g.refused
	}
	if !slices.Contains(*list, c) {
		*list = append(*list, c)
	}
}
