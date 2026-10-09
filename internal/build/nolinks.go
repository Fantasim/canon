package build

import (
	"errors"
	"io/fs"
	"path"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

// linkScan is one search for symbolic links on output paths: each directory listed once, its
// link names in lower case (DECISIONS 342: a link name matches whatever its letter case).
type linkScan struct {
	fs    project.FS
	links map[string]map[string]bool
}

// throughLinks reports E8027 for each output, a removal included, with a symbolic link below
// its base directory, the file itself included, and whether one has; a directory that cannot be
// listed is an error, never "no link" (DECISIONS 342).
func (r *run) throughLinks(outputs []*output) (bool, error) {
	scan := &linkScan{fs: r.p.fs, links: map[string]map[string]bool{}}
	found := false
	for _, o := range outputs {
		link, ok, err := scan.firstLink(r.linkBase(o.Abs), o.Abs)
		if err != nil {
			return false, displayError(o.Path, err)
		}
		if ok {
			diag.E8027.At(o.at, o.Path, r.linkShown(o.Path, o.Abs, link)).Report(r.s.bag(o.Package))
			found = true
		}
	}
	return found, nil
}

// linkBase is the directory below which abs may hold no link: the project directory for an
// output inside the project, whatever root it names; else the deepest root holding it (DECISIONS 342).
func (r *run) linkBase(abs string) string {
	if _, in := under(r.s.layout.Dir, abs); in {
		return r.s.layout.Dir
	}
	if _, in := under(r.rootMade, abs); in && r.rootMade != "" { // a root --only-root creates is no link either (DECISIONS 343)
		return project.DirOf(r.rootMade)
	}
	best := ""
	for _, d := range r.s.layout.RootDirs() {
		if _, in := under(d, abs); in && len(d) > len(best) {
			best = d
		}
	}
	return best
}

// firstLink is the link closest to base among abs and its directories strictly below base.
func (s *linkScan) firstLink(base, abs string) (string, bool, error) {
	link, found := "", false
	for name := abs; name != base && name != project.DirOf(name); name = project.DirOf(name) {
		isLink, err := s.isLink(name)
		if err != nil {
			return "", false, err
		}
		if isLink {
			link, found = name, true
		}
	}
	return link, found, nil
}

// isLink reports name a link as its directory's listing shows it; a directory that does not
// exist holds none.
func (s *linkScan) isLink(name string) (bool, error) {
	dir := project.DirOf(name)
	set, ok := s.links[dir]
	if !ok {
		list, err := s.fs.ReadDir(dir)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
		set = map[string]bool{}
		for _, e := range list {
			if linkEntry(e.Type()) {
				set[strings.ToLower(e.Name())] = true
			}
		}
		s.links[dir] = set
	}
	return set[strings.ToLower(path.Base(name))], nil
}

// linkShown is the display of link, abs or one of its directories, abs being shown as display:
// cut from display where the link lies at or below its root, else project-relative.
func (r *run) linkShown(display, abs, link string) string {
	rel, _ := under(link, abs)
	if rel == listingMark {
		return display
	}
	cut := strings.Count(rel, pathSep) + 1
	if strings.HasPrefix(display, rootMark) && cut > strings.Count(display, pathSep) {
		return r.s.shown(link)
	}
	for range cut {
		display = path.Dir(display)
	}
	return display
}
