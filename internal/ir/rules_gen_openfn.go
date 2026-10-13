package ir

import "github.com/fantasim/canonlang/internal/diag"

// checkOpenEnumFns is E8019 `OpenEnumFn` once per translated function of u whose parameter type, or a path of self its body reads, holds an enum the emit opens, at any depth (DECISIONS 365, CONFORMANCE.md §2.3): a decoder keeps a member the evaluator never holds, so no vector proves the function's answer for it.
func (s *stage) checkOpenEnumFns(u *unit, es *emitSite) {
	for _, site := range s.ownFns(u) {
		if site.fn.Kind == FnTranslated && readsOpenEnum(u.p, es.e, site.fn) {
			u.reportGenConstruct(es, site.span(), diag.KindOpenEnumFn)
		}
	}
}

// readsOpenEnum reports a translated fn taking or reading, from self, a type that holds an enum of any package that e opens.
func readsOpenEnum(p *Package, e *Emit, fn *ExportFn) bool {
	w := openWalk{p: p, e: e, seen: map[any]bool{}, own: true}
	for _, prm := range fn.Params {
		w.typ(&prm.Type)
	}
	for _, r := range fn.Reads {
		w.typ(&r.Type)
	}
	return len(w.found) > 0
}
