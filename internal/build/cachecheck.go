package build

import (
	"context"
	"crypto/sha256"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// head is a lineage's latest checked program: its session, epoch, and what it checked.
type head struct {
	sess  *check.Session
	epoch uint64
	key   lineageKey
	files []*syntax.File
}

// checkKey is what a program depends on besides its files: project.canon with the roots as placed, and the layers.
type checkKey struct {
	canon  [sha256.Size]byte
	layers string
}

// lineageKey is what a lineage is kept by: its check key and the packages it checks, by name
// (log-2026-09-29 "U8": one lineage per selection).
type lineageKey struct {
	check checkKey
	pkgs  string
}

// checkInput is one phase 2: the project, its key, the files of the loaded packages, the bags
// and the folder.
type checkInput struct {
	proj  *project.Project
	key   checkKey
	files []*syntax.File
	bags  check.Bags
	fold  check.Folder
}

// check is phase 2 over in: a Recheck of the session of in's lineage when only entries of the
// files it checked changed, else a full check starting a new session and epoch, the one the
// program's evaluator then uses with the cache's memo. Each lineage keeps its own epoch.
func (g *cacheGen) check(ctx context.Context, c *Cache, in checkInput) (*check.Program, uint64) {
	key := lineageOf(in)
	g.mu.Lock()
	h := g.lineage(key)
	g.mu.Unlock()
	if changed, ok := h.changes(key, in); ok {
		if prog, next, ok := h.sess.Recheck(ctx, changed, in.bags, in.fold); ok {
			c.forget(g.advance(h, &head{sess: next, epoch: h.epoch, key: key, files: in.files}))
			return prog, h.epoch
		}
	}
	prog, sess := check.CheckSession(ctx, in.proj, in.files, in.bags, in.fold)
	if sess == nil { // cancelled: no lineage, and no epoch spent (log-2026-09-29 M4 U8-r)
		return prog, 0
	}
	epoch := c.nextEpoch()
	c.forget(g.advance(nil, &head{sess: sess, epoch: epoch, key: key, files: in.files}))
	c.retire(g, epoch)
	return prog, epoch
}

// retire forgets epoch when g, the generation its lineage advanced in, was replaced meanwhile:
// the compaction's forgetting missed it (log-2026-09-29 M4 P3-r).
func (c *Cache) retire(g *cacheGen, epoch uint64) {
	c.mu.Lock()
	dead := c.gen != g
	c.mu.Unlock()
	if dead {
		c.memo.Forget(epoch)
	}
}

// forget drops the memo's stores of epochs no lineage continues (log-2026-09-29 M4 P3-r).
func (c *Cache) forget(epochs []uint64) {
	for _, epoch := range epochs {
		c.memo.Forget(epoch)
	}
}

// epochs is the epoch of each of g's lineages.
func (g *cacheGen) epochs() []uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]uint64, len(g.heads))
	for i, h := range g.heads {
		out[i] = h.epoch
	}
	return out
}

// lineage is the head of key's lineage, nil for none; g.mu is held.
func (g *cacheGen) lineage(key lineageKey) *head {
	if i := slices.IndexFunc(g.heads, func(h *head) bool { return h.key == key }); i >= 0 {
		return g.heads[i]
	}
	return nil
}

// advance makes next the head of its lineage, the latest checked: after a Recheck only while
// from still is; a full check's lineage replaces any head of its key. Past lineageCap
// lineages, the least recently checked is dropped. It returns the epochs no head keeps.
func (g *cacheGen) advance(from, next *head) []uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	i := slices.IndexFunc(g.heads, func(h *head) bool { return h.key == next.key })
	if from != nil && (i < 0 || g.heads[i] != from) {
		return nil
	}
	var gone []uint64
	rest := slices.Clone(g.heads)
	if i >= 0 {
		gone = append(gone, rest[i].epoch)
		rest = slices.Delete(rest, i, i+1)
	}
	for _, h := range rest[min(len(rest), lineageCap-1):] {
		gone = append(gone, h.epoch)
	}
	g.heads = append([]*head{next}, rest[:min(len(rest), lineageCap-1)]...)
	return slices.DeleteFunc(gone, func(e uint64) bool { return e == next.epoch })
}

// changes is the files of in that are not the head's, when in checks the same paths under the
// same key; a Recheck then tells whether each changed only in its entries.
func (h *head) changes(key lineageKey, in checkInput) ([]*syntax.File, bool) {
	if h == nil || h.key != key || len(h.files) != len(in.files) {
		return nil, false
	}
	var changed []*syntax.File
	for i, f := range in.files {
		if f.Src.Path != h.files[i].Src.Path {
			return nil, false
		}
		if f != h.files[i] {
			changed = append(changed, f)
		}
	}
	return changed, true
}

// lineageOf is in's lineage: its check key, and the names its files' package clauses give.
func lineageOf(in checkInput) lineageKey {
	var names []string
	for _, f := range in.files {
		if f.Package == nil {
			continue
		}
		parts := make([]string, len(f.Package.Parts))
		for i, p := range f.Package.Parts {
			parts[i] = p.Name
		}
		if name := strings.Join(parts, qnameSep); len(names) == 0 || names[len(names)-1] != name {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return lineageKey{check: in.key, pkgs: strings.Join(slices.Compact(names), layerSep)}
}

// keyOf is the check key of a snapshot's project under layers.
func keyOf(s *snapshot, layers []string) checkKey {
	return checkKey{canon: s.canon, layers: strings.Join(layers, layerSep)}
}
