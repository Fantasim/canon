package build

import (
	"context"

	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/writelock"
)

// writersLocker is a file system whose writers take the package writer locks: the OS's
// (DECISIONS 352). Any other, an embedder's or a test's, takes none.
type writersLocker interface {
	lockWriters(ctx context.Context, dir string, reqs []writelock.Request) (*writelock.Held, error)
}

// freshViewer is a file system that also reads the disk as it is now, its overlays over it: a
// workspace snapshot's, whose own reads are kept (DECISIONS 352).
type freshViewer interface {
	Fresh() project.FS
}

// noRelease releases no lock.
func noRelease() error { return nil }

// locking reports a build that takes the writer locks: one that writes, on the OS's file system.
func (r *run) locking(opt BuildOptions) bool {
	_, ok := r.p.disk.(writersLocker)
	return ok && !opt.Check && opt.OnlyRoot == ""
}

// lockWriters takes the writer locks of a build that writes, through the file system Open was
// given: each selected package exclusively, every other loaded package shared, in name order,
// waiting as long as ctx lets it. --check and --only-root take none (DECISIONS 343, 352).
func (r *run) lockWriters(ctx context.Context, opt BuildOptions) (func() error, error) {
	if !r.locking(opt) {
		return noRelease, nil
	}
	var reqs []writelock.Request
	for _, u := range r.loaded {
		if u.Name == "" {
			continue
		}
		mode := writelock.Shared
		if r.selects(u.Name) {
			mode = writelock.Exclusive
		}
		reqs = append(reqs, writelock.Request{Package: u.Name, Mode: mode})
	}
	held, err := r.p.disk.(writersLocker).lockWriters(ctx, r.p.dir, reqs)
	if err != nil {
		return nil, interrupted(ctx, err)
	}
	return held.Release, nil
}

// fresh is what the run re-reads under its writer locks: the snapshot's own view of the disk as
// it is now, overlays included; the file system itself where no snapshot keeps its reads.
func (r *run) fresh() (project.FS, bool) {
	if v, ok := r.p.fs.(freshViewer); ok {
		return v.Fresh(), true
	}
	return r.p.fs, false
}
