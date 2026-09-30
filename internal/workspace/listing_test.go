package workspace_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/workspace"
)

// fullRevision is the revision of s's read set computed whole, as before listings were kept: the
// build's inputs then every file a load read, each once, by its first display (API.md S3).
func fullRevision(s *workspace.Snapshot, inputs []build.Read, scanErr error) string {
	seen := map[string]bool{}
	var lines []build.Listed
	for _, r := range append(slices.Clone(inputs), s.Recorded()...) {
		if seen[r.Abs] {
			continue
		}
		seen[r.Abs] = true
		data, err := s.Build().FS().ReadFile(r.Abs)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			lines = append(lines, build.Listed{Display: r.Display, Unreadable: true})
		default:
			lines = append(lines, build.Listed{Display: r.Display, Sum: sha256.Sum256(data)})
		}
	}
	if scanErr != nil {
		lines = append(lines, build.Listed{Display: ".", Unreadable: true})
	}
	return build.RevisionOf(lines)
}

// sameAsFull fails the test unless s's revision, from the listing it kept or inherited, is the
// one computed whole, over the same build inputs.
func sameAsFull(t *testing.T, step string, s *workspace.Snapshot) {
	t.Helper()
	rev, err := s.Revision(context.Background())
	if err != nil {
		t.Fatalf("%s: %v", step, err)
	}
	inputs, scanErr := s.Build().Inputs()
	if listed := workspace.ListedInputs(s); !slices.Equal(listed, inputs) {
		t.Errorf("%s: the listing's inputs %v, the build's %v", step, listed, inputs)
	}
	if full := fullRevision(s, inputs, scanErr); rev != full {
		t.Errorf("%s: revision %s, computed whole %s", step, rev, full)
	}
}

// afterEdit fails the test unless an edit's After gives the revision computed whole, and the one
// a fresh project gives of the disk the edit left (log-2026-09-29 M4 PA3-r).
func afterEdit(t *testing.T, step string, fsys *memFS, after *workspace.Snapshot) {
	t.Helper()
	sameAsFull(t, step, after)
	fresh := read(t, open(t, fsys))
	analyze(t, fresh)
	sameAsFresh(t, step, after, fresh, []string{"", "a", "b", "c", "data"})
}

// API.md S3 (log-2026-09-29 M4 P18): a revision from a kept or inherited listing is the one
// computed whole, through edits, external writes, sources added, removed and renamed, a lock, a
// link retargeted, overlays and the files later loads read.
func TestListingAsFull(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	steps := []struct {
		name string
		do   func()
	}{
		{"open", func() {}},
		{"edit", func() { afterEdit(t, "edit's After", fsys, commit(t, p, setB(2)).After) }},
		{"external write", func() { write(t, fsys, "/law/a/a.json", "[7]\n") }},
		{"source added", func() { write(t, fsys, "/law/c/d.canon", "/// C.\npackage c\n") }},
		{"source renamed", func() { _ = fsys.Rename("/law/c/d.canon", "/law/c/e.canon") }},
		{"source removed", func() { _ = fsys.Remove("/law/c/e.canon") }},
		{"package added", func() { write(t, fsys, "/law/z/z.canon", "/// Z.\npackage z\n") }},
		{"lock added", func() { write(t, fsys, "/law/a/canon.lock", "canon-lock v1\n") }},
		{"link retargeted", func() { fsys.link("/law/data", "/law/a") }},
		{"loaded file changed", func() { write(t, fsys, "/law/data/c.json", "[4]\n") }},
		{"overlay", func() { _ = p.SetOverlay("a/a.json", []byte("[8]\n")) }},
		{"overlay cleared", func() { _ = p.ClearOverlay("a/a.json") }},
		{"second edit", func() { afterEdit(t, "second edit's After", fsys, commit(t, p, setB(3)).After) }},
		{"project changed", func() { write(t, fsys, "/law/project.canon", lawProject+"\n") }},
	}
	for _, step := range steps {
		step.do()
		s := read(t, p)
		sameAsFull(t, step.name+", read", s)
		analyze(t, s) // the loads add the files they read
		sameAsFull(t, step.name+", analyzed", s)
	}
}
