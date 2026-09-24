package build

import (
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// dirConflicts reports E2001 for each loaded package's directory that also legally holds a
// file of another package, loaded or not, neither being the directory's own (DECISIONS 217):
// the sole judge, once per package.
func (r *run) dirConflicts() {
	for _, u := range r.loaded {
		seen := map[string]bool{}
		for _, f := range u.Files {
			dir := path.Dir(f.Src.Path)
			if seen[dir] || !legalIn(u.Name, dotted(dir)) {
				continue
			}
			seen[dir] = true
			r.reportDirConflict(u, f, dir)
		}
	}
}

// reportDirConflict reports E2001 at f unless dir holds its own package (TYPES.md §3.1).
func (r *run) reportDirConflict(u *project.Unit, f *syntax.File, dir string) {
	names := packagesIn(r.s.units, dir)
	if slices.Contains(names, dotted(dir)) {
		return
	}
	for _, other := range names { // the alphabetically first other package
		if other != u.Name {
			diag.E2001.At(f.Span(f.Package), dir, u.Name, other).Report(r.s.bag(u.Name))
			return
		}
	}
}

// packagesIn is the sorted names of the legally placed packages holding a file in dir.
func packagesIn(all []*project.Unit, dir string) []string {
	own := dotted(dir)
	var names []string
	for _, u := range all {
		if legalIn(u.Name, own) && hasFileIn(u, dir) {
			names = append(names, u.Name)
		}
	}
	slices.Sort(names)
	return names
}

// dotted is dir's implied package name (TYPES.md §3.1), "" at the project root.
func dotted(dir string) string {
	if dir == listingMark {
		return ""
	}
	return strings.ReplaceAll(dir, pathSep, qnameSep)
}

// legalIn reports pkg as own or an ancestor of own (TYPES.md §3.1 rule 1).
func legalIn(pkg, own string) bool {
	return pkg == own || strings.HasPrefix(own, pkg+qnameSep)
}

// hasFileIn reports a file of u sitting in dir.
func hasFileIn(u *project.Unit, dir string) bool {
	for _, f := range u.Files {
		if path.Dir(f.Src.Path) == dir {
			return true
		}
	}
	return false
}
