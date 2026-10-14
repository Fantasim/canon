package build

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// dirRename is a directory the disk holds under another spelling, which the write renames to the
// declared one in one rename before any output below it, and its display path (DECISIONS 377).
type dirRename struct {
	from, to, display string
}

// spelling sets o.respell when o, an existing file of any target but a canon.outputs, is on disk
// under another spelling, the only one its directory lists: one file, which takes its declared
// spelling where the build may overwrite it, judge decides (DECISIONS 357, 377).
func (r *run) spelling(o *output) error {
	if o.listing {
		return nil
	}
	dir, base := project.DirOf(o.Abs), path.Base(o.Abs)
	names, err := r.writes().listing(r.p.fs, dir)
	if err != nil {
		return displayError(path.Dir(o.Path), err)
	}
	if slices.Contains(names, base) {
		return nil
	}
	if i := slices.IndexFunc(names, func(n string) bool { return strings.EqualFold(n, base) }); i >= 0 {
		o.respell = project.Join(dir, names[i])
	}
	return nil
}

// planner gathers a write's directory renames: each directory judged once, one that outputs
// declare under two spellings left as the disk has it (DECISIONS 377).
type planner struct {
	r        *run
	only     string
	may      *dirBounds
	sources  map[string]bool   // lower-cased: the package directories and those above one
	judged   map[string]bool   // the declared directories already judged
	declared map[string]string // lower-cased directory: the first spelling an output declares
	twice    map[string]bool   // lower-cased: directories declared under two spellings
	renames  []dirRename
}

// dirSpellings plans the directory renames of outputs, top-down; --only-root renames below its root alone (CODEGEN.md §2.4, DECISIONS 377).
func (r *run) dirSpellings(outputs []*output, only string) error {
	pl := &planner{r: r, only: only, may: r.bounds(), sources: r.packageDirs(), judged: map[string]bool{}, declared: map[string]string{}, twice: map[string]bool{}}
	for _, o := range outputs {
		if o.remove || o.listing {
			continue
		}
		if err := pl.walk(o); err != nil {
			return err
		}
	}
	r.renames = slices.DeleteFunc(pl.renames, func(rn dirRename) bool { return pl.twice[strings.ToLower(rn.to)] })
	slices.SortStableFunc(r.renames, func(a, b dirRename) int {
		return strings.Count(a.to, pathSep) - strings.Count(b.to, pathSep)
	})
	return nil
}

// walk judges the directories canon may create on o's path, top-down, up to the first that is
// on disk under neither its declared spelling nor another its parent lists alone (DECISIONS 377).
func (pl *planner) walk(o *output) error {
	for _, d := range pl.creatable(project.DirOf(o.Abs)) {
		key := strings.ToLower(d)
		if first, ok := pl.declared[key]; ok && first != d {
			pl.twice[key] = true
		}
		if pl.judged[d] {
			continue
		}
		pl.judged[d] = true
		disk, ok, err := pl.r.spelledAs(d)
		switch {
		case err != nil:
			return displayError(pl.r.dirShown(o, d), err)
		case !ok:
			return nil
		}
		if _, seen := pl.declared[key]; !seen {
			pl.declared[key] = d
		}
		if err := pl.plan(dirRename{from: disk, to: d, display: pl.r.dirShown(o, d)}); err != nil {
			return err
		}
	}
	return nil
}

// creatable are the directories on dir's path the build may create, top-down, never the project's own or a root's outside it (CODEGEN.md §2.4).
func (pl *planner) creatable(dir string) []string {
	var out []string
	for d := dir; d != project.DirOf(d); d = project.DirOf(d) {
		if d != pl.may.project && pl.may.allows(d) {
			out = append(out, d)
		}
	}
	slices.Reverse(out)
	return out
}

// plan records rn when the disk spells the directory otherwise; a directory holding sources is
// never renamed, and --only-root renames below its root alone (E8028).
func (pl *planner) plan(rn dirRename) error {
	switch {
	case rn.from == rn.to:
		return nil
	case pl.sources[strings.ToLower(rn.from)]:
		return internal(fmt.Errorf(fmtDisplayCause, rn.display, errRenameSources))
	case pl.r.mayRename(pl.only, rn):
		pl.renames = append(pl.renames, rn)
	}
	return nil
}

// spelledAs is the spelling on disk of the directory d: d itself when its parent lists it so,
// another when the listing shows only that one, equal but for case, and d resolves; false when
// the listing shows neither, or the two are two directories (DECISIONS 377).
func (r *run) spelledAs(d string) (string, bool, error) {
	parent, base := project.DirOf(d), path.Base(d)
	names, err := r.writes().listing(r.p.fs, parent)
	if err != nil {
		return "", false, err
	}
	if slices.Contains(names, base) {
		return d, true, nil
	}
	i := slices.IndexFunc(names, func(n string) bool { return strings.EqualFold(n, base) })
	if i < 0 {
		return "", false, nil
	}
	_, err = r.p.fs.Stat(d)
	switch {
	case cannotExist(err):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	return project.Join(parent, names[i]), true, nil
}

// dirShown is the display path of d, a directory on o's path: o's display path less the
// segments below d while one is left below its root, else d relative to the project.
func (r *run) dirShown(o *output, d string) string {
	if shown, ok := dirDisplay(o.Path, o.Abs, d); ok {
		return shown
	}
	rel, _ := under(r.p.dir, d) // inside: a directory below a root outside has a display path
	return rel
}

// dirDisplay is the display path of d, a directory abs passes through, abs shown as display:
// display less abs's segments below d; false when that leaves no segment below a root.
func dirDisplay(display, abs, d string) (string, bool) {
	segs := strings.Split(display, pathSep)
	keep := len(segs) - strings.Count(abs, pathSep) + strings.Count(d, pathSep)
	if keep < 1 || keep == 1 && strings.HasPrefix(segs[0], rootMark) || keep > len(segs) {
		return "", false
	}
	return strings.Join(segs[:keep], pathSep), true
}

// renamedAbove reports abs below a directory the write renames (DECISIONS 377).
func (r *run) renamedAbove(abs string) bool {
	return slices.ContainsFunc(r.renames, func(rn dirRename) bool {
		rel, in := under(rn.to, abs)
		return in && rel != listingMark
	})
}

// listedAt reports the file abs, as the disk spells it, named by a line of pkg's canon.outputs, each placed and compared exactly, as the stale judge does (CODEGEN.md §2.9, DECISIONS 377).
func (own *ownership) listedAt(pkg string, names map[string]string, abs string) bool {
	placed, ok := own.placed[pkg]
	if !ok {
		placed = map[string]bool{}
		for name := range names { //canon:unordered a set
			if at, ok := own.r.s.layout.Resolve(name, "", source.Span{}, diag.NewBag(nil, pkg)); ok {
				placed[at.Abs] = true
			}
		}
		own.placed[pkg] = placed
	}
	return placed[abs]
}

// onDisk is the path of o's file as the disk spells it: its own spelling there, below each
// renamed directory's.
func (r *run) onDisk(o *output) string {
	abs := o.Abs
	if o.respell != "" {
		abs = o.respell
	}
	for _, rn := range slices.Backward(r.renames) {
		if rel, in := under(rn.to, abs); in {
			abs = project.Join(rn.from, rel)
		}
	}
	return abs
}
