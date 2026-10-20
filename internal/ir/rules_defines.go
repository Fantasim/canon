package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
)

// checkDefineKeys is E8019 `DefineKey` at a baked lookup's parameter refing a define table: no ordinal (CODEGEN.md §5.10).
func (s *stage) checkDefineKeys(u *unit, es *emitSite) {
	for _, site := range s.ownFns(u) {
		if site.fn.Kind != FnLookup {
			continue
		}
		for i, p := range site.fn.Params {
			if i < len(site.sig.Params) && DefinesRef(p.Type) {
				u.reportGenConstruct(es, s.itemSpan(p, site.span()), diag.KindDefineKey)
			}
		}
	}
}
