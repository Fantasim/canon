package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
)

// checkTSLiterals is E8019 where a baked or embedded ts emit cannot write a literal (CODEGEN.md §5.6, §5.9, §5.10, DECISIONS 278): a dependent value whose discriminant the literal does not read (literalDependent: in a map, a literal union, a fn result, or through a ref). Constants and stored fns are written too; another package's records are object literals (§2.8, DECISIONS 323).
func (s *stage) checkTSLiterals(u *unit, es *emitSite) {
	for _, v := range selectedValues(u, es.e) {
		if kind, bad := literalDependent(&v.v.Type, v.v.V); bad {
			u.reportGenConstruct(es, v.span().span(), kind)
		}
	}
	for _, c := range u.consts {
		if kind, bad := literalDependent(&c.c.Type, c.c.V); bad {
			u.reportGenConstruct(es, c.span(), kind)
		}
	}
	for _, site := range s.ownFns(u) {
		if site.fn.Kind == FnTranslated {
			continue
		}
		for _, r := range storedResults(site.fn) {
			if kind, bad := literalDependent(&site.fn.Result, r); bad {
				u.reportGenConstruct(es, site.span(), kind)
				break
			}
		}
	}
}

// tsDefault is what a ts reader cannot write of a field's constant default (DECISIONS 278): `DependentDefault` for a dependent field's present default, whose branch an absent key's discriminant decides at run time (as gen/cpp, checkCppDefaults); a default whose stored results hold their own receiver is a cyclic fn, refused at the fn (DECISIONS 284).
func tsDefault(f *Field) (diag.Kind, bool) {
	switch {
	case f.Computed || !written(f.Default):
		return 0, false
	case typeHolds(&f.Type, isApp):
		return diag.KindDependentDefault, true
	}
	return 0, false
}
