package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
)

// emitRules are the per-emit rules of each target (CODEGEN.md §12, WIRE.md §8.1).
var emitRules [TargetView + 1][]func(*stage, *unit, *emitSite)

// modeRules are the rules of a code emit's mode (CODEGEN.md §2.2, §2.8).
var modeRules [ModeTypes + 1][]func(*stage, *unit, *emitSite)

func init() {
	code := []func(*stage, *unit, *emitSite){(*stage).checkImports, (*stage).checkMode}
	emitRules[TargetGo], emitRules[TargetCpp] = code, code
	emitRules[TargetTS] = append(slices.Clone(code), (*stage).checkSafeInts, (*stage).checkNoInputs)
	emitRules[TargetJSON] = []func(*stage, *unit, *emitSite){(*stage).checkWireForms}
	modeRules[ModeEmbedded] = []func(*stage, *unit, *emitSite){(*stage).checkContainers, (*stage).checkDecoders}
	modeRules[ModeData] = []func(*stage, *unit, *emitSite){(*stage).checkContainers, (*stage).checkDecoders, (*stage).checkDataFns}
	modeRules[ModeTypes] = []func(*stage, *unit, *emitSite){(*stage).checkDecoders, (*stage).checkTypesMode}
}

// validate reports every emit rule of one package, writing nothing (EVALUATION.md §1 phase 7).
func (s *stage) validate(u *unit) {
	s.checkFnSignatures(u)
	if hasCodeEmit(u) {
		s.checkOrderedCodes(u)
		s.checkRepresentable(u)
		s.checkBranches(u)
		s.checkOverrideNames(u)
		s.checkNameCollisions(u)
	}
	for _, es := range u.emits {
		for _, rule := range emitRules[es.e.Target] {
			rule(s, u, es)
		}
	}
	s.checkDataLink(u)
	s.checkReload(u)
}

func isCode(t Target) bool { return t == TargetGo || t == TargetCpp || t == TargetTS }

func hasCodeEmit(u *unit) bool {
	return slices.ContainsFunc(u.emits, func(es *emitSite) bool { return isCode(es.e.Target) })
}

// selectedValues are the value sites an emit selects, in its order.
func selectedValues(u *unit, e *Emit) []*valueSite {
	var out []*valueSite
	for _, name := range selectedNames(u, e) {
		for _, v := range u.values {
			if v.v.Name == name {
				out = append(out, v)
			}
		}
	}
	return out
}

func (v *valueSite) span() declSite { return declSite{file: v.obj.File(), node: v.decl.Name} }

// checkImports is E8004: a package whose types this code emit uses has an emit of its target (CODEGEN.md §2.8).
func (s *stage) checkImports(u *unit, es *emitSite) {
	for _, imp := range u.p.Imports {
		if !slices.ContainsFunc(imp.Emits, func(e *Emit) bool { return e.Target == es.e.Target }) {
			u.report(diag.E8004.At(es.span(), u.firstUse[imp.Name], imp.Name, targetWords[es.e.Target]))
		}
	}
	if es.e.Target == TargetGo {
		s.checkImportedGoRoots(u, es)
	}
}

// checkImportedGoRoots is E8007 for an unselected dependency's own go emit: it never checked itself, so the importer reports it, whose build would else reference a missing import path (IMPLEMENTATION-PLAN §4.5).
func (s *stage) checkImportedGoRoots(u *unit, es *emitSite) {
	for _, imp := range u.p.Imports {
		dep := s.units[imp.Name]
		if dep == nil || dep.selected {
			continue
		}
		for _, e := range imp.Emits {
			if e.Target == TargetGo && e.GoImport == "" {
				u.report(diag.E8007.At(es.span(), e.Out))
			}
		}
	}
}

// checkMode runs the rules of the emit's mode.
func (s *stage) checkMode(u *unit, es *emitSite) {
	for _, rule := range modeRules[es.e.Mode] {
		rule(s, u, es)
	}
}

// checkContainers is E8015: a data or embedded emit's values are tables, keyed lists or records (CODEGEN.md §2.2).
func (s *stage) checkContainers(u *unit, es *emitSite) {
	for _, v := range selectedValues(u, es.e) {
		t := v.v.Type
		if t.Kind == types.Table || t.Kind == types.Record || t.Kind == types.List && t.KeyedBy != nil {
			continue
		}
		u.report(diag.E8015.At(v.span().span(), v.v.Name, v.t))
	}
}

// checkDataFns is E8013: data files hold no package fn, and no method table keyed by a ref (CODEGEN.md §5.10).
func (s *stage) checkDataFns(u *unit, _ *emitSite) {
	for _, site := range s.ownFns(u) {
		switch {
		case site.fn.Kind == FnTranslated:
		case site.recv == nil:
			u.report(diag.E8013.AtPackage(site.span(), site.label))
		case site.fn.Kind == FnLookup && slices.ContainsFunc(site.fn.Params, func(p *Param) bool { return p.Type.Kind == types.Ref }):
			u.report(diag.E8013.AtRefParam(site.span(), site.label))
		}
	}
}

// checkTypesMode is E8014: types mode has no data for stored fns or computed defaults (§5.13).
func (s *stage) checkTypesMode(u *unit, _ *emitSite) {
	for _, site := range s.ownFns(u) {
		switch site.fn.Kind {
		case FnPrecomputed:
			u.report(diag.E8014.At(site.span(), diag.KindPrecomputedFunction, site.label))
		case FnLookup:
			u.report(diag.E8014.At(site.span(), diag.KindLookupFunction, site.label))
		case FnTranslated:
		}
	}
	s.eachOwnField(u, func(owner string, f *Field) {
		if f.Computed {
			u.report(diag.E8014.At(s.fieldSites[f].span(), diag.KindComputedDefault, owner+qnameSep+f.Name))
		}
	})
}

// checkDecoders is E8018: a record or variant of another package held by value in what this emit decodes from JSON comes from an emit that provides decoders (CODEGEN.md §2.8).
func (s *stage) checkDecoders(u *unit, es *emitSite) {
	reported := map[Type]bool{}
	w := newWalker(func(n Type) bool {
		if pkgOf(n) == u.p.Name {
			return true
		}
		_, isEnum := n.(*Enum)
		if dep := s.units[pkgOf(n)]; !isEnum && dep != nil && !reported[n] && bakedFor(dep, es.e.Target) {
			reported[n] = true
			u.report(diag.E8018.At(es.span(), n.QName(), targetWords[es.e.Target], dep.p.Name))
		}
		return false
	}, func(*TypeRef) {})
	if es.e.Mode == ModeTypes {
		for _, t := range u.p.Types {
			w.named(t)
		}
		return
	}
	for _, v := range selectedValues(u, es.e) {
		w.ref(&v.v.Type)
	}
}

// bakedFor reports a package whose emit of target t is in baked mode.
func bakedFor(u *unit, t Target) bool {
	for _, es := range u.emits {
		if es.e.Target == t {
			return es.e.Mode == ModeBaked
		}
	}
	return false
}

// checkNoInputs is E8104: a package with input fields has no TypeScript emit (§5.12).
func (s *stage) checkNoInputs(u *unit, _ *emitSite) {
	s.eachOwnField(u, func(_ string, f *Field) {
		if f.Input != nil {
			u.report(diag.E8104.At(s.fieldSites[f].span(), f.Name))
		}
	})
}

// checkWireForms is E8151: a value written to JSON has a wire form (WIRE.md §5.9, §8.1).
func (s *stage) checkWireForms(u *unit, es *emitSite) {
	defines := bakedFor(u, TargetGo)
	for _, v := range selectedValues(u, es.e) {
		if noWireForm(v.t, map[types.Type]bool{}, defines) {
			u.report(diag.E8151.At(v.span().span(), v.v.Name, v.t))
		}
	}
}

// crossPackage is E8008: two Go emits of the build write into one directory (CODEGEN.md §2.3).
func (s *stage) crossPackage() {
	byDir := map[string]*unit{}
	for _, u := range s.order {
		if !u.selected {
			continue
		}
		for _, es := range u.emits {
			if es.e.Target != TargetGo || es.e.Dir == "" {
				continue
			}
			if first := byDir[es.e.Dir]; first != nil {
				u.report(diag.E8008.At(es.outSpan, es.display, first.p.Name, u.p.Name))
				continue
			}
			byDir[es.e.Dir] = u
		}
	}
}
