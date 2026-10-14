package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// checkPositions is W8031 on every package with an emit other than view (CODEGEN.md §2.10, EVALUATION.md §13.1, DECISIONS 379): it runs after precompute and walks its roots in precompute's order (precompute.go:17-21): the package fns, a fn's result and its lookup as two roots, then the lets, then the ts data defaults; within a root, the stored method results drain breadth-first, first queued first.
func (s *stage) checkPositions() {
	for _, u := range s.order {
		if !u.selected || !hasDataEmit(u) {
			continue
		}
		w := &positionWalk{s: s, u: u, seen: map[value.Value]bool{}, sites: map[source.Span]bool{}, results: map[*ExportFn]map[*value.Record]*Instance{}}
		for _, site := range u.fns {
			w.fn(site)
		}
		for _, v := range u.values {
			w.root(v.obj.Pkg()+qnameSep+v.obj.Name(), v.span().span(), false, v.v.V)
		}
		w.defaults()
	}
}

// positionWalk is stage E's walk of one package's outputs for marked values: composites already met, the sites already reported, each method's Instances by receiver, and the stored results still to walk.
type positionWalk struct {
	s       *stage
	u       *unit
	name    string      // the output being walked, as `{output}` names it
	decl    source.Span // its declaration, the finding's span when the provenance names no site, and its related note
	text    bool        // the root is a `@text` result: a file, whose stored method results are never written (DECISIONS 308)
	seen    map[value.Value]bool
	sites   map[source.Span]bool
	results map[*ExportFn]map[*value.Record]*Instance
	pending []pendingOutput
}

// pendingOutput is the stored result of a method, and the output it is reported against.
type pendingOutput struct {
	name   string
	decl   source.Span
	values []value.Value
}

// root walks the values of the output name, declared at decl, then the stored method results they met, first queued first (precompute.go).
func (w *positionWalk) root(name string, decl source.Span, text bool, vs ...value.Value) {
	w.name, w.decl, w.text = name, decl, text
	for _, v := range vs {
		w.value(v)
	}
	w.text = false
	for len(w.pending) > 0 {
		next := w.pending[0]
		w.pending = w.pending[1:]
		w.name, w.decl = next.name, next.decl
		for _, v := range next.values {
			w.value(v)
		}
	}
}

// fn walks a package fn's stored result, then the cells of its lookup as a second root; a `@text` result is a file, whose lookup and method results are never written (DECISIONS 308).
func (w *positionWalk) fn(site *fnSite) {
	name, decl := site.pkg+qnameSep+site.label, site.span()
	w.root(name, decl, site.text, site.fn.Value)
	if !site.text && site.fn.Table != nil {
		w.root(name, decl, false, site.fn.Table.Cells...)
	}
}

// defaults walks the constant field defaults a ts data emit writes with their records' stored results (precompute.go, DECISIONS 278), each against that emit.
func (w *positionWalk) defaults() {
	if !tsData(w.u) {
		return
	}
	for _, es := range w.u.emits {
		if es.e.Target == TargetTS && es.e.Mode == ModeData {
			// the root's name is never shown: a default leaf cannot be marked (TYPES.md §15), so every finding here names a method result
			w.root(w.u.cp.Path, es.span(), false, ownDefaults(w.u.p)...)
			return
		}
	}
}

// value walks v: a marked leaf is reported, a composite is walked once, in the order walkInstances visits (EVALUATION.md §8.1), and a record's stored method results follow its fields.
func (w *positionWalk) value(v value.Value) {
	if v == nil {
		return
	}
	if value.IsPosition(v) {
		w.report(v)
		return
	}
	switch x := v.(type) {
	case *value.Record:
		w.record(x)
	case *value.List:
		w.list(x.Elems, x)
	case *value.Map:
		w.mapping(x)
	case *value.Table:
		if w.once(x) {
			for _, e := range x.Entries {
				w.value(e)
			}
		}
	case *value.Pair:
		w.list([]value.Value{x.A, x.B}, x)
	}
}

// once reports whether composite c is met for the first time.
func (w *positionWalk) once(c value.Value) bool {
	if w.seen[c] {
		return false
	}
	w.seen[c] = true
	return true
}

// list walks the elements of the composite c once.
func (w *positionWalk) list(elems []value.Value, c value.Value) {
	if !w.once(c) {
		return
	}
	for _, e := range elems {
		w.value(e)
	}
}

// mapping walks a map's keys then values, in entry order.
func (w *positionWalk) mapping(m *value.Map) {
	if !w.once(m) {
		return
	}
	for i := range m.Keys {
		w.value(m.Keys[i])
		w.value(m.Vals[i])
	}
}

// record walks the fields of r, then queues the stored results of its methods, which a `@text` result has none of.
func (w *positionWalk) record(r *value.Record) {
	if !w.once(r) {
		return
	}
	for _, f := range r.Fields {
		w.value(f)
	}
	if w.text {
		return
	}
	_, methods := classBody(w.s.declOf(r.T))
	for _, m := range methods {
		in := w.instance(m, r)
		if in == nil {
			continue
		}
		site := w.s.fnObjs[m]
		vs := []value.Value{in.Result}
		if in.Table != nil {
			vs = append(vs, in.Table.Cells...)
		}
		w.pending = append(w.pending, pendingOutput{name: site.pkg + qnameSep + site.label, decl: site.span(), values: vs})
	}
}

// instance is m's Instance for r, found through an index built at m's first receiver.
func (w *positionWalk) instance(m *ExportFn, r *value.Record) *Instance {
	idx, ok := w.results[m]
	if !ok {
		idx = make(map[*value.Record]*Instance, len(m.Instances))
		for _, in := range m.Instances {
			idx[in.Recv] = in
		}
		w.results[m] = idx
	}
	return idx[r]
}

// report is W8031 at the marked leaf's site, once per site and package, with the output's declaration as the related note (CODEGEN.md §2.10).
func (w *positionWalk) report(v value.Value) {
	span := verify.SiteOf(v).Span
	if span.File == source.NoFile {
		span = w.decl
	}
	if w.sites[span] {
		return
	}
	w.sites[span] = true
	w.u.report(diag.W8031.At(span, w.name).Related(w.decl, diag.NoteSource(w.decl)))
}
