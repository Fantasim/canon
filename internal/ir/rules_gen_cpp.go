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

// decodesClasses reports an emit whose generator decodes the package's classes from JSON: a data loader, or gen/cpp's and gen/go's types-mode decoders (CODEGEN.md §5.13).
func decodesClasses(e *Emit) bool {
	return e.Mode == ModeData || e.Target != TargetTS && e.Mode == ModeTypes
}

// readsMaps reports an emit whose decoders read maps, keys as WIRE.md §5.8 in file order: a data loader, or gen/go's types-mode decoders, which read the JSON text; gen/cpp's take an nlohmann::json, which keeps no key order (CODEGEN.md §5.9, §5.13).
func readsMaps(e *Emit) bool {
	return e.Mode == ModeData || e.Target == TargetGo && e.Mode == ModeTypes
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

// checkCppDefaults is E8019 where a types-mode decoder cannot write a field's constant default when its key is absent (CODEGEN.md §5.13), at each field of the package's classes (cppDefault).
func (s *stage) checkCppDefaults(u *unit, es *emitSite) { s.reportDefaults(u, es, cppDefault) }

// defaultJudge is what a types-mode decoder of emit e cannot write of field f's constant default, f one of fields.
type defaultJudge func(e *Emit, fields []*Field, f *Field) (diag.Kind, bool)

// reportDefaults is E8019 at each field of the package's classes whose default judge refuses.
func (s *stage) reportDefaults(u *unit, es *emitSite, judge defaultJudge) {
	for _, class := range packageClasses(u.p) {
		fields, _ := classBody(class)
		for _, f := range fields {
			if kind, bad := judge(es.e, fields, f); bad {
				u.reportGenConstruct(es, s.itemSpan(f, source.Span{}), kind)
			}
		}
	}
}

// firstDefault is the first refusal of judge at a field of class c, another package's that a reader reads.
func firstDefault(e *Emit, c any, judge defaultJudge) (diag.Kind, bool) {
	fields, _ := classBody(c)
	for _, f := range fields {
		if kind, bad := judge(e, fields, f); bad {
			return kind, true
		}
	}
	return 0, false
}

// cppDefault is what a types-mode decoder cannot write of field f's constant default: `RecordDefault` for one holding a record or case value (gen/cpp's literals write none, §5.1), `DependentDefault` for a present value of a dependent type the decoder otherwise reads, whose branch only a discriminant decides (§5.6).
func cppDefault(e *Emit, fields []*Field, f *Field) (diag.Kind, bool) {
	switch {
	case f.Input != nil || !written(f.Default):
		return 0, false
	case holdsRecordValue(f.Default):
		return diag.KindRecordDefault, true
	case typeHolds(&f.Type, isApp) && readsField(e, fields, f):
		return diag.KindDependentDefault, true
	}
	return 0, false
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

// checkClassCycles is E8019 where gen/cpp's class order meets a by-value cycle (CODEGEN.md §2.7, §7.2): `RecursiveVariantCase` through a variant, else `RecordFieldCycle` (DECISIONS 292).
func (s *stage) checkClassCycles(u *unit, es *emitSite) {
	g := newClassGraph(u.p)
	for _, c := range g.classes {
		if g.state[c] == unvisited {
			g.visit(c, false)
		}
	}
	for _, c := range g.refused {
		u.reportGenConstruct(es, s.itemSpan(c, source.Span{}), diag.KindRecursiveVariantCase)
	}
	for _, c := range g.refusedRecords {
		u.reportGenConstruct(es, s.itemSpan(c, source.Span{}), diag.KindRecordFieldCycle)
	}
}
