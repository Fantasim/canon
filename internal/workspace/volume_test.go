package workspace_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/workspace"
)

// uncRoot is a project on a network share: its names carry a volume the '/' walk must not climb above (API.md §2.2).
const (
	uncShare = "//server/share/"
	uncRoot  = uncShare + "law"
)

// onVolumes skips a test on a host whose names have no volume: there "//server/share" is a path.
func onVolumes(t *testing.T) {
	t.Helper()
	if project.HostPaths().Volume(uncRoot) == "" {
		t.Skip("the host's names carry no volume")
	}
}

// uncFiles is lawFiles on the share.
func uncFiles() map[string]string {
	out := map[string]string{}
	for name, data := range lawFiles() {
		out[uncRoot+strings.TrimPrefix(name, "/law")] = data
	}
	return out
}

// strayNames is every file and directory of fsys outside the share: a name a walk or a join lost
// its volume in.
func strayNames(fsys *memFS) []string {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()
	var out []string
	for name := range fsys.files {
		if !strings.HasPrefix(name, uncShare) {
			out = append(out, name)
		}
	}
	for name := range fsys.dirs {
		if !strings.HasPrefix(name, uncShare) && name != "/" { // the FS's own seed
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

func openUNC(t *testing.T, fsys *memFS) *workspace.Project {
	t.Helper()
	b, err := build.Open(fsys, uncRoot, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := workspace.New(b)
	t.Cleanup(p.Close)
	return p
}

// API.md §2.2, S1, S3, N10, W15 on a UNC project directory: an edit commits its files through the journal beside project.canon, publishes their display paths, and a disk change is seen.
func TestEditOnUNCVolume(t *testing.T) {
	onVolumes(t)
	fsys := newMemFS(uncFiles())
	p := openUNC(t, fsys)
	var events []workspace.Event
	defer p.Subscribe(func(e workspace.Event) { events = append(events, e) })()
	out, err := p.Edit(context.Background(), workspace.EditRequest{Changes: twoFiles(), Normalize: true})
	if err != nil || !out.Applied || out.Checked.Summary.Errors != 0 {
		t.Fatalf("Edit: %+v, %v", out, err)
	}
	if len(events) != 1 || !slices.Equal(events[0].Files, []string{"a/a.json", "b/b.canon"}) {
		t.Fatalf("API.md W15: events %+v", events)
	}
	if data, _ := fsys.ReadFile(uncRoot + "/a/a.json"); strings.Join(strings.Fields(string(data)), "") != "[5,2]" {
		t.Errorf("a.json %q", data)
	}
	if entries, err := fsys.ReadDir(uncRoot + "/.canon/journal"); err != nil || len(entries) != 0 {
		t.Errorf("API.md N10: the journal is left: %v, %v", entries, err)
	}
	if stray := strayNames(fsys); len(stray) != 0 {
		t.Errorf("API.md §2.2: names outside the share: %v", stray)
	}
	_ = fsys.WriteFile(uncRoot+"/d/d.canon", []byte("/// D.\npackage d\n"))
	next := read(t, p)
	units, err := next.Build().Packages(context.Background())
	if err != nil || len(units.Units) != 4 || next == out.After {
		t.Errorf("API.md S1: a new package on the share: %v, %d units", err, len(units.Units))
	}
}

// API.md §3.4, S2 on a UNC project directory: an overlay keyed by a display path changes the revision, and clearing it restores it.
func TestOverlayOnUNCVolume(t *testing.T) {
	onVolumes(t)
	p := openUNC(t, newMemFS(uncFiles()))
	rev := revision(t, read(t, p))
	if err := p.SetOverlay("b/b.canon", []byte(srcB+"\n")); err != nil {
		t.Fatal(err)
	}
	if revision(t, read(t, p)) == rev {
		t.Error("an overlay on the share left the revision")
	}
	if err := p.SetOverlay("e/e.canon", []byte("/// E.\npackage e\n")); err != nil {
		t.Fatal(err)
	}
	units, err := read(t, p).Build().Packages(context.Background())
	if err != nil || len(units.Units) != 4 {
		t.Errorf("a new package by overlay: %v, %d units", err, len(units.Units))
	}
	if err := p.ClearOverlay(uncRoot + "/e/e.canon"); err != nil {
		t.Fatal(err)
	}
	if err := p.ClearOverlay(uncRoot + "/b/b.canon"); err != nil {
		t.Fatal(err)
	}
	if revision(t, read(t, p)) != rev {
		t.Error("clearing the overlay by its absolute name did not restore the revision")
	}
}

// staleAfterEdit is the files a stale edit names once a.json and a new package changed on disk
// under root, as base had it.
func staleAfterEdit(t *testing.T, root string) []string {
	t.Helper()
	files := map[string]string{}
	for name, data := range lawFiles() {
		files[root+strings.TrimPrefix(name, "/law")] = data
	}
	fsys := newMemFS(files)
	b, err := build.Open(fsys, root, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := workspace.New(b)
	t.Cleanup(p.Close)
	base := revision(t, read(t, p))
	_ = fsys.WriteFile(root+"/a/a.json", []byte("[7]\n"))
	_ = fsys.WriteFile(root+"/d/d.canon", []byte("/// D.\npackage d\n"))
	_, err = p.Edit(context.Background(), workspace.EditRequest{Changes: twoFiles(), Base: base})
	got, stale := staleFiles(err)
	if !stale {
		t.Fatalf("Edit on a changed disk: %v, want a stale error", err)
	}
	return got
}

// API.md S5, S6 on a UNC project directory: a stale edit names the same files as on a plain one.
func TestStaleOnUNCVolume(t *testing.T) {
	onVolumes(t)
	want := staleAfterEdit(t, "/law")
	if len(want) == 0 {
		t.Fatal("the plain project's stale error names no file")
	}
	if got := staleAfterEdit(t, uncRoot); !slices.Equal(got, want) {
		t.Errorf("stale on the share names %v, want %v", got, want)
	}
}
