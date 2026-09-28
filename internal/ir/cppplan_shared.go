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
	reported := map[originPair]bool{}
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
			pair := originPair{f.origin, n.origin}
			if f.u != u && (f.meets == meetsNever || f.meets != n.meets) && !reported[pair] {
				reported[pair] = true
				reportShared(f, cppHolder{n, u})
			}
		}
	}
	s.hiddenCppNames(first, inner, reported)
}

// hiddenCppNames is E8005 for a detail or conformance name of one package equal to a namespace name of another package sharing the namespace: inside detail it hides that name (log-2026-09-24 "Owed (C++ plan)": b's detail::BAccess vs a's record BAccess), reported once per pair as sharedCppNames does.
func (s *stage) hiddenCppNames(first map[cppSharedKey]cppHolder, inner []cppHolder, reported map[originPair]bool) {
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
		if pair := (originPair{f.origin, h.origin}); !reported[pair] {
			reported[pair] = true
			reportShared(f, h)
		}
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
func reportShared(first, later cppHolder) {
	at := later.u
	if !at.selected {
		at = first.u
	}
	if es := emitFor(at, TargetCpp); at.selected && es != nil {
		at.report(diag.E8005.At(es.span(), check.TargetCpp, later.name, first.origin, later.origin))
	}
}

// cppNamesOf are the shared names of u's data- or types-mode cpp emit: its plan's when u is selected (checkCppNames), else those of the plan of its declarations, values unevaluated.
func (s *stage) cppNamesOf(u *unit) []cppShared {
	es := emitFor(u, TargetCpp)
	switch {
	case es == nil || es.e.Mode != ModeData && es.e.Mode != ModeTypes:
		return nil
	case u.selected:
		return u.cppNames
	}
	return PlanCppNames(s.declaredOnly(u), es.e).shared
}

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
	if !ok || d.Mods == nil || !d.Mods.Export.Valid() {
		return fns
	}
	n := nameOverrides(d.Annotations)
	return append(fns, &ExportFn{Name: obj.Name(), Kind: kindOf(sig), Go: n.goName, Cpp: n.cpp, TS: n.ts})
}
