package build

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// skipAbsent drops the outputs under an absent optional root, one W8024 (--check: E8023) per root (CODEGEN.md §2.4).
func (r *run) skipAbsent(outputs []*output, check bool) []*output {
	counts := map[string]int64{}
	kept := outputs[:0:0]
	for _, o := range outputs {
		if o.under == "" {
			kept = append(kept, o)
			continue
		}
		counts[o.under]++
	}
	for _, root := range slices.Sorted(maps.Keys(counts)) {
		r.skipFinding(root, counts[root], check).Report(r.s.own)
	}
	return kept
}

// skipFinding is the finding of n outputs skipped under root: W8024, or E8023 under --check, one or many,
// at where this machine's path of root is written.
func (r *run) skipFinding(root string, n int64, check bool) *diag.Builder {
	span, written := r.placedAs(root)
	switch {
	case check && n == 1:
		return diag.E8023.AtOne(span, root, written)
	case check:
		return diag.E8023.AtMany(span, n, root, written)
	case n == 1:
		return diag.W8024.AtOne(span, root, written)
	}
	return diag.W8024.AtMany(span, n, root, written)
}

// absentAt is the absent optional root at or closest above abs, "" for none: roots nest (CODEGEN.md §2.4).
func (r *run) absentAt(abs string) string {
	best, bestDir := "", ""
	for _, root := range r.s.proj.Roots {
		dir := rootDir(r.s.layout, root.Name)
		if _, in := under(dir, abs); in && r.s.layout.Absent(root.Name) && len(dir) > len(bestDir) {
			best, bestDir = root.Name, dir
		}
	}
	return best
}

// placedAs is root's path as written where this machine's placement sets it: --root, project.local.canon, project.canon (SPEC §3.1).
func (r *run) placedAs(root string) (source.Span, string) {
	if written, ok := r.p.opt.Roots[root]; ok {
		return source.Span{}, written
	}
	if r.s.local != nil {
		local := r.s.local.Roots
		if i := slices.IndexFunc(local, func(l project.Root) bool { return l.Name == root }); i >= 0 {
			return local[i].Span, local[i].Path
		}
	}
	declared, _ := r.s.proj.Root(root) // found: root is the root of an output's out
	return declared.Span, declared.Path
}

// dirBounds is where a build may create directories: inside the project and below a root outside it (CODEGEN.md §2.4).
type dirBounds struct {
	project string
	roots   []string // the directories of the roots placed outside the project
}

// bounds is this machine's dirBounds, the roots placed as the run's layout places them.
func (r *run) bounds() *dirBounds {
	b := &dirBounds{project: r.s.layout.Dir}
	for _, dir := range r.s.layout.RootDirs() {
		if _, in := under(b.project, dir); !in {
			b.roots = append(b.roots, dir)
		}
	}
	return b
}

// allows reports a directory the build may create: inside the project, or strictly below a
// root outside it and no such root's own directory.
func (b *dirBounds) allows(dir string) bool {
	if _, in := under(b.project, dir); in {
		return true
	}
	if slices.Contains(b.roots, dir) {
		return false
	}
	return slices.ContainsFunc(b.roots, func(root string) bool {
		_, in := under(root, dir)
		return in
	})
}
