package eval

import (
	"runtime"
	"sync"
	"weak"

	"github.com/fantasim/canonlang/internal/syntax"
)

// fileMemo keeps derive(f) per file for every evaluator, weakly (ADR-0011 derived syntax memos).
type fileMemo[T any] struct {
	mu     sync.Mutex
	files  map[weak.Pointer[syntax.File]]T
	derive func(*syntax.File) T
}

// The process's per-file memos: the index's declarations and checks, and the types each file writes.
var (
	indexedFiles = &fileMemo[fileIndex]{derive: indexFile}
	refinedFiles = &fileMemo[[]typeAt]{derive: typesIn}
)

// of is derive(f), f walked on the first ask only.
func (c *fileMemo[T]) of(f *syntax.File) T {
	key := weak.Make(f)
	c.mu.Lock()
	found, ok := c.files[key]
	c.mu.Unlock()
	if ok {
		return found
	}
	found = c.derive(f)
	c.mu.Lock()
	defer c.mu.Unlock()
	if kept, ok := c.files[key]; ok {
		return kept
	}
	if c.files == nil {
		c.files = map[weak.Pointer[syntax.File]]T{}
	}
	c.files[key] = found
	runtime.AddCleanup(f, c.forget, key)
	return found
}

// forget drops the entry of a file that no longer exists.
func (c *fileMemo[T]) forget(key weak.Pointer[syntax.File]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.files, key)
}
