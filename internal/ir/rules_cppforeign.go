package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// checkForeignDefines is E8005 (foreignFinding: foreignDefine, foreignId, foreignDerived or foreignFixed) for each name pl declares that a package u does not read writes as a #define (CODEGEN.md §3.5, §7.9, DECISIONS 403): a header of u cannot guard it, so one that included that defines header first could not compile. It is judged from the project's declarations alone, whatever the selection, at the declaring item (at the emit when there is none), once per (name, origin, way), so a name met in two ways is two findings; the defines of u itself and of the packages it reads are no finding (CppGuardNames).
func (s *stage) checkForeignDefines(u *unit, es *emitSite, pl *CppNamePlan) {
	s.foreignDefinesOf(u, es, s.declaredNames(pl))
}

// foreignDefinesOf is checkForeignDefines over the names a plan, or every mode's plan, declares.
func (s *stage) foreignDefinesOf(u *unit, es *emitSite, names []declaredName) {
	defs := s.projectDefines()
	if len(defs) == 0 {
		return
	}
	guard := nameSet(CppGuardNames(u.p)...)
	for _, d := range names {
		src, written := defs[d.name]
		if !written || guard[d.name] {
			continue
		}
		u.report(foreignFinding(d)(s.itemSpan(d.item, es.span()), d.name, d.hint(), src.Origin))
	}
}

// foreignFinding is the variant of d's finding (CODEGEN.md §7.9): `foreignId` for a table id, `foreignDerived` for a name built on the Canon name of a value, field, fn or parameter or of a dependent type's arm, which @cpp(name:) does not change, `foreignFixed` for a name the generator writes whatever the declarations say, else `foreignDefine`, which @cpp(name:) on the declaration renames.
func foreignFinding(d declaredName) func(source.Span, string, string, string) *diag.Builder {
	switch {
	case d.id:
		return diag.E8005.AtForeignId
	case d.note.derived:
		return diag.E8005.AtForeignDerived
	case d.note.fixed:
		return diag.E8005.AtForeignFixed
	}
	return diag.E8005.AtForeignDefine
}
