package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// checkCppDecoded is E8019 `MapField` and, in data mode, `InlineFoldedKey`: gen/cpp decodes every class of the package (CODEGEN.md §7.6); a types-mode decoder ignores unknown keys (§5.13), so no key of its parent folds onto an inline case's.
func (s *stage) checkCppDecoded(u *unit, es *emitSite) {
	shape := objectShape{extras: s.objectExtras(u, selectedValues(u, es.e)), deep: true}
	for _, class := range packageClasses(u.p) {
		fields, fns := classBody(class)
		for _, site := range s.decodedSites(fields, readFns(es.e, fns)) {
			s.checkDecodedType(u, es, site)
		}
		if es.e.Mode == ModeData {
			s.checkInlineFolds(u, es, class, shape)
		}
	}
}

// decodesClasses reports an emit whose generator decodes the package's classes from JSON: a data loader, or gen/cpp's types-mode decoders (CODEGEN.md §5.13).
func decodesClasses(e *Emit) bool {
	return e.Mode == ModeData || e.Target == TargetCpp && e.Mode == ModeTypes
}

// readFns are the export fns of a class whose results a decoder reads: none in types mode, where a stored fn is E8014's (CODEGEN.md §5.13).
func readFns(e *Emit, fns []*ExportFn) []*ExportFn {
	if e.Mode == ModeTypes {
		return nil
	}
	return fns
}

// checkTypesInputs is E8019 `InputField`: a types-mode emit writes no LoadInputs to read one (CODEGEN.md §2.2, §5.12).
func (s *stage) checkTypesInputs(u *unit, es *emitSite) {
	s.eachOwnField(u, func(_ string, f *Field) {
		if f.Input != nil {
			u.reportGenConstruct(es, s.fieldSites[f].span(), diag.KindInputField)
		}
	})
}

// checkCppDefaults is E8019 where a types-mode decoder cannot write a field's constant default when its key is absent (CODEGEN.md §5.13): `RecordConstant` for one holding a record or case value (gen/cpp's literals write none, §5.1), `DependentType` for a present value of a dependent type the decoder otherwise reads, whose branch only a discriminant decides (§5.6).
func (s *stage) checkCppDefaults(u *unit, es *emitSite) {
	for _, class := range packageClasses(u.p) {
		fields, _ := classBody(class)
		for _, f := range fields {
			switch {
			case f.Input != nil || !written(f.Default):
			case holdsRecordValue(f.Default):
				u.reportGenConstruct(es, s.itemSpan(f, source.Span{}), diag.KindRecordConstant)
			case typeHolds(&f.Type, isApp) && readsField(u.p.Name, fields, f, cppDiscRead):
				u.reportGenConstruct(es, s.itemSpan(f, source.Span{}), diag.KindDependentType)
			}
		}
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
