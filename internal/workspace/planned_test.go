package workspace_test

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
)

// setB sets b's constant to n.
func setB(n int64) workspace.Changes {
	return workspace.Changes{Host: build.EditHost, Ops: []edit.Operation{{Kind: edit.OpSet, Path: "b:N", Value: edit.Int(n)}}}
}

// commit is one applied edit of p, failing the test otherwise.
func commit(t *testing.T, p *workspace.Project, c workspace.Changes) *workspace.EditOutcome {
	t.Helper()
	out, err := p.Edit(context.Background(), workspace.EditRequest{Changes: c})
	if err != nil || !out.Applied {
		t.Fatalf("Edit: %+v, %v", out, err)
	}
	return out
}

// API.md S10, E18, V13 (log-2026-09-29 M4 P14): an edit publishes the snapshot its re-check
// analyzed, which keeps that analysis for an evaluation of the edited package. (The first edit
// creates the journal's directory, a change the refresh after it reads.)
func TestEditPublishesItsRecheck(t *testing.T) {
	p := open(t, newMemFS(lawFiles()))
	commit(t, p, setB(2))
	out := commit(t, p, setB(3))
	ctx := context.Background()
	touched, err := workspace.Touching(ctx, out.After, "b")
	if err != nil || !slices.Equal(touched, []string{"b"}) {
		t.Fatalf("API.md E17: b touches %v, %v", touched, err)
	}
	a, err := workspace.Analyze(ctx, read(t, p), touched)
	if err != nil || a.Result() != out.Checked {
		t.Fatalf("API.md E18: the published snapshot analyzes b again: %v", err)
	}
	again, err := workspace.Analyze(ctx, read(t, p), touched)
	if err != nil || again != a {
		t.Fatalf("NFR-02: a snapshot keeps its analysis: %v", err)
	}
	if !workspace.Covered(read(t, p), touched) {
		t.Error("log-2026-09-29 M4 P14-r: within the budget, b's analysis does not give every package's values")
	}
}

// NFR-02 (log-2026-09-29 M4 P14-r): a snapshot keeps every package's analysis and the latest of
// one selection, which another replaces.
func TestKeptBounded(t *testing.T) {
	s := read(t, open(t, newMemFS(lawFiles())))
	ctx := context.Background()
	kept := func(sel []string) *build.Analysis {
		a, err := workspace.Analyze(ctx, s, sel)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	all, a := kept(nil), kept([]string{"a"})
	if kept([]string{"a"}) != a {
		t.Error("the latest selection is not kept")
	}
	kept([]string{"c"})
	if kept([]string{"a"}) == a {
		t.Error("a selection another replaced is still kept")
	}
	if kept(nil) != all {
		t.Error("every package's analysis is not kept")
	}
}

// API.md E17, API.md V13: the packages a package touches are it and every package importing it,
// directly or not, by the import clauses of the sources; a name no package has is left out.
func TestTouching(t *testing.T) {
	s := read(t, open(t, newMemFS(lawFiles())))
	for pkg, want := range map[string][]string{"a": {"a", "b"}, "b": {"b"}, "c": {"c"}, "zz": {}} {
		got, err := workspace.Touching(context.Background(), s, pkg)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("%s touches %v, want %v (%v)", pkg, got, want, err)
		}
	}
}

// API.md S12, API.md E22 (log-2026-09-29 M4 P14): the files an edit wrote are no overlays in the
// snapshot it publishes: the next edit of the same file is applied, and the disk holds it.
func TestEditSameFileTwice(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	commit(t, p, setB(2))
	out := commit(t, p, setB(3))
	disk, _ := fsys.ReadFile("/law/b/b.canon")
	if !slices.Contains(strings.Split(string(disk), "\n"), "const N = 3") {
		t.Fatalf("the second edit: %q", disk)
	}
	if data, _ := out.After.Build().FS().ReadFile("/law/b/b.canon"); string(data) != string(disk) {
		t.Fatalf("API.md S10: the snapshot published reads %q", data)
	}
}

// API.md S1 (log-2026-09-29 M4 P14): a file an edit wrote, changed on the disk after it, is read
// again by the next call, as any file read before.
func TestEditWrittenThenChanged(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	out := commit(t, p, setB(2))
	changed := strings.Replace(srcB, "const N = 1", "const N = 7", 1)
	if err := fsys.WriteFile("/law/b/b.canon", []byte(changed)); err != nil {
		t.Fatal(err)
	}
	s := read(t, p)
	data, _ := s.Build().FS().ReadFile("/law/b/b.canon")
	if s == out.After || string(data) != changed {
		t.Fatalf("API.md S1: the change after the edit is not seen: %q", data)
	}
}

// changeFS is a memFS that changes another file on the first rename, as a program writing
// during an edit's commit does.
type changeFS struct {
	*memFS
	once sync.Once
	name string
	data []byte
}

func (c *changeFS) Rename(oldname, newname string) error {
	c.once.Do(func() { _ = c.memFS.WriteFile(c.name, c.data) })
	return c.memFS.Rename(oldname, newname)
}

// API.md W15, API.md S10 (log-2026-09-29 M4 P14): an external change made during an edit's write
// is folded into its one event, whose snapshot reads it and the file the edit wrote.
func TestEditFoldsChangeDuringWrite(t *testing.T) {
	fsys := &changeFS{memFS: newMemFS(lawFiles()), name: "/law/data/c.json", data: []byte("[9]\n")}
	b, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := workspace.New(b)
	t.Cleanup(p.Close)
	analyze(t, read(t, p)) // c.json is read before the edit
	var events []workspace.Event
	defer p.Subscribe(func(e workspace.Event) { events = append(events, e) })()
	out := commit(t, p, setB(2))
	if len(events) != 1 || events[0].Cause != workspace.CauseEdit || !slices.Equal(events[0].Files, []string{"@data/c.json", "b/b.canon"}) {
		t.Fatalf("API.md W15: events %+v", events)
	}
	data, _ := out.After.Build().FS().ReadFile("/law/data/c.json")
	if events[0].Snapshot != out.After || string(data) != "[9]\n" {
		t.Fatalf("API.md S10: the published snapshot reads %q", data)
	}
}
