package workspace

import (
	"context"
	"path/filepath"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/writelock"
)

// osDisk reports s's files the operating system's, the only ones the writer locks guard: an
// embedder's own file system takes none, by build's own predicate (DECISIONS 352).
func (s *Snapshot) osDisk() bool {
	return build.LocksWriters(s.fs.base)
}

// lockWriters takes, on the OS's files, the writer lock of each package o writes, created ones
// included, exclusively, in name order, all or none, waiting as long as ctx lets it; the commit
// then runs N9 under them (API.md S9, N9; DECISIONS 352). Its release frees them.
func (s *Snapshot) lockWriters(ctx context.Context, o *EditOutcome) (func() error, error) {
	reqs := writerRequests(o.Plan.Touched)
	if !s.osDisk() || len(reqs) == 0 {
		return noRelease, nil
	}
	held, err := writelock.Acquire(ctx, filepath.FromSlash(s.b.Dir()), reqs)
	if err != nil {
		return nil, err
	}
	return held.Release, nil
}

// writerRequests are the exclusive locks of pkgs, the package-less file's "" left out: it is no
// package's to lock.
func writerRequests(pkgs []string) []writelock.Request {
	var reqs []writelock.Request
	for _, p := range pkgs {
		if p != "" {
			reqs = append(reqs, writelock.Request{Package: p, Mode: writelock.Exclusive})
		}
	}
	return reqs
}

// noRelease frees no lock.
func noRelease() error { return nil }
