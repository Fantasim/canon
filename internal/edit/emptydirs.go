package edit

import (
	"cmp"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/project"
)

// removedDirs are the directories the edit leaves empty, deepest first (API.md N6): a
// directory whose every entry is gone, or is such a directory itself; a load.dir walk's base
// stays (DECISIONS 482), and is looked for only when some directory would go.
func (a *applier) removedDirs() ([]Change, error) {
	dirs := slices.Sorted(maps.Keys(a.emptied))
	slices.SortStableFunc(dirs, func(x, y string) int {
		return cmp.Compare(strings.Count(y, pathSep), strings.Count(x, pathSep))
	})
	over := a.overlay()
	out := a.emptiedDirs(over, dirs, nil)
	if len(out) == 0 {
		return nil, nil
	}
	kept, err := a.walkBases(out)
	if err != nil {
		return nil, err
	}
	return a.emptiedDirs(over, dirs, kept), nil
}

// emptiedDirs are those of dirs, deepest first, that hold nothing once the edit is made, kept
// ones and those holding them excepted.
func (a *applier) emptiedDirs(over *overlayFS, dirs []string, kept map[string]bool) []Change {
	gone := map[string]bool{}
	var out []Change
	for _, d := range dirs {
		abs, ok := a.env.Project.Abs(d)
		if !ok || kept[d] {
			continue
		}
		list, err := over.ReadDir(abs)
		if err != nil || slices.ContainsFunc(list, func(e fs.DirEntry) bool { return !gone[path.Join(d, e.Name())] }) {
			continue
		}
		gone[d] = true
		out = append(out, Change{Kind: ChangeRemovedDir, Path: d})
	}
	return out
}

// walkBases are those of changes' directories that a load.dir walk starts at, by real path,
// once the edit is made: a file it rewrites counts by its new text, one it deletes not at all
// (API.md N6, DECISIONS 482; WIRE.md 6.5).
func (a *applier) walkBases(changes []Change) (map[string]bool, error) {
	an := a.base.a
	var names []string
	for _, b := range an.WalkBases() {
		if s, edited := a.files[b.File]; !edited || !s.changed() {
			names = append(names, b.Dir)
		}
	}
	for _, display := range slices.Sorted(maps.Keys(a.files)) {
		s := a.files[display]
		if s.cur == nil || !s.changed() || path.Ext(display) != project.SourceExt {
			continue
		}
		f, err := parseAs(display, s.cur)
		if err != nil {
			return nil, err
		}
		names = append(names, an.FileWalkBases(f)...)
	}
	bases := len(names)
	for _, c := range changes {
		abs, _ := a.env.Project.Abs(c.Path) // emptiedDirs kept only directories that resolve
		names = append(names, abs)
	}
	real := an.RealPaths(names...)
	want := map[string]bool{}
	for _, r := range real[:bases] {
		want[r] = true
	}
	kept := map[string]bool{}
	for i, r := range real[bases:] {
		kept[changes[i].Path] = want[r]
	}
	return kept, nil
}

// changed reports that the edit leaves the file other than the base held it.
func (s *fileState) changed() bool {
	_, ok := s.change()
	return ok
}
