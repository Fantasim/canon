package ir

import "github.com/fantasim/canonlang/internal/diag"

// checkForeignDefines is E8005 `foreignDefine` for each name pl declares that a package u does not read writes as a #define (CODEGEN.md §3.5, §7.9, DECISIONS 403): a header of u cannot guard it, so one that included that defines header first could not compile. It is judged from the project's declarations alone, whatever the selection, at the declaring item (at the emit when there is none), once per (name, origin); the defines of u itself and of the packages it reads are no finding (CppGuardNames).
func (s *stage) checkForeignDefines(u *unit, es *emitSite, pl *CppNamePlan) {
	defs := s.projectDefines()
	if len(defs) == 0 {
		return
	}
	guard := nameSet(CppGuardNames(u.p)...)
	for _, d := range s.declaredNames(pl) {
		src, written := defs[d.name]
		if !written || guard[d.name] {
			continue
		}
		u.report(diag.E8005.AtForeignDefine(s.itemSpan(d.item, es.span()), d.name, d.origin, src.Origin))
	}
}
