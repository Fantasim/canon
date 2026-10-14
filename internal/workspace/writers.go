package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/writelock"
)

// osDisk reports s's files the operating system's, the only ones the writer locks guard: an
// embedder's own file system takes none, by build's own predicate (DECISIONS 352).
func (s *Snapshot) osDisk() bool {
	return build.LocksWriters(s.fs.base)
}

// osLocker takes the writer locks of the project at an OS directory, never waiting (DECISIONS 375: J3).
type osLocker string

// TryLock takes every package of pkgs exclusive, all or none, without waiting.
func (dir osLocker) TryLock(pkgs []string) (func() error, bool, error) {
	held, ok, err := writelock.TryAcquire(string(dir), writelock.ExclusiveOf(pkgs...))
	if err != nil || !ok {
		return nil, false, err
	}
	return held.Release, true, nil
}

// lockerFor is the Locker of the project at dir on fsys: nil unless fsys takes writer locks
// and this platform has them, the one test of lockability (DECISIONS 375: J12).
func lockerFor(fsys project.FS, dir string) edit.Locker {
	if !build.LocksWriters(fsys) || !writelock.Lockable() {
		return nil
	}
	return osLocker(filepath.FromSlash(dir))
}

// writers are the packages o locks: those it touches, each written canon.lock's, and every
// reader in a, the analysis before it, of a data file it writes (DECISIONS 375: J7).
func (o *EditOutcome) writers(a *build.Analysis) []string {
	pkgs := slices.Clone(o.Plan.Touched)
	for _, l := range o.wroteLocks {
		pkgs = append(pkgs, l.Package)
	}
	pkgs = append(pkgs, dataReaders(a, o.dataFiles())...)
	pkgs = slices.DeleteFunc(pkgs, func(p string) bool { return p == "" })
	slices.Sort(pkgs)
	return slices.Compact(pkgs)
}

// dataFiles are the absolute names of the files o writes, renamed ones' two names included, that
// are neither a source nor a canon.lock.
func (o *EditOutcome) dataFiles() []string {
	var out []string
	for _, c := range o.Changes {
		for _, display := range []string{c.Path, c.OldPath} {
			if !o.isData(c, display) {
				continue
			}
			if abs, ok := o.Before.b.Abs(display); ok {
				out = append(out, abs)
			}
		}
	}
	return out
}

// isData says whether display, a name of c, is a data file o writes.
func (o *EditOutcome) isData(c edit.Change, display string) bool {
	if display == "" || c.Kind == edit.ChangeRemovedDir || strings.HasSuffix(display, project.SourceExt) {
		return false
	}
	return !slices.ContainsFunc(o.wroteLocks, func(l build.Lock) bool { return l.Path == display })
}

// dataReaders are the packages of a whose Reads hold one of files, compared by real path.
func dataReaders(a *build.Analysis, files []string) []string {
	if len(files) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, rp := range a.RealPaths(files...) {
		want[rp] = true
	}
	units := a.Units()
	reads := make([][]string, len(units))
	var all []string
	for i, u := range units {
		if u.Name != "" {
			reads[i] = fileReads(a.Reads(u.Name))
			all = append(all, reads[i]...)
		}
	}
	slices.Sort(all)
	all = slices.Compact(all)
	held := map[string]bool{}
	for i, rp := range a.RealPaths(all...) {
		held[all[i]] = want[rp]
	}
	var out []string
	for i, u := range units {
		if slices.ContainsFunc(reads[i], func(abs string) bool { return held[abs] }) {
			out = append(out, u.Name)
		}
	}
	return out
}

// fileReads are the absolute names of the files reads holds, listings and link targets left out.
func fileReads(reads []build.Read) []string {
	var out []string
	for _, rd := range reads {
		if !rd.Dir && !rd.Link {
			out = append(out, rd.Abs)
		}
	}
	return out
}

// lockWriters takes on the OS's files pkgs exclusive, waiting with ctx, grown to the journals they
// meet (J4); a file no package owns, or no lock primitive, takes them and records none (J7, J12).
// The commit then runs N9 under them (API.md S9, N9; DECISIONS 352, 375).
func (s *Snapshot) lockWriters(ctx context.Context, site *edit.Site, pkgs []string, unowned bool) (func() error, []writelock.Request, error) {
	reqs := writelock.ExclusiveOf(pkgs...)
	if !s.osDisk() || len(reqs) == 0 {
		return writelock.NoRelease, nil, nil
	}
	locker := lockerFor(s.fs.base, s.b.Dir())
	if locker == nil || unowned {
		release, err := plainAcquire(ctx, s.fs.base, s.b.Dir(), reqs)
		return release, nil, err
	}
	site.Locks = locker
	held, reqs, err := acquireGrown(ctx, site, s.b.Dir(), writelock.Normalize(reqs))
	if err != nil {
		return nil, nil, err
	}
	return held.Release, reqs, nil
}

// settleFirst rolls back the journals the commit's locks cover; one that changed a file makes the
// edit a *StaleError naming them, which writes nothing (API.md N11; DECISIONS 375: J5).
func (s *Snapshot) settleFirst(site edit.Site, reqs []writelock.Request) error {
	if site.Locks == nil {
		return nil
	}
	changed, err := settle(site, reqs, s.p.logger)
	if len(changed) > 0 {
		return &StaleError{Files: changed}
	}
	return err
}

// plainAcquire takes reqs on fsys's project at dir as DECISIONS 352 does, settling nothing: none
// on a file system that takes no writer lock.
func plainAcquire(ctx context.Context, fsys project.FS, dir string, reqs []writelock.Request) (func() error, error) {
	if !build.LocksWriters(fsys) || len(reqs) == 0 {
		return writelock.NoRelease, nil
	}
	held, err := writelock.Acquire(ctx, filepath.FromSlash(dir), reqs)
	if err != nil {
		return nil, err
	}
	return held.Release, nil
}

// acquireGrown holds reqs; while a journal they meet records a package not held exclusive, it frees
// all and takes the union, never a lock while holding one (DECISIONS 375: J4). site.Held is what
// it holds exclusive.
func acquireGrown(ctx context.Context, site *edit.Site, dir string, reqs []writelock.Request) (*writelock.Held, []writelock.Request, error) {
	for {
		held, err := writelock.Acquire(ctx, filepath.FromSlash(dir), reqs)
		if err != nil {
			return nil, nil, err
		}
		site.Held = exclusive(reqs)
		next, grown, err := grow(*site, reqs)
		if err == nil && !grown {
			return held, reqs, nil
		}
		if rerr := held.Release(); err != nil || rerr != nil {
			return nil, nil, errors.Join(err, rerr)
		}
		reqs = writelock.Normalize(next)
	}
}
