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

// workspaceProject is the project of one project.canon: the handlers open it and set its
// overlays from the buffers, the pass reads it.
type workspaceProject struct {
	root    string
	mu      sync.Mutex
	ws      *workspace.Project // nil while it cannot be opened
	failed  error              // why the last open failed: a *build.OpenError carries findings
	applied map[string][]byte  // the overlays set, by absolute name
}

func newProject(root string) *workspaceProject {
	return &workspaceProject{root: root, applied: map[string][]byte{}}
}

// sync makes the project match the buffers: opened over them first, so a project.canon fixed in
// its buffer opens, then each buffer an overlay. In-memory writes, which take no ctx.
func (p *workspaceProject) sync(docs map[string][]byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ws == nil && !p.open(docs) {
		return nil
	}
	return p.overlay(docs)
}

// open opens the project over the buffers, then the disk; false and failed set when it cannot.
func (p *workspaceProject) open(docs map[string][]byte) bool {
	fsys := newBufferFS(build.OS(), docs)
	b, err := build.Open(fsys, p.root, build.Options{})
	fsys.release()
	if err != nil {
		p.failed = err
		return false
	}
	p.ws, p.failed = workspace.New(b), nil
	return true
}

// overlay makes the overlays the buffers, changed ones set, closed ones cleared; p.mu is held.
func (p *workspaceProject) overlay(docs map[string][]byte) error {
	for _, abs := range slices.Sorted(maps.Keys(docs)) {
		if old, ok := p.applied[abs]; ok && bytes.Equal(old, docs[abs]) {
			continue
		}
		if err := p.ws.SetOverlay(abs, docs[abs]); err != nil {
			return err
		}
		p.applied[abs] = docs[abs]
	}
	for _, abs := range slices.Sorted(maps.Keys(p.applied)) {
		if _, open := docs[abs]; open {
			continue
		}
		if err := p.ws.ClearOverlay(abs); err != nil {
			return err
		}
		delete(p.applied, abs)
	}
	return nil
}

// current is the workspace, or nil and why it could not be opened.
func (p *workspaceProject) current() (*workspace.Project, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ws, p.failed
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
