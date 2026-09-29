package verify

import (
	"sync"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// FileCache keeps a scan of each file's tree for the next programs of one lineage, one cache per lineage (IMPLEMENTATION-PLAN §7.6).
type FileCache[T any] struct {
	mu    sync.Mutex
	files map[*syntax.File]T
}

// Of is scan(f), computed once while c holds f; a nil c keeps nothing. Safe for concurrent use.
func (c *FileCache[T]) Of(f *syntax.File, scan func(*syntax.File) T) T {
	if c == nil {
		return scan(f)
	}
	c.mu.Lock()
	v, ok := c.files[f]
	c.mu.Unlock()
	if ok {
		return v
	}
	v = scan(f)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.files == nil {
		c.files = map[*syntax.File]T{}
	}
	c.files[f] = v
	return v
}

// KeepOnly forgets the files prog does not hold, so the cache keeps no tree it will not see again.
func (c *FileCache[T]) KeepOnly(prog *check.Program) {
	if c == nil {
		return
	}
	live := map[*syntax.File]bool{}
	if prog != nil {
		for _, pkg := range prog.Packages {
			for _, f := range pkg.Files {
				live[f] = true
			}
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for f := range c.files { //canon:unordered deleting the dead ones, in any order
		if !live[f] {
			delete(c.files, f)
		}
	}
}
