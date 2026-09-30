package workspace

import (
	"io/fs"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

// loaders reads an entry of each kind from the file system under a snapshot, as on first use.
var loaders = [...]func(*snapFS, string) *entry{
	kindFile: (*snapFS).readFile,
	kindDir:  (*snapFS).readDir,
	kindStat: (*snapFS).stat,
	kindLink: (*snapFS).resolve,
}

// diff is what differs between two snapshots, by absolute name: every entry and listed name,
// and the files among them.
type diff struct {
	changed  []string
	files    []string
	relisted []string
}

// since is what differs from from, published before s, run as s is published: the files that
// changed or a listing gained or lost, with published, the files the publisher named, and every
// name that differs, absolute, in byte order (API.md S2). A name s lacks is read in s now.
func (s *Snapshot) since(from *Snapshot, published []string) Change {
	old := from.fs.entries()
	var d diff
	for _, n := range slices.SortedFunc(maps.Keys(old), compareNames) {
		e := old[n]
		now := s.fs.get(n, func(a string) *entry { return loaders[n.kind](s.fs, a) })
		if now.sum != e.sum {
			d.add(n, e, now)
		}
	}
	files := make([]Path, 0, len(d.files)+len(published))
	for _, a := range d.files {
		files = append(files, Path{Display: s.display(a), Abs: a})
	}
	for _, disp := range published {
		if a, ok := s.b.Abs(disp); ok {
			files = append(files, Path{Display: disp, Abs: a})
			d.changed = append(d.changed, a)
		}
	}
	return Change{Files: unionPaths(files, nil), Changed: union(d.changed, nil), Relisted: union(d.relisted, nil)}
}

// add records n, whose entry was e and is now: a file, or the names a listing gained or lost.
func (d *diff) add(n name, e, now *entry) {
	d.changed = append(d.changed, n.abs)
	if n.kind == kindFile {
		d.files = append(d.files, n.abs)
	}
	if n.kind == kindDir {
		d.relisted = append(d.relisted, n.abs)
		d.gone(n.abs, e.list, now.list)
		d.gone(n.abs, now.list, e.list)
	}
}

// gone records every name the listing of dir had holds and has does not.
func (d *diff) gone(dir string, had, has []fs.DirEntry) {
	kept := map[listed]bool{}
	for _, e := range has {
		kept[listed{e.Name(), e.IsDir()}] = true
	}
	for _, e := range had {
		if kept[listed{e.Name(), e.IsDir()}] {
			continue
		}
		abs := project.Join(dir, e.Name())
		d.changed = append(d.changed, abs)
		if !e.IsDir() {
			d.files = append(d.files, abs)
		}
	}
}

// listed is an entry of a listing as a diff compares it.
type listed struct {
	name string
	dir  bool
}

// entries is every entry s holds now, by name.
func (s *snapFS) entries() map[name]*entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.ents)
}

// Recorded is every file a load of this project read so far, which the revision lists with the
// build's inputs (API.md S3).
func (s *Snapshot) Recorded() []build.Read {
	return s.fs.recorded()
}
