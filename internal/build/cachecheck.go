package build

import (
	"context"
	"crypto/sha256"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// head is a generation's latest checked program: its session, epoch, and what it checked.
type head struct {
	sess  *check.Session
	epoch uint64
	key   checkKey
	files []*syntax.File
}

// checkKey is what a program depends on besides its files: project.canon and the layers.
type checkKey struct {
	canon  [sha256.Size]byte
	layers string
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

// check is phase 2 over in: a Recheck of the head's session when only entries of the files it
// checked changed, else a full check starting a new session and epoch, the one the program's
// evaluator then uses with the cache's memo.
func (g *cacheGen) check(ctx context.Context, c *Cache, in checkInput) (*check.Program, uint64) {
	g.mu.Lock()
	h := g.head
	g.mu.Unlock()
	if changed, ok := h.changes(in); ok {
		if prog, next, ok := h.sess.Recheck(ctx, changed, in.bags, in.fold); ok {
			g.advance(h, &head{sess: next, epoch: h.epoch, key: in.key, files: in.files})
			return prog, h.epoch
		}
	}
	prog, sess := check.CheckSession(ctx, in.proj, in.files, in.bags, in.fold)
	if sess == nil { // cancelled: no lineage, and no epoch spent (log-2026-09-29 M4 U8-r)
		return prog, 0
	}
	epoch := c.nextEpoch()
	g.advance(nil, &head{sess: sess, epoch: epoch, key: in.key, files: in.files})
	return prog, epoch
}

// advance makes next the head, after a Recheck only while from still is; a full check's
// lineage replaces any head.
func (g *cacheGen) advance(from, next *head) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if from == nil || g.head == from {
		g.head = next
	}
}

// changes is the files of in that are not the head's, when in checks the same paths under the
// same key; a Recheck then tells whether each changed only in its entries.
func (h *head) changes(in checkInput) ([]*syntax.File, bool) {
	if h == nil || h.key != in.key || len(h.files) != len(in.files) {
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

// keyOf is the check key of a snapshot's project under layers.
func keyOf(s *snapshot, layers []string) checkKey {
	return checkKey{canon: s.canon, layers: strings.Join(layers, layerSep)}
}
