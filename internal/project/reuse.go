package project

import (
	"maps"
	"sync"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Reuse keeps one project's parses between snapshots by path and content SHA-256 (NFR-02).
type Reuse struct {
	set   *source.FileSet // the persistent, append-only set every kept file lives in
	mu    sync.Mutex
	files map[string]entry // by display path, so one Reuse serves one project Dir: Abs is the first parse's
}

// entry is the parse of one path and the content it was parsed from.
type entry struct {
	sum sha256Sum
	parsedFile
}

// parsedFile is one file read and parsed: its source file, its tree, the package its package
// line names ("" for none) and whether parsing it reported findings. reused is set on the
// copies a Reuse hands back.
type parsedFile struct {
	src      *source.File
	file     *syntax.File
	pkg      string
	findings bool
	reused   bool
}

// NewReuse is an empty store for readers whose Set is set.
func NewReuse(set *source.FileSet) *Reuse {
	return &Reuse{set: set, files: map[string]entry{}}
}

// lookup is the parse of a file when its content still has the given sum; a nil Reuse holds none.
func (u *Reuse) lookup(name string, sum sha256Sum) (parsedFile, bool) {
	if u == nil {
		return parsedFile{}, false
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	e, ok := u.files[name]
	if !ok || e.sum != sum {
		return parsedFile{}, false
	}
	e.reused = true
	return e.parsedFile, true
}

// store records the parse of a file, replacing the one of an older content.
func (u *Reuse) store(name string, sum sha256Sum, p parsedFile) {
	if u == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.files[name] = entry{sum: sum, parsedFile: p}
}

// retain forgets every file but names: one gone from the project is not brought back by an
// older snapshot's entry.
func (u *Reuse) retain(names []string) {
	if u == nil {
		return
	}
	live := make(map[string]struct{}, len(names))
	for _, name := range names {
		live[name] = struct{}{}
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	maps.DeleteFunc(u.files, func(name string, _ entry) bool {
		_, ok := live[name]
		return !ok
	})
}
