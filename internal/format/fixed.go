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

// layoutKey names a layout: a file judged in a role, so no verdict crosses roles (DECISIONS 258).
type layoutKey struct {
	file    weak.Pointer[syntax.File]
	project bool
}

// keyOf is the key of f judged in the role of kind.
func keyOf(f *syntax.File, kind syntax.FileKind) layoutKey {
	return layoutKey{file: weak.Make(f), project: kind == syntax.FileProject}
}

// layoutCache holds the layout of each file in each role: derived data, held weakly, never ranged over.
type layoutCache struct {
	mu    sync.Mutex
	files map[layoutKey]layout // ADR-0011, derived syntax memos
}

// layouts is the process's cache of layouts by file.
var layouts layoutCache

// Canonical reports whether the text of f is a fixed point of the formatter in the role of kind,
// judged once per file and role; the error is Rewrite's refusal of f (ErrSyntax, ErrLayout).
func Canonical(f *syntax.File, kind syntax.FileKind) (bool, error) {
	// API.md M9, FORMATTER.md §13, DECISIONS 258
	l := layouts.of(f, kind)
	return l.canonical, l.err
}

// Adopt gives f the layout of a usable, canonical tree in the role of kind, if it holds none; the
// caller has seen a tree of the same bytes judged fixed in that role (API.md M9).
func Adopt(f *syntax.File, kind syntax.FileKind) {
	layouts.keep(f, keyOf(f, kind), layout{canonical: true})
}

// of is f's layout in the role of kind, judged on the first ask only.
func (c *layoutCache) of(f *syntax.File, kind syntax.FileKind) layout {
	key := keyOf(f, kind)
	c.mu.Lock()
	found, ok := c.files[key]
	c.mu.Unlock()
	if ok {
		return found
	}
	return c.keep(f, key, judgeLayout(f, kind))
}

// keep records found as f's layout unless one is held, and returns the one held.
func (c *layoutCache) keep(f *syntax.File, key layoutKey, found layout) layout {
	c.mu.Lock()
	defer c.mu.Unlock()
	if kept, ok := c.files[key]; ok {
		return kept
	}
	if c.files == nil {
		c.files = map[layoutKey]layout{}
	}
	c.files[key] = found
	runtime.AddCleanup(f, c.forget, key)
	return found
}

// forget drops the entry of a file that no longer exists.
func (c *layoutCache) forget(key layoutKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.files, key)
}

// judgeLayout is f's layout in the role of kind, judged afresh.
func judgeLayout(f *syntax.File, kind syntax.FileKind) layout {
	if err := usable(f, kind); err != nil {
		return layout{err: err}
	}
	return layout{canonical: bytes.Equal(render(newBuilder(f).file()), f.Src.Content)}
}
