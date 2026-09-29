package build

import (
	"crypto/sha256"
	"sync"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/verify"
)

// Cache is what the builds of one project's snapshots share (NFR-02): a file set holding the
// parses of unchanged files, the latest checked program's session and the evaluation memo, a
// new epoch for each full check, one along a Recheck lineage. It is safe for concurrent use.
type Cache struct {
	mu    sync.Mutex
	gen   *cacheGen
	memo  *eval.Memo
	epoch uint64 // the last epoch given
}

// cacheGen is the cache over one file set; compacting the set starts a new generation, and a
// snapshot keeps the one it began with, so the trees it holds and its set always agree.
type cacheGen struct {
	set      *source.FileSet
	reuse    *project.Reuse
	mu       sync.Mutex
	files    map[fileName]keptFile // project.canon and each canon.lock, by name
	head     *head
	sites    map[*syntax.File][]loadAt
	measured source.FileID     // the files the byte total counts
	total    int               // the bytes of every file of set
	live     int               // the bytes the latest snapshot parsed, project.canon and locks included
	base     source.FileID     // the last file of set when that snapshot began
	loads    loadSem           // held by one run's loads at a time (readLog.loading)
	vix      verify.IndexCache // verify's per-file index walks, by tree (NFR-02)
	rix      rules.IndexCache  // rules' per-file index walks, by tree (NFR-02)
}

// fileName is a file the cache keeps by name: its display path and absolute name.
type fileName struct {
	display, abs string
}

// keptFile is the source file of a name's content, by its SHA-256.
type keptFile struct {
	sum [sha256.Size]byte
	src *source.File
}

// NewCache is an empty cache.
func NewCache() *Cache {
	return &Cache{gen: newCacheGen(), memo: eval.NewMemo()}
}

func newCacheGen() *cacheGen {
	set := &source.FileSet{}
	return &cacheGen{set: set, reuse: project.NewReuse(set), files: map[fileName]keptFile{}, sites: map[*syntax.File][]loadAt{}, loads: make(loadSem, 1)}
}

// WithCache is p sharing c with every project Over makes of it; a nil c runs every call cold.
func (p *Project) WithCache(c *Cache) *Project {
	out := *p
	out.cache = c
	return &out
}

// begin is the generation a new snapshot reads into, a new one when the file set holds more
// than compactRatio times what the latest snapshot used: snapshots begun earlier keep theirs.
func (c *Cache) begin() (*cacheGen, source.FileID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	g := c.gen
	if g.grown() {
		g = newCacheGen()
		c.gen = g
	}
	return g, g.last()
}

// nextEpoch is a new epoch, for a full check.
func (c *Cache) nextEpoch() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	return c.epoch
}

// grown reports a set past compactFloor bytes that holds more than compactRatio times what
// the latest snapshot used: its parse and what it loaded since it began.
func (g *cacheGen) grown() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.measure()
	live := g.live
	for id := g.base + 1; id <= g.measured; id++ {
		live += len(g.set.Content(id))
	}
	return g.total > compactFloor && g.total > compactRatio*live
}

// measure adds the bytes of the files added to the set since the last measure.
func (g *cacheGen) measure() {
	for g.set.File(g.measured+1) != nil {
		g.measured++
		g.total += len(g.set.Content(g.measured))
	}
}

// last is the id of the set's last file.
func (g *cacheGen) last() source.FileID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.measure()
	return g.measured
}

// parsed records what a snapshot begun at base parsed: its files and their bytes.
func (g *cacheGen) parsed(base source.FileID, bytes int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.base, g.live = base, bytes
}

// file is the source file of display's content, the one kept when it did not change, so a
// new snapshot adds no copy of it to the set.
func (g *cacheGen) file(display, abs string, content []byte) (*source.File, error) {
	name, sum := fileName{display: display, abs: abs}, sha256.Sum256(content)
	g.mu.Lock()
	kept, ok := g.files[name]
	g.mu.Unlock()
	if ok && kept.sum == sum {
		return kept.src, nil
	}
	src, err := g.set.Add(display, abs, content)
	if err != nil {
		return nil, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.files[name] = keptFile{sum: sum, src: src}
	return src, nil
}
