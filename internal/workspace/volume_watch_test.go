package workspace_test

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/workspace"
)

// API.md W12, W14 on a UNC project directory: a created and a removed file are reported by display path and by the absolute names of their directories and themselves.
func TestWatchOnUNCVolume(t *testing.T) {
	onVolumes(t)
	fsys := newMemFS(uncFiles())
	p := openUNC(t, fsys)
	s := read(t, p)
	analyze(t, s)
	revision(t, s)
	clk := newFakeClock()
	changes, _, _ := startWatch(t, p, clk)
	clk.idle(t)
	write(t, fsys, uncRoot+"/a/more.canon", "/// A.\npackage a\n")
	_ = fsys.Remove(uncRoot + "/b/b.canon")
	for range 3 {
		clk.step(t, poll)
	}
	c := next(t, changes)
	if got := shown(c); !slices.Contains(got, "a/more.canon") || !slices.Contains(got, "b/b.canon") {
		t.Errorf("files %v", got)
	}
	for _, want := range []string{uncRoot + "/a", uncRoot + "/b/b.canon"} {
		if !slices.Contains(c.Changed, want) {
			t.Errorf("changed %v lacks %s", c.Changed, want)
		}
	}
}

// API.md S9 on a UNC project directory: an older snapshot that has read nothing yet still sees a directory a writer creates as missing, and its parent's listing without it.
func TestPinOnUNCVolume(t *testing.T) {
	onVolumes(t)
	p := openUNC(t, newMemFS(uncFiles()))
	old := read(t, p)
	_, err := p.Write(context.Background(), workspace.CauseEdit, func(_ context.Context, s *workspace.Snapshot) error {
		w := s.Build().FS().(writeFS)
		if err := w.MkdirAll(uncRoot + "/x/y"); err != nil {
			return err
		}
		return w.WriteFile(uncRoot+"/x/y/z.txt", []byte("z\n"))
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Build().FS().Stat(uncRoot + "/x"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the older snapshot sees the new directory: %v", err)
	}
	list, err := old.Build().FS().ReadDir(uncRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range list {
		if e.Name() == "x" {
			t.Error("the older snapshot lists the new directory")
		}
	}
}
