package ir

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"github.com/fantasim/canonlang/internal/wire"
)

// textWalk is stage E's walk of one `@text` result written as JSON: each part with no wire form, and each map's keys that share a text (WIRE.md §5.1, §5.8; DECISIONS 283, 308). stored holds the composites of the lets of every package, which stage B verified, shared by the whole stage and never written; seen the records and maps the package's walks already met.
type textWalk struct {
	u      *unit
	at     source.Span
	stored map[value.Value]bool
	seen   map[value.Value]bool
}

// checkTextResults is E8151, E8102 and E3317 on the `@text` fns of a package with `emit text` whose result is not text (WIRE.md §5.9, §8.5; CODEGEN.md §2.9; DECISIONS 308).
func (s *stage) checkTextResults(u *unit, _ *emitSite) {
	var seen map[value.Value]bool
	for _, site := range u.fns {
		if !site.text || isText(site.fn.Result) {
			continue
		}
		if what := wireFind(site.sig.Result, map[types.Type]bool{}, noWire); what != nil {
			u.report(refusedResult(site, what))
			continue
		}
		if seen == nil {
			seen = map[value.Value]bool{}
		}
		w := &textWalk{u: u, at: site.span(), stored: s.letComposites(), seen: seen}
		w.value(site.fn.Value)
	}
}

// letComposites are the records, lists, maps and tables held by the values of the lets of every package, public or local: stage B verified them where they are written, so a `@text` result returning an imported let repeats no finding. The set is the same for every package, so the stage collects it once, at the first `@text` result walked.
func (s *stage) letComposites() map[value.Value]bool {
	if s.lets != nil {
		return s.lets
	}
	s.lets = map[value.Value]bool{}
	for _, u := range s.order {
		s.unitComposites(u, s.lets)
	}
	return s.lets
}

// unitComposites adds the composites of u's lets to out.
func (s *stage) unitComposites(u *unit, out map[value.Value]bool) {
	for _, obj := range u.cp.Decls {
		if obj.Kind() != check.ObjLet {
			continue
		}
		if v, ok := s.in.Host.Value(s.ctx, obj.Pkg(), obj.Name()); ok {
			collectComposites(v, out)
		}
	}
}

// collectComposites adds v and every composite it holds to set.
func collectComposites(v value.Value, set map[value.Value]bool) {
	if set[v] {
		return
	}
	switch x := v.(type) {
	case *value.Record:
		set[v] = true
		collectEach(x.Fields, set)
	case *value.List:
		set[v] = true
		collectEach(x.Elems, set)
	case *value.Map:
		set[v] = true
		collectEach(x.Keys, set)
		collectEach(x.Vals, set)
	case *value.Table:
		set[v] = true
		for _, e := range x.Entries {
			collectComposites(e, set)
		}
	}
}

func collectEach(vs []value.Value, set map[value.Value]bool) {
	for _, v := range vs {
		if v != nil {
			collectComposites(v, set)
		}
	}
}

// refusedResult is E8151 for a `@text` fn whose result type holds what, which has no wire form.
func refusedResult(site *fnSite, what types.Type) *diag.Builder {
	if isDefineType(what.Base()) {
		return diag.E8151.AtDefine(site.span(), site.label)
	}
	return diag.E8151.AtType(site.span(), site.label, site.sig.Result)
}

// value walks v, skipping what stage B verified, and each composite once.
func (w *textWalk) value(v value.Value) {
	if v == nil || w.stored[v] || w.seen[v] {
		return
	}
	switch x := v.(type) {
	case *value.Record:
		w.seen[v] = true
		w.record(x)
	case *value.List:
		for _, e := range x.Elems {
			w.value(e)
		}
	case *value.Map:
		w.seen[v] = true
		w.mapping(x)
	case *value.Table:
		for _, e := range x.Entries {
			w.value(e)
		}
	}
}

// record reports the fields of r with no wire form, then walks them (E8102 names the field, as the verifier does).
func (w *textWalk) record(r *value.Record) {
	for i, f := range verify.Fields(r.T) {
		if i >= len(r.Fields) {
			break
		}
		for _, x := range wire.FieldForms(f, r.Fields[i]) {
			w.report(x, f.Name)
		}
		w.value(r.Fields[i])
	}
}

// report is E8102 for x, at the part's own span when it has one.
func (w *textWalk) report(x wire.Formless, field string) {
	span := verify.SiteOf(x.Site).Span
	if span.File == source.NoFile {
		span = w.at
	}
	w.u.report(x.At(span, field))
}

// mapping reports a key of m whose text an earlier, different key has (E3317, at the second), then walks the values.
func (w *textWalk) mapping(m *value.Map) {
	first := map[string]value.Value{}
	for i, k := range m.Keys {
		if i < len(m.Vals) {
			w.value(m.Vals[i])
		}
		text, err := wire.KeyText(k)
		if err != nil {
			continue
		}
		prev, dup := first[text]
		switch {
		case !dup:
			first[text] = k
		case !value.Equal(prev, k):
			span := verify.SiteOf(k).Span
			if span.File == source.NoFile {
				span = w.at
			}
			w.u.report(diag.E3317.At(span, verify.MapKeyArg{V: prev}, verify.MapKeyArg{V: k}, text))
		}
	}
}
