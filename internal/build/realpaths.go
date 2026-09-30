package build

import (
	"errors"
	"io/fs"
	"path"

	"github.com/fantasim/canonlang/internal/project"
)

// realPaths resolves names through a file system's links as a load does (log-2026-09-29 M4
// P14-r3): each directory once, and a name only when its directory's listing shows a link; a name
// that cannot be resolved stays as written below its directory's real path.
type realPaths struct {
	fs    project.FS
	dirs  map[string]string
	links map[string]map[string]bool
}

func newRealPaths(fsys project.FS) *realPaths {
	return &realPaths{fs: fsys, dirs: map[string]string{}, links: map[string]map[string]bool{}}
}

// file is name's real path.
func (r *realPaths) file(name string) string {
	return r.follow(name, maxLinks)
}

// follow is file, at most hops links deep: a dangling link leads, through its own text, to the
// name it would be (log-2026-09-29 M4 P14-r4).
func (r *realPaths) follow(name string, hops int) string {
	dir := project.DirOf(name)
	if dir == name {
		return r.dir(name)
	}
	base := path.Base(name)
	if r.linked(dir)[base] {
		real, err := project.EvalSymlinks(r.fs, name)
		if err == nil {
			return real
		}
		if lr, ok := r.fs.(LinkReader); ok && hops > 0 && errors.Is(err, fs.ErrNotExist) {
			if text, err := lr.Readlink(name); err == nil {
				return r.follow(linkTo(dir, text), hops-1)
			}
		}
	}
	return project.Join(r.dir(dir), base)
}

// dir is the real path of directory name; one that does not resolve, as one an edit creates, is
// its own name below its parent's real path.
func (r *realPaths) dir(name string) string {
	if real, ok := r.dirs[name]; ok {
		return real
	}
	real, err := project.EvalSymlinks(r.fs, name)
	if parent := project.DirOf(name); err != nil && parent != name {
		real = project.Join(r.dir(parent), path.Base(name))
	} else if err != nil {
		real = name
	}
	r.dirs[name] = real
	return real
}

// linkTo is the name a link in dir whose text is text leads to: text itself when absolute.
func linkTo(dir, text string) string {
	sys := project.HostPaths()
	if path.IsAbs(text) || sys.Volume(text) != "" {
		return sys.FromAPI(text, dir)
	}
	return project.Join(dir, text)
}

// linked is the names dir's listing shows as links.
func (r *realPaths) linked(dir string) map[string]bool {
	if set, ok := r.links[dir]; ok {
		return set
	}
	set := map[string]bool{}
	list, err := r.fs.ReadDir(dir)
	if err == nil {
		for _, e := range list {
			if e.Type()&fs.ModeSymlink != 0 {
				set[e.Name()] = true
			}
		}
	}
	r.links[dir] = set
	return set
}
