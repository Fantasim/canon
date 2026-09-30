package build

import (
	"crypto/sha256"

	"github.com/fantasim/canonlang/internal/source"
)

// adder is how the run's loads put a file they read into the set: the cache generation's copy of
// unchanged content, so they add nothing and a replayed value's spans stay in the set; each file
// is noted with the display this run's loads gave it.
func (h *evalHost) adder(l *readLog) func(display, abs string, data []byte, sum [sha256.Size]byte) (*source.File, error) {
	set, g := h.loader.Set, h.loadGen()
	return func(display, abs string, data []byte, sum [sha256.Size]byte) (*source.File, error) {
		l.keep(display, abs, sum)
		src, err := g.loadFile(set, fileName{display: display, abs: abs}, data, sum)
		if err != nil {
			return nil, err
		}
		l.added(src)
		return src, nil
	}
}

// keeper is the adder of a file the run's loads did not read, its SHA-256 known (load.Loader.Kept,
// log-2026-09-29 P18): the generation's copy of that content, noted as adder notes a file; nil
// without a generation, false when it holds no such content.
func (h *evalHost) keeper(l *readLog) func(display, abs string, sum [sha256.Size]byte) (*source.File, bool) {
	g := h.loadGen()
	if g == nil {
		return nil
	}
	return func(display, abs string, sum [sha256.Size]byte) (*source.File, bool) {
		src, ok := g.keptOf(fileName{display: display, abs: abs}, sum)
		if !ok {
			return nil, false
		}
		l.keep(display, abs, sum)
		g.used(src)
		l.added(src)
		return src, true
	}
}

// loadGen is the cache generation whose set the run's loads add to, nil for none.
func (h *evalHost) loadGen() *cacheGen {
	if h.sites != nil && h.sites.gen != nil && h.sites.gen.set == h.loader.Set {
		return h.sites.gen
	}
	return nil
}

// loadFile is the generation's file of a load's content, added to set without a generation.
func (g *cacheGen) loadFile(set *source.FileSet, name fileName, data []byte, sum [sha256.Size]byte) (*source.File, error) {
	if g == nil {
		return set.Add(name.display, name.abs, data)
	}
	src, err := g.fileSum(name, data, sum)
	if err != nil {
		return nil, err
	}
	g.used(src)
	return src, nil
}

// used counts a file kept from before the latest snapshot began toward what that snapshot uses,
// which decides compaction (grown).
func (g *cacheGen) used(src *source.File) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if src.ID <= g.base {
		g.live += len(src.Content)
	}
}
