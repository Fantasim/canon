package workspace

import (
	"log/slog"
	"os"
	"sync"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
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

// Recover rolls back the unfinished edits of b's project, one Warn line on logger when it does
// (API.md O5, N11): a journal it must keep is an error wrapping edit.ErrJournal that names it
// and its files. A file system that cannot be written holds no edit to roll back.
func Recover(b *build.Project, logger *slog.Logger) error {
	w, ok := b.FS().(build.WriteFS)
	if !ok {
		return nil
	}
	return edit.Recover(edit.Site{FS: w, Layout: b, Self: self(b.Dir())}, logger)
}
