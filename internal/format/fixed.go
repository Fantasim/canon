package format

import (
	"bytes"
	"runtime"
	"sync"
	"weak"

	"github.com/fantasim/canonlang/internal/syntax"
)

// layout is what a file's text is to the formatter: an error when Rewrite refuses it, else
// whether it is a fixed point.
type layout struct {
	err       error
	canonical bool
}

// layoutCache holds the layout of each file: data derived only from an immutable file, held
// weakly and never ranged over; it changes no output, only time.
type layoutCache struct {
	mu    sync.Mutex
	files map[weak.Pointer[syntax.File]]layout // ADR-0011, derived syntax memos
}

// layouts is the process's cache of layouts by file.
var layouts layoutCache

// Canonical reports whether the text of f is a fixed point of the formatter, judged once per
// file; the error is Rewrite's refusal of f (ErrSyntax, ErrLayout).
func Canonical(f *syntax.File) (bool, error) {
	// API.md M9, FORMATTER.md §13
	l := layouts.of(f)
	return l.canonical, l.err
}

// Adopt gives f the layout of a usable, canonical tree; one already held is left as it is. The caller
// has seen a tree of the same bytes and parse kind judged fixed by Canonical (API.md M9).
func Adopt(f *syntax.File) {
	layouts.keep(f, weak.Make(f), layout{canonical: true})
}

// of is f's layout, judged on the first ask only.
func (c *layoutCache) of(f *syntax.File) layout {
	key := weak.Make(f)
	c.mu.Lock()
	found, ok := c.files[key]
	c.mu.Unlock()
	if ok {
		return found
	}
	return c.keep(f, key, judgeLayout(f))
}

// keep records found as f's layout unless one is held, and returns the one held.
func (c *layoutCache) keep(f *syntax.File, key weak.Pointer[syntax.File], found layout) layout {
	c.mu.Lock()
	defer c.mu.Unlock()
	if kept, ok := c.files[key]; ok {
		return kept
	}
	if c.files == nil {
		c.files = map[weak.Pointer[syntax.File]]layout{}
	}
	c.files[key] = found
	runtime.AddCleanup(f, c.forget, key)
	return found
}

// forget drops the entry of a file that no longer exists.
func (c *layoutCache) forget(key weak.Pointer[syntax.File]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.files, key)
}

// judgeLayout is f's layout, judged afresh.
func judgeLayout(f *syntax.File) layout {
	if err := usable(f); err != nil {
		return layout{err: err}
	}
	return layout{canonical: bytes.Equal(render(newBuilder(f).file()), f.Src.Content)}
}
