package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
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

// checkRefUnions is E8019 `RefUnion`: gen/cpp holds no literal union over a ref, and gen/go's data loader reads none, wherever the emit's code does (CODEGEN.md §4.1).
func (s *stage) checkRefUnions(u *unit, es *emitSite) {
	read := s.readSpans(u, es, isRefUnion)
	for _, site := range s.typeSites(u, es) {
		if es.e.Target == TargetCpp && typeHolds(site.t, isRefUnion) || es.e.Target != TargetCpp && read[site.span] {
			u.reportGenConstruct(es, site.span, diag.KindRefUnion)
		}
	}
}

// isRefUnion is a literal union over a ref written as a string (an Int-keyed one is check's E3002).
func isRefUnion(t *TypeRef) bool {
	return t.Kind == types.LitUnion && t.Elem != nil && t.Elem.Kind == types.Ref && StringWire(t.Elem)
}
