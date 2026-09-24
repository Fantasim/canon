package ir

import (
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// checkOverrideNames is E8011 for @cpp and @ts(name:): an override its emitted target cannot declare, at every position that takes one, enum members and cases included (CODEGEN.md §3.5); the go target's come from the name plan (checkGoNames).
func (s *stage) checkOverrideNames(u *unit) {
	cpp, ts := hasTarget(u, TargetCpp), hasTarget(u, TargetTS)
	for _, site := range overrideSites(u.p).sites {
		if cpp && site.cpp != "" && !cppValidIdent(site.cpp) {
			u.report(diag.E8011.At(s.itemSpan(site.item, source.Span{}), site.cpp, check.TargetCpp))
		}
		if ts && site.tsName != "" && !identPattern.MatchString(site.tsName) {
			u.report(diag.E8011.At(s.itemSpan(site.item, source.Span{}), site.tsName, check.TargetTS))
		}
	}
}

// checkGoNames reports the go emit's names (CODEGEN.md §3.5): E8011 for a @go(name:) that is not an exported identifier (decision 182), in every mode; in baked mode, from the name plan gen/go writes from (decision 194), E8011 for a derived name that is no Go identifier (decision 202) and E8005 for two names of one scope, at the item named second, or at the emit for an import or the package's own names. The other modes have no plan until M2 (decision 203); cpp and ts have no backend yet to plan their names.
func (s *stage) checkGoNames(u *unit) {
	es := emitFor(u, TargetGo)
	if es == nil {
		return
	}
	problems := goOverrideProblems(u.p)
	if es.e.Mode == ModeBaked {
		problems = PlanGoNames(u.p, es.e).Problems()
	}
	refused := map[any]bool{}
	for _, pr := range problems {
		span := s.itemSpan(pr.Item, es.span())
		if pr.Kind == GoCollision {
			u.report(diag.E8005.At(span, check.TargetGo, pr.Name, pr.First, pr.Origin))
			continue
		}
		if pr.Item != nil && refused[pr.Item] {
			continue // one E8011 per declaration: its override, else its first derived name
		}
		refused[pr.Item] = true
		u.report(diag.E8011.At(span, shownName(pr), check.TargetGo))
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
