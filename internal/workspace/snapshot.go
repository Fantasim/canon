package workspace

import (
	"context"
	"slices"
	"sync"

	"github.com/fantasim/canonlang/internal/build"
)

// Snapshot is what one call reads, each file fixed when first read, and the build over it.
type Snapshot struct {
	p     *Project
	fs    *snapFS
	b     *build.Project
	mu    sync.Mutex
	calls map[string]*call // the computations running now, by key (S8)
}

// Build is the build pipeline over this snapshot's files.
func (s *Snapshot) Build() *build.Project { return s.b }

// Revision is the snapshot's revision (API.md S3): the listing of project.canon, every source
// and existing canon.lock, and every file a load of this project has read, with their content
// in this snapshot, overlays included. The project remembers it (S4).
func (s *Snapshot) Revision(ctx context.Context) (string, error) {
	reads, scanErr := s.b.Inputs()
	reads = append(reads, s.fs.recorded()...)
	lines := make([]build.Listed, 0, len(reads))
	seen := map[string]bool{}
	for _, r := range reads {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if seen[r.Abs] {
			continue
		}
		seen[r.Abs] = true
		e := s.fs.get(name{kind: kindFile, abs: r.Abs}, s.fs.readFile)
		switch e.sum.class {
		case classMissing, classGone:
		case classUnreadable:
			lines = append(lines, build.Listed{Display: r.Display, Unreadable: true})
		case classOK:
			lines = append(lines, build.Listed{Display: r.Display, Sum: e.sum.hash})
		}
	}
	if scanErr != nil {
		lines = append(lines, build.Listed{Display: listingDisplay, Unreadable: true})
	}
	rev := build.RevisionOf(lines)
	s.p.remember(rev, s.fs)
	return rev, nil
}

// Stale is a *StaleError naming each of reads that differs now from revision base, nil for none
// or base "" (API.md S5, S6); an unknown base is stale (S4), and a name first read after base
// is compared with what it first held.
func (s *Snapshot) Stale(base string, reads []build.Read) error {
	if base == "" {
		return nil
	}
	probes := make([][]probe, len(reads))
	for i, r := range reads {
		probes[i] = s.probes(r)
	}
	s.p.mu.Lock()
	defer s.p.mu.Unlock()
	at, ok := s.p.hist.find(base)
	if !ok {
		return &StaleError{}
	}
	var files []string
	for i, r := range reads {
		if pr, changed := s.p.hist.changed(at, probes[i]); changed {
			files = append(files, s.p.hist.named(at, r, pr)...)
		}
	}
	if len(files) == 0 {
		return nil
	}
	slices.Sort(files)
	return &StaleError{Files: slices.Compact(files)}
}

// probe is one entry of a read and its content key in this snapshot; for a package directory,
// the sources it lists now.
type probe struct {
	n       name
	now     sum
	sources []string
}

// probes is what r consulted, as this snapshot holds it, read now if it was not yet: the path
// its links lead to, its listing, its sources, or the file's content then its stat.
func (s *Snapshot) probes(r build.Read) []probe {
	switch {
	case r.Link:
		n := name{kind: kindLink, abs: r.Abs}
		return []probe{{n: n, now: s.fs.get(n, s.fs.resolve).sum}}
	case r.Dir && r.Sources:
		e := s.fs.get(name{kind: kindDir, abs: r.Abs}, s.fs.readDir)
		return []probe{{n: name{kind: kindSources, abs: r.Abs}, now: e.src, sources: s.fs.sources(r.Abs, e.list)}}
	case r.Dir:
		n := name{kind: kindDir, abs: r.Abs}
		return []probe{{n: n, now: s.fs.get(n, s.fs.readDir).sum}}
	}
	file, st := name{kind: kindFile, abs: r.Abs}, name{kind: kindStat, abs: r.Abs}
	return []probe{{n: file, now: s.fs.get(file, s.fs.readFile).sum}, {n: st, now: s.fs.get(st, s.fs.stat).sum}}
}

// displays is the display path of every file among names, in byte order.
func (s *Snapshot) displays(names []name) []string {
	var out []string
	for _, n := range names {
		if n.kind == kindFile {
			out = append(out, s.display(n.abs))
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// display is abs as a load first displayed it, else as the project displays it (WIRE.md §2.3).
func (s *Snapshot) display(abs string) string {
	s.fs.mu.Lock()
	d, ok := s.fs.inputs[abs]
	s.fs.mu.Unlock()
	if ok {
		return d
	}
	return s.b.Display(abs)
}
