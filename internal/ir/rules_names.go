package ir

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// checkOverrideNames is E8011 for @cpp and @ts(name:): an override its emitted target cannot declare, at every position that takes one, enum members and cases included (CODEGEN.md §3.5); the go target's come from the name plan (checkGoNames).
func (s *stage) checkOverrideNames(u *unit) {
	cpp, ts := hasTarget(u, TargetCpp), hasTarget(u, TargetTS)
	for _, site := range overrideSites(u.p).sites {
		if ok, _ := cppValidIdent(site.cpp); cpp && site.cpp != "" && !ok {
			u.report(diag.E8011.AtOverride(s.itemSpan(site.item, source.Span{}), site.cpp, check.TargetCpp))
		}
		if ts && site.tsName != "" && !identPattern.MatchString(site.tsName) {
			u.report(diag.E8011.AtOverride(s.itemSpan(site.item, source.Span{}), site.tsName, check.TargetTS))
		}
	}
}

// checkGoNames reports the go emit's names (CODEGEN.md §3.5): E8011 for a @go(name:) that is not an exported identifier (decision 182), in every mode; in baked and data mode, from the name plan gen/go writes from (decisions 194, 203), E8011 for a derived name that is no Go identifier (decision 202) and E8005 for two names of one scope, at the item named second, or at the emit for an import or the package's own names. Embedded and types mode have no generator, hence no plan yet.
func (s *stage) checkGoNames(u *unit) {
	es := emitFor(u, TargetGo)
	if es == nil {
		return
	}
	problems := goOverrideProblems(u.p)
	if es.e.Mode == ModeBaked || es.e.Mode == ModeData {
		problems = slices.DeleteFunc(PlanGoNames(u.p, es.e).Problems(), s.refusedImport(u))
	}
	reportNames(u, s.itemSpans(es), problems, check.TargetGo, map[any]bool{})
}

// refusedImport reports the import name of a dependency whose go package check refused at that emit (E8009), written or defaulted from out (decisions 213, 215), matched by its import path, never by the name's text: the importer adds nothing to it.
func (s *stage) refusedImport(u *unit) func(GoNameProblem) bool {
	refused := map[string]bool{}
	for _, imp := range u.p.Imports {
		if dep := s.units[imp.Name]; dep != nil {
			if es := emitFor(dep, TargetGo); es != nil && es.refused {
				refused[es.e.GoImport] = true
			}
		}
	}
	return func(pr GoNameProblem) bool { return pr.Kind == GoNotIdentifier && pr.Item == nil && refused[pr.Origin] }
}

// checkCppNames reports a data-mode cpp emit's names from the name plan gen/cpp writes from (CODEGEN.md §3.5, decision 37): E8011 for a derived name C++ cannot declare, a keyword or a reserved namespace included (decision 202), E8005 for two names of one scope; the names its header shares with the other packages of its namespace wait for crossPackage. Other modes have no generator yet; overrides are checkOverrideNames'.
func (s *stage) checkCppNames(u *unit) {
	es := emitFor(u, TargetCpp)
	if es == nil || es.e.Mode != ModeData {
		return
	}
	pl := PlanCppNames(u.p, es.e)
	u.cppNames = pl.shared
	refused := map[any]bool{}
	for _, site := range overrideSites(u.p).sites {
		ok, _ := cppValidIdent(site.cpp)
		refused[site.item] = site.cpp != "" && !ok // checkOverrideNames reported it
	}
	reportNames(u, s.itemSpans(es), pl.Problems(), check.TargetCpp, refused)
}

// itemSpans locates a plan's item, at the emit for an import or the package's own names.
func (s *stage) itemSpans(es *emitSite) func(any) source.Span {
	return func(item any) source.Span { return s.itemSpan(item, es.span()) }
}

// reportNames reports a plan's problems for target: E8005 for a collision, E8011 once per declaration not refused yet (decisions 202, 218).
func reportNames(u *unit, span func(any) source.Span, problems []GoNameProblem, target string, refused map[any]bool) {
	for _, pr := range problems {
		if pr.Kind == GoCollision {
			u.report(diag.E8005.At(span(pr.Item), target, pr.Name, pr.First, pr.Origin))
			continue
		}
		if pr.Item != nil && refused[pr.Item] {
			continue
		}
		refused[pr.Item] = true
		u.report(nameProblemFinding(span(pr.Item), pr, target))
	}
}

// nameProblemFinding is the E8011 variant a non-collision GoNameProblem reports (decisions 182, 202, 218).
func nameProblemFinding(span source.Span, pr GoNameProblem, target string) *diag.Builder {
	name := shownName(pr)
	switch {
	case pr.Kind == GoOverrideInvalid:
		return diag.E8011.AtOverride(span, name, target)
	case pr.Kind == GoUnexported:
		return diag.E8011.AtUnexported(span, name)
	case pr.Reserved:
		return diag.E8011.AtReserved(span, name, pr.Origin)
	default:
		return diag.E8011.AtDerived(span, name, pr.Origin, target)
	}
}

// shownName is the name an E8011 shows: the derived or override name, or the source identifier when the derived name is empty (`__`), since a message never shows a blank name (meta/decisions/log-2026-09-24.md, IR round 2 review).
func shownName(pr GoNameProblem) string {
	if pr.Name != "" {
		return pr.Name
	}
	return pr.Origin[strings.LastIndex(pr.Origin, qnameSep)+1:]
}

// emitFor is the package's emit of target t, or nil (CODEGEN.md §2.1: at most one per target).
func emitFor(u *unit, t Target) *emitSite {
	for _, es := range u.emits {
		if es.e.Target == t {
			return es
		}
	}
	return nil
}

// itemSpan locates an IR node of this package: a type, field or export fn at its declaration, an enum member, case, parameter, constant or value at its name; fallback for anything else.
func (s *stage) itemSpan(item any, fallback source.Span) source.Span {
	var d declSite
	switch x := item.(type) {
	case Type:
		d = s.decls[x]
	case *Field:
		if site := s.fieldSites[x]; site != nil {
			d = site.declSite
		}
	case *ExportFn:
		if site := s.fnObjs[x]; site != nil {
			return site.span()
		}
	default:
		d = s.nodeSites[item]
	}
	if d.file == nil {
		return fallback
	}
	return d.span()
}
