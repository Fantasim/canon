package ir

import (
	"path"
	"slices"
	"strings"

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

// climbsAbove reports that the relative path from directory from to directory to climbs above the project's parent directories: once their common leading segments go, what remains of from holds a `..`, so the path would come back down through directories above the project, whose names depend on the checkout (CODEGEN.md §2.8, DECISIONS 332). Both are cleaned project-relative directories.
func climbsAbove(from, to string) bool {
	f, t := dirSegments(from), dirSegments(to)
	i := 0
	for i < len(f) && i < len(t) && f[i] == t[i] {
		i++
	}
	return slices.Contains(f[i:], parentDir)
}

// dirSegments are the segments of a cleaned directory, none for the project directory itself.
func dirSegments(dir string) []string {
	dir = path.Clean(dir)
	if dir == curDir {
		return nil
	}
	return strings.Split(dir, pathSep)
}

// checkAboveProject is E8025: a cpp or ts copy that reaches the copy of an imported package it uses by a relative path climbing above the project (CODEGEN.md §2.8, DECISIONS 332), whatever machine builds it. A copy under no usable root is E8004's.
func (s *stage) checkAboveProject(u *unit, es *emitSite) {
	if es.e.Target != TargetCpp && es.e.Target != TargetTS || es.e.Dir == "" {
		return
	}
	for _, imp := range u.p.Imports {
		if _, used := u.importUse(imp.Name, es.e.Target); !used {
			continue
		}
		o := copyFor(s.in.Project, imp.Emits, es.e)
		if o == nil || o.Dir == "" || !climbsAbove(es.e.Dir, o.Dir) {
			continue
		}
		u.report(diag.E8025.At(es.outSpan, es.display, s.displayOf(imp.Name, o), imp.Name))
	}
}

// displayOf is the display path of the out of emit o of package pkg, its out as written when its site is unknown.
func (s *stage) displayOf(pkg string, o *Emit) string {
	if dep := s.units[pkg]; dep != nil {
		for _, es := range dep.emits {
			if es.e == o {
				return es.display
			}
		}
	}
	return o.Out
}
