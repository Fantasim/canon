package ir

import (
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// cppSharedKey is a name in one C++ namespace: the emit's, or its detail or conformance one.
type cppSharedKey struct{ scope, name string }

// cppHolder is a shared name with the package that declares it.
type cppHolder struct {
	cppShared
	u *unit
}

// sharedCppNames is E8005 for a name the cpp headers of two packages the build loads declare in one namespace, selected or imported (CODEGEN.md §3.5: a C++ namespace holds all packages emitted into it that the build loads; log-2026-09-24 "ir plans review"), once per pair of items, at the later package's emit when it is selected, else at the earlier's; overloads of each other (ToName, ToWire, Decode), names local to each package's sources, and a namespace opened twice meet legally.
func (s *stage) sharedCppNames() {
	first := map[cppSharedKey]cppHolder{}
	found := sharedFound{at: map[originPair]int{}}
	var inner []cppHolder
	for _, u := range s.order {
		for _, n := range s.cppNamesOf(u) {
			k := cppSharedKey{n.scope, n.name}
			if innerScope(n.scope) != "" {
				inner = append(inner, cppHolder{n, u})
			}
			f, seen := first[k]
			if !seen {
				first[k] = cppHolder{n, u}
				continue
			}
			if f.u != u && (f.meets == meetsNever || f.meets != n.meets) {
				found.add(f, cppHolder{n, u})
			}
		}
	}
	s.hiddenCppNames(first, inner, &found)
	found.report()
}

// sharedFound are the names two packages share in one namespace, one per pair of items, in the order met: a pair that also meets in a name no single @cpp(name:) renames without the other is reported at that one, as `derived` (DECISIONS 203, 442, 458).
type sharedFound struct {
	list []sharedFinding
	at   map[originPair]int
}

// sharedFinding is two holders of one name, derived when both names are built on Canon names that @cpp(name:) does not change, under one owner.
type sharedFinding struct {
	first, later cppHolder
	derived      bool
}

// add keeps the meeting of first and later, unless their pair has one already that is as good.
func (f *sharedFound) add(first, later cppHolder) {
	pair := originPair{first.origin, later.origin}
	found := sharedFinding{first, later, first.note.derived && later.note.derived && first.note.owner == later.note.owner}
	at, seen := f.at[pair]
	switch {
	case !seen:
		f.at[pair] = len(f.list)
		f.list = append(f.list, found)
	case found.derived && !f.list[at].derived:
		f.list[at] = found
	}
}

// report reports each meeting at the later package when selected, else at the earlier.
func (f *sharedFound) report() {
	for _, m := range f.list {
		reportShared(m)
	}
}

// hiddenCppNames is E8005 for a detail or conformance name of one package equal to a namespace name of another package sharing the namespace: inside detail it hides that name (log-2026-09-24 "Owed (C++ plan)": b's detail::BAccess vs a's record BAccess), reported once per pair as sharedCppNames does.
func (s *stage) hiddenCppNames(first map[cppSharedKey]cppHolder, inner []cppHolder, found *sharedFound) {
	index := map[*unit]int{}
	for i, u := range s.order {
		index[u] = i
	}
	for _, h := range inner {
		f, seen := first[cppSharedKey{innerScope(h.scope), h.name}]
		if !seen || f.u == h.u {
			continue
		}
		if index[h.u] < index[f.u] {
			f, h = h, f
		}
		found.add(f, h)
	}
}

// innerScope is the namespace holding scope when scope is its detail or conformance namespace, else "".
func innerScope(scope string) string {
	for _, inner := range []string{cppDetail, cppConformance} {
		if parent, ok := strings.CutSuffix(scope, cppScope+inner); ok {
			return parent
		}
	}
	return ""
}

// reportShared reports two packages' one name at the later one when it is selected, else at the earlier one, when selected.
func reportShared(m sharedFinding) {
	at := m.later.u
	if !at.selected {
		at = m.first.u
	}
	es := emitFor(at, TargetCpp)
	if !at.selected || es == nil {
		return
	}
	if m.derived {
		at.report(diag.E8005.AtDerived(es.span(), check.TargetCpp, m.later.name, m.first.origin, m.later.origin))
		return
	}
	at.report(diag.E8005.AtBoth(es.span(), check.TargetCpp, m.later.name, m.first.origin, m.later.origin))
}

// cppNamesOf are the shared names of u's baked, data- or types-mode cpp emit: its plan's when u is selected (checkCppNames), else those of the plan of its declarations, values unevaluated.
func (s *stage) cppNamesOf(u *unit) []cppShared {
	es := emitFor(u, TargetCpp)
	switch {
	case es == nil || !cppPlanned(es.e) && !refusedCppMode(es.e):
		return nil
	case u.selected:
		return u.cppNames
	case refusedCppMode(es.e): // what every mode's plan shares (DECISIONS 429)
		return commonShared(cppPlansOfEveryMode(s.declaredOnly(u), es.e))
	}
	return PlanCppNames(s.declaredOnly(u), es.e).shared
}

// cppPlanned reports a cpp emit in a mode gen/cpp writes, whose names the plan holds: baked, data or types (embedded is refused, CODEGEN.md §2.2).
func cppPlanned(e *Emit) bool { return plannedMode(e.Mode) }

// plannedMode reports a mode gen/cpp writes names for: baked, data or types.
func plannedMode(m Mode) bool { return m == ModeBaked || m == ModeData || m == ModeTypes }

// declaredOnly is an unselected package's public IR as declared, broken declarations left out (decision 213), with no value evaluated: what its C++ names need.
func (s *stage) declaredOnly(u *unit) *Package {
	p := &Package{Name: u.p.Name, Dir: u.p.Dir, Emits: u.p.Emits}
	for _, obj := range u.cp.Decls {
		if s.info.Broken[obj] {
			continue
		}
		switch d := obj.Decl().(type) {
		case *syntax.RecordDecl, *syntax.EnumDecl, *syntax.VariantDecl, *syntax.TypeDecl:
			if t := s.publicType(obj); t != nil {
				p.Types = append(p.Types, t)
			}
		case *syntax.ConstDecl:
			if !local(d.Mods) {
				p.Consts = append(p.Consts, s.constDecl(obj, d))
			}
		case *syntax.LetDecl:
			if !local(d.Mods) {
				p.Values = append(p.Values, s.letDecl(obj, d))
			}
		case *syntax.FnDecl:
			p.Fns = appendDeclaredFn(p.Fns, obj, d)
		}
	}
	return p
}

// appendDeclaredFn adds an export fn's name, kind and overrides.
func appendDeclaredFn(fns []*ExportFn, obj check.Object, d *syntax.FnDecl) []*ExportFn {
	sig, ok := obj.Type().(*types.FuncType)
	if !ok || !exported(d) {
		return fns
	}
	n := nameOverrides(d.Annotations)
	return append(fns, &ExportFn{Name: obj.Name(), Kind: kindOf(sig), Go: n.goName, Cpp: n.cpp, TS: n.ts})
}
