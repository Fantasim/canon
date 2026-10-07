package workspace_test

import (
	"context"
	"crypto/sha256"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/workspace"
)

const (
	lawProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    data: \"data\"\n  }\n}\n"
	srcA       = "/// A.\npackage a\n\n/// Xs.\nlet xs: [Int] = load(\"a.json\")\n"
	srcB       = "/// B.\npackage b\n\nimport a\n\n/// N.\nconst N = 1\n"
	srcC       = "/// C.\npackage c\n\n/// Ys.\nlet ys: [Int] = load(\"@data/c.json\")\n"
)

// lawFiles is three packages: a loads a file beside it, b imports a, c loads through a root.
func lawFiles() map[string]string {
	return map[string]string{
		"/law/project.canon": lawProject, "/law/a/a.canon": srcA, "/law/a/a.json": "[1, 2]\n",
		"/law/b/b.canon": srcB, "/law/c/c.canon": srcC, "/law/data/c.json": "[3]\n",
	}
}

func open(t *testing.T, fsys *memFS) *workspace.Project {
	t.Helper()
	b, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := workspace.New(b)
	t.Cleanup(p.Close)
	return p
}

func read(t *testing.T, p *workspace.Project) *workspace.Snapshot {
	t.Helper()
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// analyze is every package analyzed on s, as a read shares it.
func analyze(t *testing.T, s *workspace.Snapshot) *build.Analysis {
	t.Helper()
	ctx := context.Background()
	a, err := workspace.Share(ctx, s, workspace.Key(workspace.OpAnalyze, nil), func(ctx context.Context) (*build.Analysis, error) {
		return s.Build().Analyze(ctx, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func revision(t *testing.T, s *workspace.Snapshot) string {
	t.Helper()
	rev, err := s.Revision(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return rev
}

// API.md S1: a file whose stat is unchanged is not read again; a write makes a new snapshot; a
// file written again with the same bytes keeps the snapshot and its revision.
func TestRefreshByStat(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	s := read(t, p)
	analyze(t, s)
	rev := revision(t, s)
	reads := fsys.readsOf("/law/a/a.canon")
	for range 3 {
		if again := read(t, p); again != s {
			t.Fatal("an unchanged disk made a new snapshot")
		}
		analyze(t, s)
	}
	if n := fsys.readsOf("/law/a/a.canon"); n != reads {
		t.Errorf("a/a.canon read %d times, want %d: its stat did not change", n, reads)
	}
	_ = fsys.WriteFile("/law/a/a.canon", []byte(srcA))
	if again := read(t, p); again != s || revision(t, again) != rev {
		t.Error("the same bytes written again made a new snapshot")
	}
	_ = fsys.WriteFile("/law/b/b.canon", []byte(srcB+"\n"))
	next := read(t, p)
	if next == s || revision(t, next) == rev || revision(t, s) != rev {
		t.Error("a changed file left the snapshot, or changed the old one's revision")
	}
}

// API.md S1: a file system without modification times is read again at every refresh, and a
// change of the same size is seen.
func TestRefreshZeroModTime(t *testing.T) {
	fsys := newMemFS(lawFiles())
	fsys.zero = true
	p := open(t, fsys)
	s := read(t, p)
	analyze(t, s)
	rev := revision(t, s)
	before := fsys.readsOf("/law/b/b.canon")
	if again := read(t, p); again != s || fsys.readsOf("/law/b/b.canon") != before+1 {
		t.Errorf("zero mtime: %d reads, want %d, same snapshot %v", fsys.readsOf("/law/b/b.canon"), before+1, again == s)
	}
	_ = fsys.WriteFile("/law/b/b.canon", []byte(srcB[:len(srcB)-2]+"2\n"))
	if next := read(t, p); next == s || revision(t, next) == rev {
		t.Error("a same-size change under a zero mtime was missed")
	}
}

// API.md S1: a directory the scan listed is listed again when it changes: a new source is seen.
func TestRefreshListing(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	s := read(t, p)
	units, err := s.Build().Packages(context.Background())
	if err != nil || len(units.Units) != 3 {
		t.Fatalf("Packages: %v", err)
	}
	_ = fsys.WriteFile("/law/d/d.canon", []byte("/// D.\npackage d\n"))
	next := read(t, p)
	if units, err = next.Build().Packages(context.Background()); err != nil || len(units.Units) != 4 {
		t.Errorf("after a new package: %v, %d units", err, len(units.Units))
	}
}

// API.md S3 (DECISIONS 330): the revision lists project.canon, every source and existing lock, and
// every file a load names, by display path and SHA-256, the same before and after an analysis;
// a change of a loaded file changes it.
func TestRevisionReadSet(t *testing.T) {
	files := lawFiles()
	files["/law/a/canon.lock"] = "canon-lock v1\n"
	fsys := newMemFS(files)
	p := open(t, fsys)
	s := read(t, p)
	scanOnly := revision(t, s)
	analyze(t, s)
	rev := revision(t, s)
	var lines []build.Listed
	for _, display := range []string{"project.canon", "a/a.canon", "b/b.canon", "c/c.canon", "a/canon.lock", "a/a.json", "@data/c.json"} {
		abs := "/law/" + display
		if display == "@data/c.json" {
			abs = "/law/data/c.json"
		}
		lines = append(lines, build.Listed{Display: display, Sum: sha256.Sum256([]byte(files[abs]))})
	}
	if want := build.RevisionOf(lines); rev != want || scanOnly != rev {
		t.Errorf("revision %s, want %s (before the loads: %s)", rev, want, scanOnly)
	}
	_ = fsys.WriteFile("/law/data/c.json", []byte("[4]\n"))
	if next := read(t, p); revision(t, next) == rev {
		t.Error("a loaded file changed and the revision did not")
	}
}

// API.md §3.4, S3: an overlay is read, listed and revised, never written; clearing it restores.
func TestOverlay(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	var events []workspace.Event
	defer p.Subscribe(func(e workspace.Event) { events = append(events, e) })()
	rev := revision(t, read(t, p))
	if err := p.SetOverlay("b/b.canon", []byte(srcB+"\n")); err != nil {
		t.Fatal(err)
	}
	if err := p.SetOverlay("/law/e/e.canon", []byte("/// E.\npackage e\n")); err != nil {
		t.Fatal(err)
	}
	s := read(t, p)
	data, err := s.Build().FS().ReadFile("/law/b/b.canon")
	units, uerr := s.Build().Packages(context.Background())
	if err != nil || string(data) != srcB+"\n" || uerr != nil || len(units.Units) != 4 || revision(t, s) == rev {
		t.Errorf("overlay: %q, %v; %v units, %v", data, err, units, uerr)
	}
	if disk, _ := fsys.ReadFile("/law/b/b.canon"); string(disk) != srcB {
		t.Error("an overlay was written")
	}
	for _, file := range []string{"b/b.canon", "/law/e/e.canon", "b/b.canon"} {
		if err := p.ClearOverlay(file); err != nil {
			t.Fatal(err)
		}
	}
	if back := revision(t, read(t, p)); back != rev {
		t.Errorf("revision after clearing %s, want %s", back, rev)
	}
	causes := []workspace.Cause{workspace.CauseOverlay, workspace.CauseOverlay, workspace.CauseOverlay, workspace.CauseOverlay}
	if got := causesOf(events); !slices.Equal(got, causes) || !slices.Equal(events[0].Files, []string{"b/b.canon"}) {
		t.Errorf("events %v, want %v (a clear of nothing publishes nothing)", got, causes)
	}
	if err := p.SetOverlay("../out.canon", nil); err == nil {
		t.Error("an overlay outside the project was accepted")
	}
}

func causesOf(events []workspace.Event) []workspace.Cause {
	var out []workspace.Cause
	for _, e := range events {
		out = append(out, e.Cause)
	}
	return out
}
