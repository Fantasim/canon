package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// checkCppDecoded is E8019 `MapField` and `InlineFoldedKey`: gen/cpp's loader decodes every class of the package (CODEGEN.md §7.6).
func (s *stage) checkCppDecoded(u *unit, es *emitSite) {
	shape := objectShape{extras: s.objectExtras(u, selectedValues(u, es.e)), deep: true}
	for _, class := range packageClasses(u.p) {
		fields, fns := classBody(class)
		for _, site := range s.decodedSites(fields, fns) {
			s.checkDecodedType(u, es, site)
		}
		s.checkInlineFolds(u, es, class, shape)
	}
}

// checkForeignPairs is E8019 `ForeignPairsField`: gen/cpp fills a pair record as its friend, which another package's record is not (CODEGEN.md §4.2).
func (s *stage) checkForeignPairs(u *unit, es *emitSite) {
	for _, class := range packageClasses(u.p) {
		fields, _ := classBody(class)
		for _, f := range fields {
			if f.Pairs == nil || f.Type.Kind != types.List || f.Type.Elem == nil {
				continue
			}
			if rec, ok := f.Type.Elem.Named.(*Record); ok && rec.Pkg != u.p.Name {
				u.reportGenConstruct(es, s.itemSpan(f, source.Span{}), diag.KindForeignPairsField)
			}
		}
	}
}

// checkSelfReads is E8019 `SelfReadNotAPath`: gen/cpp passes a translated method its receiver's field paths only (CONFORMANCE.md §2.3).
func (s *stage) checkSelfReads(u *unit, es *emitSite) {
	for _, class := range packageClasses(u.p) {
		fields, fns := classBody(class)
		for _, fn := range fns {
			if fn.Kind == FnTranslated && slices.ContainsFunc(fn.Reads, func(r *Read) bool { return !fieldPath(fields, r.Path) }) {
				u.reportGenConstruct(es, s.itemSpan(fn, source.Span{}), diag.KindSelfReadNotAPath)
			}
		}
	}
}

// fieldPath reports a path of fields from fields, every step but the last holding a record; an empty path is no read to judge.
func fieldPath(fields []*Field, path []string) bool {
	for i, name := range path {
		j := slices.IndexFunc(fields, func(f *Field) bool { return f.Name == name })
		if j < 0 {
			return false
		}
		rec, ok := fields[j].Type.Named.(*Record)
		if i+1 < len(path) && (fields[j].Type.Kind != types.Record || !ok) {
			return false
		}
		if ok {
			fields = rec.Fields
		}
	}
	return true
}

// checkClassCycles is E8019 where gen/cpp's class order meets a by-value cycle (CODEGEN.md §2.7, §7.2): `RecursiveVariantCase` through a variant, else `RecordCycleThroughMethod`.
func (s *stage) checkClassCycles(u *unit, es *emitSite) {
	g := newClassGraph(u.p)
	for _, c := range g.classes {
		if g.state[c] == unvisited {
			g.visit(c)
		}
	}
	for _, c := range g.refused {
		u.reportGenConstruct(es, s.itemSpan(c, source.Span{}), diag.KindRecursiveVariantCase)
	}
	for _, c := range g.refusedRecords {
		u.reportGenConstruct(es, s.itemSpan(c, source.Span{}), diag.KindRecordCycleThroughMethod)
	}
}
