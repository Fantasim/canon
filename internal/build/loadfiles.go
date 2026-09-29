package build

import (
	"github.com/fantasim/canonlang/internal/source"
)

// adder is how the run's loads put a file they read into the set: the cache generation's copy of
// unchanged content, so they add nothing and a replayed value's spans stay in the set; each file
// is noted with the display this run's loads gave it.
func (h *evalHost) adder(l *readLog) func(display, abs string, data []byte) (*source.File, error) {
	set := h.loader.Set
	var g *cacheGen
	if h.sites != nil && h.sites.gen != nil && h.sites.gen.set == set {
		g = h.sites.gen
	}
	return func(display, abs string, data []byte) (*source.File, error) {
		src, err := g.loadFile(set, display, abs, data)
		if err != nil {
			return nil, err
		}
		l.added(src)
		return src, nil
	}
}

// loadFile is the generation's file of a load's content, added to set without a generation. A
// file kept from before the latest snapshot began counts toward what that snapshot uses, which
// decides compaction (grown).
func (g *cacheGen) loadFile(set *source.FileSet, display, abs string, data []byte) (*source.File, error) {
	if g == nil {
		return set.Add(display, abs, data)
	}
	src, err := g.file(display, abs, data)
	if err != nil {
		return nil, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if src.ID <= g.base {
		g.live += len(src.Content)
	}
	return src, nil
}
