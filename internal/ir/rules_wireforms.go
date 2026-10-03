package ir

import (
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"github.com/fantasim/canonlang/internal/wire"
)

// resultForms walks the precomputed results a JSON emit writes; stored are the records its values hold, verified in stage B.
type resultForms struct {
	u      *unit
	stored map[*value.Record]bool
	seen   map[*value.Record]bool
}

// checkFnResultForms is E8102: a stored export fn result a JSON emit writes, `$` keys and `$fns`, has a wire form (WIRE.md §5.1, §5.11; decision 283).
func (s *stage) checkFnResultForms(u *unit, es *emitSite) {
	rf := &resultForms{u: u, stored: map[*value.Record]bool{}, seen: map[*value.Record]bool{}}
	var recvs []*value.Record
	for _, v := range selectedValues(u, es.e) {
		walkInstances(v.v.V, rf.stored, func(r *value.Record) { recvs = append(recvs, r) })
	}
	for _, site := range u.fns {
		rf.results(site, site.fn.Value, site.fn.Table)
	}
	done := map[*ExportFn]bool{}
	for _, r := range recvs {
		for _, m := range s.methodsOf(r.T) {
			if !done[m] && s.fnObjs[m] != nil {
				done[m] = true
				rf.instances(s.fnObjs[m])
			}
		}
	}
}

// instances checks a method's results for the receivers this emit writes.
func (rf *resultForms) instances(site *fnSite) {
	for _, in := range site.fn.Instances {
		if rf.stored[in.Recv] {
			rf.results(site, in.Result, in.Table)
		}
	}
}

// results checks one result, or each cell of its table; a part with no location is reported at the fn.
func (rf *resultForms) results(site *fnSite, result value.Value, table *LookupTable) {
	at := site.span()
	rf.value(result, at)
	if table != nil {
		for _, c := range table.Cells {
			rf.value(c, at)
		}
	}
}

// value checks each field of the records in v that stage B did not verify as stored values.
func (rf *resultForms) value(v value.Value, at source.Span) {
	switch x := v.(type) {
	case *value.Record:
		if rf.stored[x] || rf.seen[x] {
			return
		}
		rf.seen[x] = true
		for i, f := range verify.Fields(x.T) {
			if i < len(x.Fields) {
				rf.field(f.Name, wire.FieldForms(f, x.Fields[i]), at)
				rf.value(x.Fields[i], at)
			}
		}
	case *value.List:
		for _, e := range x.Elems {
			rf.value(e, at)
		}
	case *value.Map:
		for _, e := range x.Vals {
			rf.value(e, at)
		}
	case *value.Table:
		for _, e := range x.Entries {
			rf.value(e, at)
		}
	}
}

func (rf *resultForms) field(name string, forms []wire.Formless, at source.Span) {
	for _, x := range forms {
		span := verify.SiteOf(x.Site).Span
		if span.File == source.NoFile {
			span = at
		}
		rf.u.report(x.At(span, name))
	}
}
