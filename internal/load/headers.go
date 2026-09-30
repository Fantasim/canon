package load

import (
	"sync"

	"github.com/fantasim/canonlang/internal/source"
)

// Headers keeps, per file name, the last header classified without a finding, while its source
// file is the same: the file is still read, only its classification is kept (NFR-02).
type Headers struct {
	mu   sync.Mutex
	kept map[headerName]classified
	made int // the classifications kept, for tests
}

// headerName is a header the cache keeps by name: its display path and absolute name.
type headerName struct {
	display, abs string
}

// classified is a header's source file and its #defines classified, in file order.
type classified struct {
	src     *source.File
	defs    []headerDefine
	skipped []skippedDefine
	ok      bool
}

// classify is src's classification: the one kept for its name when it is the same file, else
// classified into req's bag and, when it reported nothing and req is not scratch, kept. A nil
// Headers keeps nothing.
func (h *Headers) classify(src *source.File, req Request) classified {
	if h == nil {
		return classifyHeader(src, req)
	}
	name := headerName{display: src.Path, abs: src.Abs}
	h.mu.Lock()
	kept, ok := h.kept[name]
	h.mu.Unlock()
	if ok && kept.src == src {
		return kept
	}
	c := classifyHeader(src, req)
	if !c.ok || req.Scratch { // an E7102 is reported by a real run each time; a scratch request caches nothing
		return c
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.kept == nil {
		h.kept = map[headerName]classified{}
	}
	h.kept[name] = c
	h.made++
	return c
}

// Made is how many classifications h has kept.
func (h *Headers) Made() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.made
}

// classifyHeader classifies src's #defines, reporting into req's bag (WIRE.md §6.8).
func classifyHeader(src *source.File, req Request) classified {
	defs, skipped, ok := readDefines(src, req)
	return classified{src: src, defs: defs, skipped: skipped, ok: ok}
}
