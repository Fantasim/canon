package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

// CopyOf is p as its emit copy e sees it, each import's emits of e's target narrowed to the copy e uses (p itself when none narrows): every Generator is called on it (CODEGEN.md §2.8, DECISIONS 229).
func CopyOf(proj *project.Project, p *Package, e *Emit) *Package {
	var refs []*PackageRef
	for i, ref := range p.Imports {
		use := copyFor(proj, ref.Emits, e)
		if use == nil || countTarget(ref.Emits, e.Target) < severalCopies {
			continue
		}
		if refs == nil {
			refs = slices.Clone(p.Imports)
		}
		kept := slices.DeleteFunc(slices.Clone(ref.Emits), func(o *Emit) bool { return o.Target == e.Target && o != use })
		refs[i] = &PackageRef{Name: ref.Name, Dir: ref.Dir, Emits: kept}
	}
	if refs == nil {
		return p
	}
	view := *p
	view.Imports = refs
	return &view
}

// copyFor is the copy of emits e uses, the only one of its target or the one under its owning root; nil when none fits (CODEGEN.md §2.8).
func copyFor(proj *project.Project, emits []*Emit, e *Emit) *Emit {
	if countTarget(emits, e.Target) == 1 {
		return emits[slices.IndexFunc(emits, func(o *Emit) bool { return o.Target == e.Target })]
	}
	owner := check.OwningRoot(proj, e.Dir)
	for _, o := range emits {
		if o.Target == e.Target && o.Dir != "" && check.OwningRoot(proj, o.Dir) == owner {
			return o
		}
	}
	return nil
}

// countTarget is the number of emits of target t: the copies of a package's one emit of t.
func countTarget(emits []*Emit, t Target) int {
	n := 0
	for _, o := range emits {
		if o.Target == t {
			n++
		}
	}
	return n
}

// checkCopies is E8004 `noCopy`: a placed copy of a code emit finds a copy it can use in each import with several (CODEGEN.md §2.8, DECISIONS 229, 269).
func (s *stage) checkCopies(u *unit, es *emitSite) {
	if !isCode(es.e.Target) || es.e.Dir == "" {
		return
	}
	proj := s.in.Project
	for _, imp := range u.p.Imports {
		first, used := u.importUse(imp.Name, es.e.Target)
		if !used || countTarget(imp.Emits, es.e.Target) < severalCopies || copyFor(proj, imp.Emits, es.e) != nil {
			continue
		}
		root := check.RootLabel(proj, check.OwningRoot(proj, es.e.Dir))
		u.report(diag.E8004.AtNoCopy(es.outSpan, first, imp.Name, targetWords[es.e.Target], root))
	}
}
