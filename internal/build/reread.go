package build

import (
	"crypto/sha256"
	"io/fs"
	"maps"
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
)

// unmoved reads again, under the writer locks and through the snapshot's own view, what the run's
// outputs depend on; a *StaleError names each file that changed (DECISIONS 352).
func (r *run) unmoved(opt BuildOptions) error {
	if !r.locking(opt) {
		return nil
	}
	view, _ := r.fresh()
	moved := r.movedFiles(view)
	moved = append(moved, r.movedSources(view)...)
	moved = append(moved, r.movedLoads(view)...)
	moved = append(moved, r.movedListings(view)...)
	if len(moved) == 0 {
		return nil
	}
	slices.Sort(moved)
	return &StaleError{Files: slices.Compact(moved)}
}

// movedFiles are the project files, the loaded packages' sources and canon.lock places, and the
// loaded data files whose bytes in view are not the snapshot's: other, appeared or gone.
func (r *run) movedFiles(view project.FS) []string {
	read := map[string][sha256.Size]byte{}
	for _, f := range r.s.sums {
		read[f.Path] = f.Sum
	}
	var moved []string
	for _, name := range r.dependedOn() {
		sum, was := read[name]
		if movedFile(view, project.Join(r.p.dir, name), sum, was) {
			moved = append(moved, name)
		}
	}
	loaded := r.inputs().loaded
	for _, abs := range slices.Sorted(maps.Keys(loaded)) {
		if movedFile(view, abs, loaded[abs].sum, true) {
			moved = append(moved, loaded[abs].display)
		}
	}
	return moved
}

// dependedOn is the project-relative path of the project files, the loaded packages' files and
// every canon.lock place above them, whether or not it exists.
func (r *run) dependedOn() []string {
	names := []string{project.FileName, project.LocalFileName}
	var sources []string
	for _, u := range r.loaded {
		for _, f := range u.Files {
			sources = append(sources, f.Src.Path)
		}
	}
	names = append(append(names, sources...), lockPaths(sources)...)
	slices.Sort(names)
	return slices.Compact(names)
}

// movedFile reports the file abs in view not as the snapshot read it: other bytes than sum, or
// present where the snapshot found nothing (was false), or gone.
func movedFile(view project.FS, abs string, sum [sha256.Size]byte, was bool) bool {
	data, err := view.ReadFile(abs)
	switch {
	case cannotExist(err):
		return was
	case err != nil:
		return true
	}
	return !was || sha256.Sum256(data) != sum
}

// movedSources are the sources that appeared in, or left, the directory of a loaded package.
func (r *run) movedSources(view project.FS) []string {
	then := map[string][]string{}
	for _, name := range r.s.names {
		then[path.Dir(name)] = append(then[path.Dir(name)], name)
	}
	dirs := map[string]bool{}
	for _, u := range r.loaded {
		for _, f := range u.Files {
			dirs[path.Dir(f.Src.Path)] = true
		}
	}
	var moved []string
	for _, dir := range slices.Sorted(maps.Keys(dirs)) {
		entries, err := view.ReadDir(project.Join(r.p.dir, dir))
		var now []string
		for _, e := range entries {
			if name := path.Join(dir, e.Name()); project.IsSource(name, e) {
				now = append(now, name)
			}
		}
		if err != nil && !cannotExist(err) {
			now = append(now, dir)
		}
		moved = append(moved, symmetric(then[dir], now)...)
	}
	return moved
}

// movedLoads are the files a load of a loaded package names, a load.dir's matches included, in
// view and not when the run began, or the other way round.
func (r *run) movedLoads(view project.FS) []string {
	return symmetric(r.writes().named, r.namedByLoads(view))
}

// namedByLoads is the display path of every file a load of a loaded package names in fsys.
func (r *run) namedByLoads(fsys project.FS) []string {
	var out []string
	scratch := diag.NewBag(nil, "")
	for _, u := range r.loaded {
		for _, f := range u.Files {
			for _, at := range r.s.gen.loadsOf(f) {
				out = append(out, displays(load.Files(fsys, r.s.layout, path.Dir(f.Src.Path), at.e, scratch))...)
			}
		}
	}
	return out
}

// movedListings are the directories an asset check listed whose names in view are not those the
// run used.
func (r *run) movedListings(view project.FS) []string {
	var moved []string
	for _, h := range r.manifestHosts() {
		if h.assets == nil {
			continue
		}
		for _, abs := range slices.Sorted(maps.Keys(h.assets.dirs)) {
			if len(symmetric(listingNames(h.assets.dirs[abs]), listedNow(view, abs))) > 0 {
				moved = append(moved, relativeTo(r.p.dir, abs, pathRune))
			}
		}
	}
	return moved
}

// listedNow are the names of dir in view an asset check lists: a link only when it leads to
// something; none for a directory gone, as the run listed none.
func listedNow(view project.FS, dir string) []string {
	entries, _ := view.ReadDir(dir) // a listing that fails lists nothing, as list does
	var out []string
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink != 0 {
			if _, err := view.Stat(project.Join(dir, e.Name())); err != nil {
				continue
			}
		}
		out = append(out, e.Name())
	}
	return out
}

// listingNames are a listing's file and subdirectory names.
func listingNames(l dirListing) []string {
	return append(slices.Collect(maps.Keys(l.files)), slices.Collect(maps.Keys(l.subs))...)
}

// displays are the display paths of files.
func displays(files []load.File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Display
	}
	return out
}

// symmetric is the names in one of a and b but not both, sorted.
func symmetric(a, b []string) []string {
	count := map[string]int{}
	for _, x := range slices.Compact(slices.Sorted(slices.Values(a))) {
		count[x]++
	}
	for _, x := range slices.Compact(slices.Sorted(slices.Values(b))) {
		count[x]++
	}
	return slices.Sorted(func(yield func(string) bool) {
		for x, n := range count { //canon:unordered sorted by slices.Sorted
			if n == 1 && !yield(x) {
				return
			}
		}
	})
}
