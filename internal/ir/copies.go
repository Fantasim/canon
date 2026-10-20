package ir

import (
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

// CopyOf is p as its emit copy e sees it, each import's emits of e's target narrowed to the copy e uses, none when e cannot use an import's only copy (another consumer root's, DECISIONS 347); p itself when none narrows. Every Generator is called on it (CODEGEN.md §2.8, DECISIONS 229).
func CopyOf(proj *project.Project, p *Package, e *Emit) *Package {
	var refs []*PackageRef
	for i, ref := range p.Imports {
		use, n := copyFor(proj, ref.Emits, e), countTarget(ref.Emits, e.Target)
		if n == 0 || use != nil && n < severalCopies || use == nil && n >= severalCopies {
			continue
		}
		if refs == nil {
			refs = slices.Clone(p.Imports)
		}
		kept := slices.DeleteFunc(slices.Clone(ref.Emits), func(o *Emit) bool { return o.Target == e.Target && o != use })
		refs[i] = &PackageRef{Name: ref.Name, Dir: ref.Dir, Emits: kept, CppDefines: ref.CppDefines}
	}
	if refs == nil {
		return p
	}
	view := *p
	view.Imports = refs
	return &view
}

// copyFor is the copy of emits e uses: the only one of its target unless it lies under a consumer root other than e's, or the one under e's owning root; nil when none fits (CODEGEN.md §2.8, DECISIONS 229, 347).
func copyFor(proj *project.Project, emits []*Emit, e *Emit) *Emit {
	if countTarget(emits, e.Target) == 1 {
		if only := onlyCopy(emits, e.Target); otherConsumer(proj, only, e) == "" {
			return only
		}
		return nil
	}
	owner := check.OwningRoot(proj, e.Dir)
	for _, o := range emits {
		if o.Target == e.Target && o.Dir != "" && check.OwningRoot(proj, o.Dir) == owner {
			return o
		}
	}
	return nil
}

// onlyCopy is the first emit of target t in emits: its only copy when it has one.
func onlyCopy(emits []*Emit, t Target) *Emit {
	return emits[slices.IndexFunc(emits, func(o *Emit) bool { return o.Target == t })]
}

// otherConsumer is the consumer root copy o lies under when it is not the one e lies under, "" otherwise: each consumer root is written by its own --only-root build alone, so a copy under another one is code no build of e writes (DECISIONS 343, 347). An unplaced copy is judged by the rules that place it.
func otherConsumer(proj *project.Project, o, e *Emit) string {
	if o.Dir == "" || e.Dir == "" {
		return ""
	}
	if c := check.ConsumerRoot(proj, o.Dir); c != check.ConsumerRoot(proj, e.Dir) {
		return c
	}
	return ""
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

// checkCopies is E8004 at a placed copy of a code emit that finds no copy it can use in an import it uses: `noCopy` when the import has several and none under its owning root, `consumerCopy` when the import's only copy lies under another consumer root (CODEGEN.md §2.8, DECISIONS 229, 269, 347).
func (s *stage) checkCopies(u *unit, es *emitSite) {
	if !isCode(es.e.Target) || es.e.Dir == "" {
		return
	}
	proj, target := s.in.Project, es.e.Target
	root := check.RootLabel(proj, check.OwningRoot(proj, es.e.Dir))
	for _, imp := range u.p.Imports {
		first, used := u.importUse(imp.Name, target)
		n := countTarget(imp.Emits, target)
		switch {
		case !used || n == 0 || copyFor(proj, imp.Emits, es.e) != nil:
		case n >= severalCopies:
			u.report(diag.E8004.AtNoCopy(es.outSpan, first, imp.Name, targetWords[target], root))
		default:
			other := otherConsumer(proj, onlyCopy(imp.Emits, target), es.e)
			u.report(diag.E8004.AtConsumerCopy(es.outSpan, imp.Name, targetWords[target], root, other))
		}
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
