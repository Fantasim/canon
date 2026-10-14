package build

import (
	"context"
	"errors"
	"slices"

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

// locking reports a build that takes the writer locks: one that writes, on the OS's file system.
func (r *run) locking(opt BuildOptions) bool {
	return LocksWriters(r.p.disk) && !opt.Check && opt.OnlyRoot == ""
}

// lockWriters takes a writing build's locks in name order (selected exclusive, other loaded shared),
// waiting as long as ctx lets it, grows them to the journals it meets and settles those (J4).
// --check and --only-root take none and settle nothing (DECISIONS 343, 352, 375).
func (r *run) lockWriters(ctx context.Context, opt BuildOptions) (func() error, error) {
	if !r.locking(opt) {
		return writelock.NoRelease, nil
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
	held, err := r.settled(ctx, writelock.Normalize(reqs))
	if err != nil {
		return nil, err
	}
	return held.Release, nil
}

// settled holds reqs, grown by the Settler's meeting journals until stable, then settles (ADR-0021 J4).
func (r *run) settled(ctx context.Context, reqs []writelock.Request) (*writelock.Held, error) {
	locker := r.p.disk.(writersLocker)
	for {
		held, err := locker.lockWriters(ctx, r.p.dir, reqs)
		if err != nil {
			return nil, interrupted(ctx, err)
		}
		if r.p.settle == nil {
			return held, nil
		}
		want, err := r.p.settle.Meeting(reqs)
		if err != nil {
			return nil, errors.Join(err, held.Release())
		}
		next := writelock.Normalize(append(slices.Clone(reqs), want...))
		if slices.Equal(next, reqs) {
			return r.settle(held, reqs)
		}
		if err := held.Release(); err != nil { // never acquire while holding: flock is per open file
			return nil, err
		}
		reqs = next
	}
}

// settle rolls back the journals held covers; the run's re-read then sees what it changed (DECISIONS 352).
func (r *run) settle(held *writelock.Held, reqs []writelock.Request) (*writelock.Held, error) {
	if _, err := r.p.settle.Settle(reqs); err != nil {
		return nil, errors.Join(err, held.Release())
	}
	return held, nil
}

// WithSettle is p settling through s before a build writes, as is every Over of it (ADR-0021 J10).
func (p *Project) WithSettle(s Settler) *Project {
	out := *p
	out.settle = s
	return &out
}

// fresh is what the run re-reads under its writer locks: the snapshot's own view of the disk as
// it is now, overlays included; the file system itself where no snapshot keeps its reads.
func (r *run) fresh() (project.FS, bool) {
	if v, ok := r.p.fs.(freshViewer); ok {
		return v.Fresh(), true
	}
	return r.p.fs, false
}
