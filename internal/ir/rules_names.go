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
		if ts && site.tsName != "" && !tsValidOverride(site) {
			u.report(diag.E8011.AtOverride(s.itemSpan(site.item, source.Span{}), site.tsName, check.TargetTS))
		}
	}
}

// tsValidOverride reports a @ts(name:) override TypeScript can declare: an identifier that is no reserved word, and for a type no predefined type name (CODEGEN.md §3.4, §3.5).
func tsValidOverride(site nameSite) bool {
	if !identPattern.MatchString(site.tsName) || slices.Contains(tsReserved, site.tsName) {
		return false
	}
	_, isType := site.item.(Type)
	return !isType || !slices.Contains(tsPredefined, site.tsName)
}

// checkTSNames reports the ts emit's names from the name plan gen/ts writes from (CODEGEN.md §3.5, DECISIONS 278): E8005 for two names of one scope, a helper's included; for a mode check refused, the collisions of every mode (decision 213, log-2026-11-01).
func (s *stage) checkTSNames(u *unit) {
	es := emitFor(u, TargetTS)
	if es == nil {
		return
	}
	report := nameReport{s.itemSpans(es), check.TargetTS, tsIDName}
	if !modeRefused(es.e) {
		reportNames(u, report, planTSNames(u.p, es.e).found(), map[any]bool{})
		return
	}
	plans := make([][]GoNameProblem, 0, len(tsModes))
	for _, m := range tsModes {
		plans = append(plans, planTSNames(u.p, inMode(es.e, m)).found())
	}
	reportNames(u, report, inEveryMode(plans), map[any]bool{})
}

// inMode is a copy of e in mode m.
func inMode(e *Emit, m Mode) *Emit {
	c := *e
	c.Mode = m
	return &c
}

// problemKey is what tells one collision from another across the plans of several modes.
type problemKey struct {
	scope, name, first, origin string
}

// inEveryMode are the collisions of plans[0] that every plan has: what a refused mode cannot change (decision 213).
func inEveryMode(plans [][]GoNameProblem) []GoNameProblem {
	keyOf := func(pr GoNameProblem) problemKey { return problemKey{pr.Scope, pr.Name, pr.First, pr.Origin} }
	out := slices.DeleteFunc(slices.Clone(plans[0]), func(pr GoNameProblem) bool { return pr.Kind != GoCollision })
	for _, other := range plans[1:] {
		has := map[problemKey]bool{}
		for _, pr := range other {
			has[keyOf(pr)] = pr.Kind == GoCollision
		}
		out = slices.DeleteFunc(out, func(pr GoNameProblem) bool { return !has[keyOf(pr)] })
	}
	return out
}

// checkGoNames reports the go emit's names (CODEGEN.md §3.5): E8011 for a @go(name:) that is not an exported identifier (decision 182), in every mode; in a mode gen/go writes (baked, data, types), from the name plan it writes from (decisions 194, 203), E8011 for a derived name that is no Go identifier (decision 202) and E8005 for two names of one scope, at the item named second, or at the emit for an import or the package's own names. Embedded mode has no generator, hence no plan yet.
func (s *stage) checkGoNames(u *unit) {
	es := emitFor(u, TargetGo)
	if es == nil {
		return
	}
	problems := goOverrideProblems(u.p)
	report := nameReport{span: s.itemSpans(es), target: check.TargetGo}
	if !modeRefused(es.e) {
		pl := PlanGoNames(s.view(u, es), es.e)
		problems = slices.DeleteFunc(pl.Problems(), s.refusedImport(u))
		report.idOf = func(rec *Record) string { return pl.IDTypeName(rec) }
	} else if !unbuiltMode(es.e) { // an unbuilt mode is E8019's alone (checkUnbuilt)
		problems = append(problems, s.goCollisionsOfEveryMode(u, es)...)
	}
	reportNames(u, report, problems, map[any]bool{})
}

// checkCppNamesRefused judges a cpp emit whose mode check refused over what the plan of every mode gen/cpp writes has: its collisions, the names its namespace shares, the #defines of packages it does not read (DECISIONS 429); an unbuilt mode is E8019's alone, W8006 one plan's.
func (s *stage) checkCppNamesRefused(u *unit, es *emitSite, refused map[any]bool) {
	if !refusedCppMode(es.e) {
		return
	}
	plans := cppPlansOfEveryMode(u.p, es.e)
	problems := make([][]GoNameProblem, 0, len(plans))
	for _, pl := range plans {
		problems = append(problems, pl.Problems())
	}
	reportNames(u, nameReport{s.itemSpans(es), check.TargetCpp, plans[0].IDName}, inEveryMode(problems), refused)
	u.cppNames = commonShared(plans)
	s.foreignDefinesOf(u, es, s.commonDeclared(plans))
}

// goCollisionsOfEveryMode are the collisions the go plan of every mode gen/go writes has, for an emit whose mode was refused (decision 213).
func (s *stage) goCollisionsOfEveryMode(u *unit, es *emitSite) []GoNameProblem {
	plans := make([][]GoNameProblem, 0, len(goModes))
	for _, m := range goModes {
		e := inMode(es.e, m)
		plans = append(plans, slices.DeleteFunc(PlanGoNames(CopyOf(s.in.Project, u.p, e), e).Problems(), s.refusedImport(u)))
	}
	return inEveryMode(plans)
}

// refusedImport reports the import name of a dependency whose go package check refused at that emit (E8009): written, defaulted from out, or none for an emit with no out or one that does not resolve (decisions 213, 215), matched by its import path, never by the name's text: the importer adds nothing to it.
func (s *stage) refusedImport(u *unit) func(GoNameProblem) bool {
	refused := map[string]bool{}
	for _, imp := range u.p.Imports {
		dep := s.units[imp.Name]
		if dep == nil {
			continue
		}
		for _, es := range dep.emits {
			if es.e.Target == TargetGo && es.refused {
				refused[es.e.GoImport] = true // every copy (DECISIONS 229)
			}
		}
	}
	return func(pr GoNameProblem) bool { return pr.Kind == GoNotIdentifier && pr.Item == nil && refused[pr.Origin] }
}

// checkCppNames reports a baked, data- or types-mode cpp emit's names from the name plan gen/cpp writes from (CODEGEN.md §3.5, decision 37): E8011 for a derived name C++ cannot declare, a keyword or a reserved namespace included (decision 202), E8005 for two names of one scope, or equal to a #define of a package it does not read (checkForeignDefines); the names its header shares with the other packages of its namespace wait for crossPackage. Embedded has no generator yet; overrides are checkOverrideNames'.
func (s *stage) checkCppNames(u *unit) {
	es := emitFor(u, TargetCpp)
	if es == nil {
		return
	}
	refused := map[any]bool{}
	for _, site := range overrideSites(u.p).sites {
		ok, _ := cppValidIdent(site.cpp)
		refused[site.item] = site.cpp != "" && !ok // checkOverrideNames reported it
	}
	if !cppPlanned(es.e) {
		s.checkCppNamesRefused(u, es, refused)
		return
	}
	pl := PlanCppNames(u.p, es.e)
	u.cppNames = pl.shared
	reportNames(u, nameReport{s.itemSpans(es), check.TargetCpp, pl.IDName}, pl.Problems(), refused)
	for _, w := range pl.warnings { // W8006: never a problem, generation proceeds
		u.report(diag.W8006.At(s.itemSpan(w.Item, es.span()), w.Name, w.Origin))
	}
	s.checkForeignDefines(u, es, pl)
}

// itemSpans locates a plan's item, at the emit for an import or the package's own names.
func (s *stage) itemSpans(es *emitSite) func(any) source.Span {
	return func(item any) source.Span { return s.itemSpan(item, es.span()) }
}

// nameReport is how one target's plan problems are reported: where an item is, the target's word, and the id type it gives a table's record (nil where no plan names one).
type nameReport struct {
	span   func(any) source.Span
	target string
	idOf   func(*Record) string
}

// reportNames reports a plan's problems for a target: E8005 for a collision, E8011 once per declaration not refused yet (decisions 202, 218).
func reportNames(u *unit, r nameReport, problems []GoNameProblem, refused map[any]bool) {
	for _, pr := range problems {
		if pr.Kind == GoCollision {
			u.report(r.collisionFinding(pr))
			continue
		}
		if pr.Item != nil && refused[pr.Item] {
			continue
		}
		refused[pr.Item] = true
		u.report(nameProblemFinding(r.span(pr.Item), pr, r.target))
	}
}

// collisionFinding is E8005 with the way out its pair has (DECISIONS 305): `reached` for two packages a go emit reaches giving it one reader or rt name, a distinct package option on one of their emits; `idType` for two table holders whose records' id types are the colliding name, one held in a keyed list (log-2026-10-06 "Own table Badge beside table base.Badge", "U5 review FAIL" 4); `both` otherwise.
func (r nameReport) collisionFinding(pr GoNameProblem) *diag.Builder {
	span := r.span(pr.Item)
	switch {
	case r.target == check.TargetGo && reachedPair(pr):
		return diag.E8005.AtReached(span, r.target, pr.Name, pr.First, pr.Origin)
	case strings.HasPrefix(pr.Scope, tsContainerScope): // two @stable fields' lookups (DECISIONS 369)
		return diag.E8005.AtFinder(span, r.target, pr.Name, pr.First, pr.Origin)
	case pr.Item != pr.FirstItem && r.isIDType(pr.Name, pr.Item) && r.isIDType(pr.Name, pr.FirstItem):
		return diag.E8005.AtIdType(span, r.target, pr.Name, pr.First, pr.Origin)
	}
	return diag.E8005.AtBoth(span, r.target, pr.Name, pr.First, pr.Origin)
}

// isIDType reports name the id type of a table item holds: a table value's record, or a table a field holds, at any depth.
func (r nameReport) isIDType(name string, item any) bool {
	if r.idOf == nil {
		return false
	}
	recs := heldTableRecords(item)
	return slices.ContainsFunc(recs, func(rec *Record) bool { return r.idOf(rec) == name })
}

// reachedPair reports a collision where an import's Go package name is one of the names: a distinct package option on that import's emit is the way out (CODEGEN.md §2.8, §3.5; DECISIONS 305).
func reachedPair(pr GoNameProblem) bool {
	_, item := pr.Item.(*PackageRef)
	_, first := pr.FirstItem.(*PackageRef)
	return item || first
}

// heldTableRecords are the records the tables of a table value or a field hold (CODEGEN.md §5.3, §5.9).
func heldTableRecords(item any) []*Record {
	var t TypeRef
	switch x := item.(type) {
	case *Value:
		t = x.Type
	case *Field:
		t = x.Type
	default:
		return nil
	}
	var out []*Record
	walkTypeRef(t, func(sub TypeRef) {
		if rec, ok := tableElem(sub); ok {
			out = append(out, rec)
		}
	})
	return out
}

// nameProblemFinding is the E8011 variant a non-collision GoNameProblem reports (decisions 182, 202, 218).
func nameProblemFinding(span source.Span, pr GoNameProblem, target string) *diag.Builder {
	name := shownName(pr)
	switch {
	case pr.Kind == GoOverrideInvalid:
		return diag.E8011.AtOverride(span, name, target)
	case pr.Kind == GoUnexported:
		return diag.E8011.AtUnexported(span, name)
	case pr.Reserved && cppStdMacros[name]: // before reserved: the more precise variant
		return diag.E8011.AtMacro(span, name, pr.Origin)
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
