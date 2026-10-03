package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// checkTSLiterals is E8019 where a baked or embedded ts emit cannot write a literal (CODEGEN.md §5.6, §5.9, §5.10, DECISIONS 278): `DependentType` for a dependent value whose discriminant the literal does not read (in a map, a literal union, a fn result, or through a ref); `CrossPackageBakedValue` for a plain value of another package's record whose interface requires `id` (judge). Constants and stored fns are written too.
func (s *stage) checkTSLiterals(u *unit, es *emitSite) {
	lit := &tsLit{own: u.p.Name, strict: s.tsStrictRows(u)}
	for _, v := range selectedValues(u, es.e) {
		if kind, bad := lit.judge(&v.v.Type, v.v.V); bad {
			u.reportGenConstruct(es, v.span().span(), kind)
		}
	}
	for _, c := range u.consts {
		if kind, bad := lit.judge(&c.c.Type, c.c.V); bad {
			u.reportGenConstruct(es, c.span(), kind)
		}
	}
	for _, site := range s.ownFns(u) {
		if site.fn.Kind == FnTranslated {
			continue
		}
		for _, r := range storedResults(site.fn) {
			if kind, bad := lit.judge(&site.fn.Result, r); bad {
				u.reportGenConstruct(es, site.span(), kind)
				break
			}
		}
	}
}

// tsLit judges literals gen/ts writes for package own.
type tsLit struct {
	own    string
	strict func(*Record) bool
}

// judge is `DependentType` for a dependent value the literal cannot place, then `CrossPackageBakedValue` for a plain value of another package's record whose interface requires `id` (a row there, never a plain value: CODEGEN.md §5.4).
func (l *tsLit) judge(t *TypeRef, v value.Value) (diag.Kind, bool) {
	if literalDependent(l.own, t, v, cppDiscRead) {
		return diag.KindDependentType, true
	}
	plainRow := literalHolds(t, v, func(t *TypeRef, v value.Value) bool {
		_, ok := v.(*value.Record)
		rec, isRec := t.Named.(*Record)
		return ok && isRec && t.Kind == types.Record && l.strict(rec)
	})
	return diag.KindCrossPackageBakedValue, plainRow
}

// tsStrictRows reports a record of another package that its own ts emit writes as a table row with a required `id`: one it never writes as a plain value (ir.TSRows there).
func (s *stage) tsStrictRows(u *unit) func(*Record) bool {
	cache := map[string]map[*Record]bool{}
	return func(r *Record) bool {
		if r.Pkg == u.p.Name || s.units[r.Pkg] == nil {
			return false
		}
		strict, ok := cache[r.Pkg]
		if !ok {
			strict = map[*Record]bool{}
			if e := tsEmitOf(u.p, r.Pkg); e != nil {
				rows, loose := TSRows(s.units[r.Pkg].p, e)
				for rec := range rows { //canon:unordered a set
					strict[rec] = !loose[rec]
				}
			}
			cache[r.Pkg] = strict
		}
		return strict[r]
	}
}

// tsDefault is what a ts reader cannot write of a field's constant default (DECISIONS 278): a dependent field's present default, whose branch an absent key's discriminant decides at run time (as gen/cpp, checkCppDefaults); a default whose stored results hold their own receiver is a cyclic fn, refused at the fn (DECISIONS 284).
func tsDefault(f *Field) (diag.Kind, bool) {
	switch {
	case f.Computed || !written(f.Default):
		return 0, false
	case typeHolds(&f.Type, isApp):
		return diag.KindDependentType, true
	}
	return 0, false
}

// checkTSForeignTables is E8019 in every ts mode (CODEGEN.md §5.3, §5.4): a table of another package's record, held anywhere, has rows with `id` and `retired` that the record's interface in its own module does not declare, and no reader here reads them; `CrossPackageBakedValue` in baked and embedded mode, `ForeignDataRecord` in data and types mode, where readers read it.
func (s *stage) checkTSForeignTables(u *unit, es *emitSite) {
	own, kind := u.p.Name, diag.KindForeignDataRecord
	if !decodes(es.e) {
		kind = diag.KindCrossPackageBakedValue
	}
	for _, site := range s.typeSites(u, es) {
		if typeHolds(site.t, func(t *TypeRef) bool { return foreignTable(own, *t) }) {
			u.reportGenConstruct(es, site.span, kind)
		}
	}
}

// decodes reports a ts emit whose module has readers: data and types mode.
func decodes(e *Emit) bool { return e.Mode == ModeData || e.Mode == ModeTypes }
