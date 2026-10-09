package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// foreignJudge is one limit a generator meets building class c, a class of another package than u's that emit e reads or writes (CODEGEN.md §2.8, DECISIONS 320, 323): the E8019 kind it refuses, if any.
type foreignJudge func(s *stage, u *unit, e *Emit, c any) (diag.Kind, bool)

// foreignReadJudges are the limits of each target's readers on another package's classes, by mode, those its readers have on the package's own (checkGoDecoded, checkCppDecoded, checkTSDecoded and their dependents' rules): Q reads P's classes with its own readers, whatever P's mode (DECISIONS 323).
var foreignReadJudges [targetCount][ModeTypes + 1][]foreignJudge

func init() {
	foreignReadJudges[TargetGo][ModeData] = []foreignJudge{judgeMaps, judgeDependents, judgeCaseReads, judgeRefUnions, judgeInlineFolds, judgeLookupParams}
	foreignReadJudges[TargetCpp][ModeData] = []foreignJudge{judgeMaps, judgeDependents, judgeInlineFolds, judgeLookupParams}
	foreignReadJudges[TargetCpp][ModeTypes] = []foreignJudge{judgeMaps, judgeDependents, judgeCppDefaults}
	foreignReadJudges[TargetGo][ModeTypes] = []foreignJudge{judgeMaps, judgeDependents, judgeCaseReads, judgeRefUnions, judgeGoDefaults}
	ts := []foreignJudge{judgeTSFields, judgeLookupParams}
	foreignReadJudges[TargetTS][ModeData], foreignReadJudges[TargetTS][ModeTypes] = ts, ts
}

// foreignReaches calls visit with each class of another package each root of emit es reaches (foreignRoots, the walk ForeignUses makes), with the root's span, in root order, then in first-reach order: a root is a value, constant, export fn or, in types mode, a field or dependent type of u, so a finding on another package's class sits at the site of this package that builds it. readOnly keeps the roots readers read.
func (s *stage) foreignReaches(u *unit, es *emitSite, readOnly bool, visit func(at source.Span, use *ForeignUse, c any)) {
	for _, r := range foreignRoots(s.view(u, es), es.e) {
		if readOnly && !r.read {
			continue
		}
		use := &ForeignUse{variantOf: map[*Case]*Variant{}}
		w := newForeignWalk(u.p.Name, use)
		w.root(r)
		at := s.itemSpan(r.item, es.span())
		for _, c := range w.out {
			visit(at, use, c)
		}
	}
}

// checkForeignReads is E8019 at each site of u whose readers read a class of another package that the emit's readers cannot read (foreignReadJudges), in data and types mode.
func (s *stage) checkForeignReads(u *unit, es *emitSite) {
	judges := foreignReadJudges[es.e.Target][es.e.Mode]
	s.foreignReaches(u, es, true, func(at source.Span, _ *ForeignUse, c any) {
		for _, judge := range judges {
			if kind, bad := judge(s, u, es.e, c); bad {
				u.reportGenConstruct(es, at, kind)
			}
		}
	})
}

// checkForeignBuilt is E8019 `ForeignResolvedRef` at each site of u whose go or cpp code reads or writes a record or case of another package whose owner writes no make hook for it: a ref its data or types-mode loader resolves inside its own values (CODEGEN.md §2.8 Refs, §5.14; log-2026-10-06 "U1 review" 3).
func (s *stage) checkForeignBuilt(u *unit, es *emitSite) {
	s.foreignReaches(u, es, false, func(at source.Span, use *ForeignUse, c any) {
		if !s.hookWritten(es.e.Target, use, c) {
			u.reportGenConstruct(es, at, diag.KindForeignResolvedRef)
		}
	})
}

// ownerKey is a package and a target, whose own name plan tells which hooks it writes.
type ownerKey struct {
	pkg string
	t   Target
}

// hookWritten reports that the owner of class c writes its make hook in target t, a record's or each case's of a variant or case, as the owner's own plan says (GoHook.Written, CppHook.Written); a dependent type's branches have no resolved ref; an owner without an emit of t is E8004's.
func (s *stage) hookWritten(t Target, use *ForeignUse, c any) bool {
	written := s.ownerWrites(ownerKey{use.classPkg(c), t})
	if written == nil {
		return true
	}
	switch x := c.(type) {
	case *Record:
		return written(x, nil)
	case *Variant:
		return !slices.ContainsFunc(x.Cases, func(cs *Case) bool { return !written(x, cs) })
	case *Case:
		return written(use.VariantOf(x), x)
	}
	return true
}

// ownerWrites is the owner's Written for a record (cs nil) or a case of target k.t, from its own plan, built once per stage: from its IR when it is selected, else from its declarations (declaredOnly); nil without an emit of that target, or with one whose mode is refused (E8009, or E8019 `unbuilt`: its own finding stands, log-2026-10-06 "U5 review FAIL").
func (s *stage) ownerWrites(k ownerKey) func(class Type, cs *Case) bool {
	if f, ok := s.owners[k]; ok {
		return f
	}
	u := s.units[k.pkg]
	var f func(Type, *Case) bool
	if es := ownerEmitSite(u, k.t); es != nil && !modeRefused(es.e) {
		f = planWrites(s.ownerIR(u), es.e)
	}
	s.owners[k] = f
	return f
}

// ownerEmitSite is u's emit of target t, nil for no unit or no such emit.
func ownerEmitSite(u *unit, t Target) *emitSite {
	if u == nil {
		return nil
	}
	return emitFor(u, t)
}

// ownerIR is u's IR: assembled when u is selected and emits, else its declarations, values unevaluated.
func (s *stage) ownerIR(u *unit) *Package {
	if u.selected && len(u.emits) > 0 {
		return u.p
	}
	return s.declaredOnly(u)
}

// planWrites is the Written of p's own hooks in emit e's plan: go and cpp have hooks, other targets none.
func planWrites(p *Package, e *Emit) func(Type, *Case) bool {
	if e.Target == TargetGo {
		pl := PlanGoNames(p, e)
		return func(class Type, cs *Case) bool {
			if v, ok := class.(*Variant); ok {
				return pl.CaseHook(v, cs).Written
			}
			return pl.RecordHook(class.(*Record)).Written
		}
	}
	if e.Target == TargetCpp {
		pl := PlanCppNames(p, e)
		return func(class Type, cs *Case) bool {
			if v, ok := class.(*Variant); ok {
				return pl.CaseHook(v, cs).Written
			}
			return pl.RecordHook(class.(*Record)).Written
		}
	}
	return nil
}

// checkForeignTypesMode is E8014 at each site of a types-mode emit of u that reads a record or case of another package with what types mode cannot emit on its own records: a precomputed or lookup export fn, a computed default (CODEGEN.md §5.13; log-2026-10-06 "U4 (gen/ts) done" (f) and "Scribe follow-up 3").
func (s *stage) checkForeignTypesMode(u *unit, es *emitSite) {
	s.foreignReaches(u, es, true, func(at source.Span, use *ForeignUse, c any) {
		for _, class := range classAndCases(c) {
			reportTypesData(u, at, use.classOrigin(class)+qnameSep, class)
		}
	})
}

// reportTypesData is E8014 at at for each stored fn and computed default of class, named under origin.
func reportTypesData(u *unit, at source.Span, origin string, class any) {
	fields, fns := classBody(class)
	for _, fn := range fns {
		if what, stored := typesFnKinds[fn.Kind]; stored {
			u.report(diag.E8014.At(at, what, origin+fn.Name))
		}
	}
	for _, f := range fields {
		if f.Computed {
			u.report(diag.E8014.At(at, diag.KindComputedDefault, origin+f.Name))
		}
	}
}

// typesFnKinds are the export fn kinds types mode has no data for, with E8014's word (CODEGEN.md §5.13).
var typesFnKinds = map[FnKind]diag.Kind{FnPrecomputed: diag.KindPrecomputedFunction, FnLookup: diag.KindLookupFunction}

// classAndCases is a record or case itself, or each case of a variant: a case without fields stores its results too (log-2026-10-06 "U1 review" 6).
func classAndCases(c any) []any {
	v, ok := c.(*Variant)
	if !ok {
		return []any{c}
	}
	out := make([]any, len(v.Cases))
	for i, cs := range v.Cases {
		out[i] = cs
	}
	return out
}

// checkLegacyStructs is E8019 `LegacyStruct` at each record mapped onto a hand-written C++ struct (@cpp(struct:) or @cpp(access:)): gen/cpp writes none before M6 (CODEGEN.md §7.8, DECISIONS 320).
func (s *stage) checkLegacyStructs(u *unit, es *emitSite) {
	for _, t := range u.p.Types {
		if r, ok := t.(*Record); ok && (r.Cpp.Struct != "" || r.Cpp.Access != AccessNone) {
			u.reportGenConstruct(es, s.itemSpan(r, source.Span{}), diag.KindLegacyStruct)
		}
	}
}

// judgeMaps is `MapField` for a map the emit's loader cannot read in what it reads of c (checkDecodedType).
func judgeMaps(_ *stage, _ *unit, e *Emit, c any) (diag.Kind, bool) {
	fields, fns := classBody(c)
	bad := slices.ContainsFunc(readSites(fields, readFns(e, fns), noSpan), func(site typeSite) bool { return typeHolds(site.t, unreadMap(e)) })
	return diag.KindMapField, bad
}

// judgeDependents is fieldDependent at each field of c, then `DependentOutsideField` for a stored fn's dependent result (reportDecodedDependents).
func judgeDependents(_ *stage, _ *unit, e *Emit, c any) (diag.Kind, bool) {
	fields, fns := classBody(c)
	for _, f := range fields {
		if kind, bad := fieldDependent(e, fields, f); bad {
			return kind, true
		}
	}
	unread := slices.ContainsFunc(readFns(e, fns), func(fn *ExportFn) bool { return fn.Kind != FnTranslated && unreadDependent(e, &fn.Result) })
	return diag.KindDependentOutsideField, unread
}

// judgeCaseReads is `CaseField` for a case used as a type in what gen/go's data loader reads of c (checkCaseFields).
func judgeCaseReads(_ *stage, _ *unit, e *Emit, c any) (diag.Kind, bool) {
	fields, fns := classBody(c)
	bad := slices.ContainsFunc(readSites(fields, readFns(e, fns), noSpan), func(site typeSite) bool {
		return typeHolds(site.t, func(t *TypeRef) bool { return t.Kind == types.Case })
	})
	return diag.KindCaseField, bad
}

// judgeRefUnions is `RefUnion` for a literal union over a ref in what gen/go's data loader reads of c (checkRefUnions).
func judgeRefUnions(_ *stage, _ *unit, e *Emit, c any) (diag.Kind, bool) {
	fields, fns := classBody(c)
	bad := slices.ContainsFunc(readSites(fields, readFns(e, fns), noSpan), func(site typeSite) bool { return typeHolds(site.t, isRefUnion) })
	return diag.KindRefUnion, bad
}

// judgeInlineFolds is `InlineFoldedKey` for an inline key of c folding onto another key of its object (checkInlineFolds), with a case's tag and a row's `$id` and `$retired` among them; gen/cpp lists inline case keys deep.
func judgeInlineFolds(_ *stage, _ *unit, e *Emit, c any) (diag.Kind, bool) {
	shape := objectShape{extras: map[any][]string{c: {fpDollar + GoIDStore, fpDollar + GoRetiredStore}}, deep: e.Target == TargetCpp}
	return diag.KindInlineFoldedKey, len(inlineFolds(c, shape)) > 0
}

// judgeLookupParams is `ForeignTableLookupParam` for a lookup method of c whose ref parameter's table has no id enum in the emit's target: its reader reads the cells by the id enum's wire keys, a ts reader by the owner's id set (log-2026-10-06 "U2 fixes done", "U5 review FAIL" 1).
func judgeLookupParams(_ *stage, u *unit, e *Emit, c any) (diag.Kind, bool) {
	_, fns := classBody(c)
	bad := slices.ContainsFunc(fns, func(fn *ExportFn) bool {
		return fn.Kind == FnLookup && slices.ContainsFunc(fn.Params, func(p *Param) bool { return noIDEnum(u.p, e.Target, p.Type) })
	})
	return diag.KindForeignTableLookupParam, bad
}

// judgeCppDefaults is cppDefault at each field of c: a types-mode decoder writes an absent key's default.
func judgeCppDefaults(_ *stage, _ *unit, e *Emit, c any) (diag.Kind, bool) {
	return firstDefault(e, c, cppDefault)
}

// judgeTSFields is tsField at each field of c, then tsUnreadType at each stored fn's result its reader reads.
func judgeTSFields(_ *stage, _ *unit, e *Emit, c any) (diag.Kind, bool) {
	fields, fns := classBody(c)
	for _, f := range fields {
		if kind, bad := tsField(fields, f); bad {
			return kind, true
		}
	}
	for _, fn := range readFns(e, fns) {
		if kind, bad := tsUnreadType(&fn.Result, true); fn.Kind != FnTranslated && bad {
			return kind, true
		}
	}
	return 0, false
}
