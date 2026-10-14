package workspace

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/writelock"
)

// running counts the commits this process runs, by project directory: a journal it wrote is a
// running writer's only while one runs there.
type running struct {
	sync.Mutex
	in map[string]int
}

var commits running

// committing counts a commit in dir until the returned function runs, once.
func committing(dir string) func() {
	commits.Lock()
	if commits.in == nil {
		commits.in = map[string]int{}
	}
	commits.in[dir]++
	commits.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			commits.Lock()
			defer commits.Unlock()
			if commits.in[dir]--; commits.in[dir] == 0 {
				delete(commits.in, dir)
			}
		})
	}
}

// self is this process as a journal of dir records it: its host ("" when the system names
// none), its pid, and whether a writer of this host runs, this process only while it commits
// in dir, so the next Open recovers a journal it left.
func self(dir string) edit.Process {
	host, err := os.Hostname()
	if err != nil {
		host = ""
	}
	pid := os.Getpid()
	return edit.Process{Host: host, PID: pid, Alive: func(p int) bool {
		if p != pid {
			return alive(p)
		}
		commits.Lock()
		defer commits.Unlock()
		return commits.in[dir] > 0
	}}
}

// Recover rolls back b's unfinished edits, one Warn on logger (API.md O5, N11): a journal with locks
// only taking them all without waiting (DECISIONS 375: J3); one it must keep is an error wrapping
// edit.ErrJournal that names it and its files. A file system that cannot be written holds none.
func Recover(b *build.Project, logger *slog.Logger) error {
	w, ok := b.FS().(build.WriteFS)
	if !ok {
		return nil
	}
	return edit.Recover(edit.Site{FS: w, Layout: b, Self: self(b.Dir()), Locks: lockerFor(w, b.Dir())}, logger)
}

// withSettle is b settling, before a build writes, the journals its locks meet, on the OS's
// files where the platform has writer locks; b itself elsewhere (DECISIONS 375: J10, J12).
func withSettle(b *build.Project, logger *slog.Logger) *build.Project {
	site, ok := diskSite(b)
	if !ok {
		return b
	}
	return b.WithSettle(settler{site: site, logger: logger})
}

// diskSite is b's project on its own file system with its Locker; false where it takes no lock.
func diskSite(b *build.Project) (edit.Site, bool) {
	w, ok := b.FS().(build.WriteFS)
	locker := lockerFor(b.FS(), b.Dir())
	if !ok || locker == nil {
		return edit.Site{}, false
	}
	return edit.Site{FS: w, Layout: b, Self: self(b.Dir()), Locks: locker}, true
}

// LockAndSettle takes reqs, b's writer locks, waiting with ctx, grown to the journals of this host
// they meet, and rolls those back (DECISIONS 375: J4, J10): changed are the files it restored or
// removed; release frees the locks. A file system that takes no writer lock settles nothing.
func LockAndSettle(ctx context.Context, b *build.Project, reqs []writelock.Request, logger *slog.Logger) (release func() error, changed []string, err error) {
	site, ok := diskSite(b)
	if !ok {
		release, err := plainAcquire(ctx, b.FS(), b.Dir(), reqs)
		return release, nil, err
	}
	held, reqs, err := acquireGrown(ctx, &site, b.Dir(), strongest(reqs))
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if release == nil {
			err = errors.Join(err, held.Release())
		}
	}()
	changed, err = settle(site, reqs, logger)
	if err != nil {
		return nil, changed, err
	}
	return held.Release, changed, nil
}
