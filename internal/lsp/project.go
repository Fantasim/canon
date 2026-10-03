package lsp

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"slices"
	"sync"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/workspace"
)

// workspaceProject is the project of one project.canon: the pass opens it and sets its overlays
// from the buffers; requests read it once open (DECISIONS 285).
type workspaceProject struct {
	root    string
	hooks   projectHooks
	read    func() buffers    // the buffers now, read under the turn
	turn    chan struct{}     // held by the one syncing it: a lock whose wait a ctx ends
	applied map[string][]byte // the overlays set, by absolute name; the turn's holder's
	gen     uint64            // the generation of the buffers applied; the turn's holder's

	mu     sync.Mutex
	ws     *workspace.Project // nil until it opens
	failed error              // why the last open failed: a *build.OpenError carries findings
}

// projectHooks are a test's: called as the project opens, and as the pass syncs it.
type projectHooks struct {
	opening func()
	syncing func(ctx context.Context)
}

func newProject(root string, hooks projectHooks, read func() buffers) *workspaceProject {
	return &workspaceProject{root: root, hooks: hooks, read: read, turn: make(chan struct{}, 1), applied: map[string][]byte{}}
}

// take waits for the turn to sync the project, or for ctx to end.
func (p *workspaceProject) take(ctx context.Context) error {
	select {
	case p.turn <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *workspaceProject) give() { <-p.turn }

// sync is the pass's: the project opened over the buffers first, so a project.canon fixed in its
// buffer opens, then each buffer an overlay. Only the pass opens: never a notification or a
// request, so the reading goroutine never runs build.Open (DECISIONS 285).
func (p *workspaceProject) sync(ctx context.Context) error {
	if err := p.take(ctx); err != nil {
		return err
	}
	defer p.give()
	if p.hooks.syncing != nil {
		p.hooks.syncing(ctx)
	}
	b := p.read()
	if ws, _ := p.current(); ws == nil && !p.open(b.docs) {
		return nil
	}
	return p.overlay(ctx, b)
}

// opened is the project synced to the buffers for a request, under its ctx; nil before the pass
// first opened it (DECISIONS 285).
func (p *workspaceProject) opened(ctx context.Context) (*workspace.Project, error) {
	if ws, _ := p.current(); ws == nil {
		return nil, nil
	}
	if err := p.take(ctx); err != nil {
		return nil, err
	}
	defer p.give()
	if err := p.overlay(ctx, p.read()); err != nil {
		return nil, err
	}
	ws, _ := p.current()
	return ws, nil
}

// open opens the project over the buffers, then the disk; false and failed set when it cannot.
// The turn is held.
func (p *workspaceProject) open(docs map[string][]byte) bool {
	if p.hooks.opening != nil {
		p.hooks.opening()
	}
	fsys := newBufferFS(build.OS(), docs)
	b, err := build.Open(fsys, p.root, build.Options{})
	fsys.release()
	p.mu.Lock()
	defer p.mu.Unlock()
	if err != nil {
		p.failed = err
		return false
	}
	p.ws, p.failed = workspace.New(b), nil
	return true
}

// overlay makes the overlays the buffers, changed ones set, closed ones cleared, unless they are
// older than those applied (DECISIONS 285); the turn is held.
func (p *workspaceProject) overlay(ctx context.Context, b buffers) error {
	if b.gen < p.gen {
		return nil
	}
	docs := b.docs
	ws, _ := p.current()
	for _, abs := range slices.Sorted(maps.Keys(docs)) {
		if old, ok := p.applied[abs]; ok && bytes.Equal(old, docs[abs]) {
			continue
		}
		if err := ws.SetOverlayContext(ctx, abs, docs[abs]); err != nil {
			return err
		}
		p.applied[abs] = docs[abs]
	}
	for _, abs := range slices.Sorted(maps.Keys(p.applied)) {
		if _, open := docs[abs]; open {
			continue
		}
		if err := ws.ClearOverlayContext(ctx, abs); err != nil {
			return err
		}
		delete(p.applied, abs)
	}
	p.gen = b.gen
	return nil
}

// current is the workspace, or nil and why it could not be opened.
func (p *workspaceProject) current() (*workspace.Project, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ws, p.failed
}

// checkSynced is check once the project is synced to its buffers.
func (p *workspaceProject) checkSynced(ctx context.Context) (map[string][]diagnostic, error) {
	if err := p.sync(ctx); err != nil {
		return nil, err
	}
	return p.check(ctx)
}

// check is every finding of every package, by file (API.md R2); a project.canon in error gives
// its own findings alone (API.md W13).
func (p *workspaceProject) check(ctx context.Context) (map[string][]diagnostic, error) {
	ws, failed := p.current()
	if ws == nil {
		return openFailed(failed, p.root)
	}
	snap, err := ws.Read(ctx)
	if err != nil {
		return nil, err
	}
	a, err := workspace.Analyze(ctx, snap, nil)
	if err != nil {
		return openFailed(err, p.root)
	}
	return diagnosticsOf(a.Result().Findings, p.root)
}

// openFailed is the findings of a project.canon in error, else err itself.
func openFailed(err error, root string) (map[string][]diagnostic, error) {
	var oe *build.OpenError
	if errors.As(err, &oe) {
		return diagnosticsOf(oe.Findings, root)
	}
	return nil, err
}

// close closes the project's workspace, if open.
func (p *workspaceProject) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ws != nil {
		p.ws.Close()
	}
}

// close closes every project; the pass goroutine has stopped.
func (s *server) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	//canon:unordered each project is closed on its own
	for _, p := range s.projects {
		p.close()
	}
}
