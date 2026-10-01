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
	code := []func(*stage, *unit, *emitSite){(*stage).checkImports, (*stage).checkMode, (*stage).checkGenSupport}
	emitRules[TargetGo], emitRules[TargetCpp] = code, code
	emitRules[TargetTS] = append(slices.Clone(code), (*stage).checkSafeInts, (*stage).checkNoInputs)
	emitRules[TargetJSON] = []func(*stage, *unit, *emitSite){(*stage).checkWireForms}
	modeRules[ModeEmbedded] = []func(*stage, *unit, *emitSite){(*stage).checkContainers, (*stage).checkDecoders, (*stage).checkFingerprinted}
	modeRules[ModeData] = []func(*stage, *unit, *emitSite){(*stage).checkContainers, (*stage).checkDecoders, (*stage).checkDataFns, (*stage).checkFingerprinted}
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
		s.checkGoNames(u)
		s.checkCppNames(u)
		s.translateFns(u)
	}
	for _, es := range u.emits {
		s.checkCopies(u, es)
		if es.index > 0 {
			continue // a further copy: the same options, judged once (CODEGEN.md §2.1)
		}
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

// selectedValues are the value sites an emit selects, in its order; none in types mode, which writes no value (CODEGEN.md §2.2).
func selectedValues(u *unit, e *Emit) []*valueSite {
	if e.Mode == ModeTypes {
		return nil
	}
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

// emittedValues are the value sites some code emit writes, in declaration order: selected (CODEGEN.md §2.1), in a mode that writes values (§2.2; selectedValues), whose mode was not refused (decision 213).
func emittedValues(u *unit) []*valueSite {
	written := map[*valueSite]bool{}
	for _, es := range u.emits {
		if !isCode(es.e.Target) || modeRefused(es.e) {
			continue
		}
		for _, v := range selectedValues(u, es.e) {
			written[v] = true
		}
	}
	var out []*valueSite
	for _, v := range u.values {
		if written[v] {
			out = append(out, v)
		}
	}
	return out
}

func (v *valueSite) span() declSite { return declSite{file: v.obj.File(), node: v.decl.Name} }

// checkImports is E8004 `noEmit`: a package whose types this code emit uses has an emit of its target (CODEGEN.md §2.8).
func (s *stage) checkImports(u *unit, es *emitSite) {
	for _, imp := range u.p.Imports {
		if countTarget(imp.Emits, es.e.Target) == 0 {
			u.report(diag.E8004.AtNoEmit(es.span(), u.firstUse[imp.Name], imp.Name, targetWords[es.e.Target]))
		}
	}
	if es.e.Target == TargetGo {
		s.checkImportedGoRoots(u, es)
	}
}

// checkImportedGoRoots is E8007 for an unselected dependency's own go emit whose output resolved under no go_module root: it never checked itself, so the importer reports it, whose build would else reference a missing import path (IMPLEMENTATION-PLAN §4.5). An out that is missing or does not resolve is not E8007: the dependency's own finding, reported when it is checked.
func (s *stage) checkImportedGoRoots(u *unit, es *emitSite) {
	for _, imp := range u.p.Imports {
		dep := s.units[imp.Name]
		if dep == nil || dep.selected {
			continue
		}
		for _, d := range dep.emits {
			if d.e.Target == TargetGo && d.unmapped {
				u.report(diag.E8007.At(es.span(), d.e.Out))
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
		if !dataContainer(v.v.Type) {
			u.report(diag.E8015.At(v.span().span(), v.v.Name, v.t))
		}
	}
}

// dataContainer reports a type data and embedded modes emit: a table, a keyed list or a record (CODEGEN.md §2.2).
func dataContainer(t TypeRef) bool {
	return t.Kind == types.Table || t.Kind == types.Record || t.Kind == types.List && t.KeyedBy != nil
}

// checkDataFns is E8013: data files hold no package fn and no method table keyed by a ref (CODEGEN.md §5.10).
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

// bakedGoSelects reports a value the package's baked go emit writes: that emit cannot hold a define record or table (decisions 180, 194, judged per emit).
func bakedGoSelects(u *unit, v *valueSite) bool {
	es := emitFor(u, TargetGo)
	return es != nil && es.e.Mode == ModeBaked && slices.Contains(selectedValues(u, es.e), v)
}

// bakedFor reports a package whose emit of target t is in baked mode.
func bakedFor(u *unit, t Target) bool {
	es := emitFor(u, t)
	return es != nil && es.e.Mode == ModeBaked
}

// checkNoInputs is E8104: a package with input fields has no TypeScript emit (§5.12).
func (s *stage) checkNoInputs(u *unit, _ *emitSite) {
	s.eachOwnField(u, func(_ string, f *Field) {
		if f.Input != nil {
			u.report(diag.E8104.At(s.fieldSites[f].span(), f.Name))
		}
	})
}

// checkWireForms is E8151: a value written to JSON has a wire form and a fingerprint, so no define record (WIRE.md §5.9, §8.1; decisions 126, 194); the `define` variant reports a load.defines table or record specifically, the general one a Range or a function.
func (s *stage) checkWireForms(u *unit, es *emitSite) {
	for _, v := range selectedValues(u, es.e) {
		what := wireFind(v.t, map[types.Type]bool{}, noWire)
		switch {
		case what == nil:
		case isDefineType(what.Base()):
			u.report(diag.E8151.AtDefine(v.span().span(), v.v.Name))
		default:
			u.report(diag.E8151.AtType(v.span().span(), v.v.Name, v.t))
		}
	}
}

// checkFingerprinted is E8012 `define` for a data or embedded code emit: each value it selects carries its fingerprint, which a define record has none of (decisions 126, 194); a value a baked go emit selects, and so already refuses as a whole (checkRepresentable), is not reported twice.
func (s *stage) checkFingerprinted(u *unit, es *emitSite) {
	for _, v := range selectedValues(u, es.e) {
		if bakedGoSelects(u, v) && unrepresentable(v.t, false, true) != nil {
			continue
		}
		if wireFind(v.t, map[types.Type]bool{}, isDefineType) != nil {
			u.report(diag.E8012.AtDefine(v.span().span(), v.v.Name))
		}
	}
}

// crossPackage is E8008, two different Go emits of the build writing into one directory, two copies of one emit there being E8009 `outRoot` (CODEGEN.md §2.3), and E8005 for two packages declaring one name in a C++ namespace they share (§3.5).
func (s *stage) crossPackage() {
	s.sharedCppNames()
	type placed struct {
		u  *unit
		es *emitSite
	}
	byDir := map[string]placed{}
	for _, u := range s.order {
		if !u.selected {
			continue
		}
		for _, es := range u.emits {
			if es.e.Target != TargetGo || es.e.Dir == "" {
				continue
			}
			first, taken := byDir[es.e.Dir]
			switch {
			case !taken:
				byDir[es.e.Dir] = placed{u, es}
			case first.es.decl != es.decl:
				u.report(diag.E8008.At(es.outSpan, es.display, first.u.p.Name, u.p.Name))
			}
		}
	}
}
