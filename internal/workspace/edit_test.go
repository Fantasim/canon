package workspace_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
)

// twoFiles sets b's constant and the first element a loads: b/b.canon and a/a.json.
func twoFiles() workspace.Changes {
	return workspace.Changes{Host: build.EditHost, Ops: []edit.Operation{
		{Kind: edit.OpSet, Path: "b:N", Value: edit.Int(2)},
		{Kind: edit.OpSet, Path: "a:xs[0]", Value: edit.Int(5)},
	}}
}

// API.md E17, API.md E18, API.md W15, API.md S10, API.md N10: an edit commits its files, removes
// its journal, and publishes one snapshot with cause edit naming them, the one After is.
func TestEditCommits(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	var events []workspace.Event
	defer p.Subscribe(func(e workspace.Event) { events = append(events, e) })()
	out, err := p.Edit(context.Background(), workspace.EditRequest{Changes: twoFiles(), Normalize: true})
	if err != nil || !out.Applied || out.Checked.Summary.Errors != 0 {
		t.Fatalf("Edit: %+v, %v", out, err)
	}
	if len(events) != 1 || events[0].Cause != workspace.CauseEdit || !slices.Equal(events[0].Files, []string{"a/a.json", "b/b.canon"}) {
		t.Fatalf("API.md W15: events %+v", events)
	}
	if read(t, p) != out.After || events[0].Snapshot != out.After {
		t.Error("API.md S10: After is not the snapshot published")
	}
	if data, _ := fsys.ReadFile("/law/a/a.json"); strings.Join(strings.Fields(string(data)), "") != "[5,2]" {
		t.Errorf("a.json %q", data)
	}
	if entries, err := fsys.ReadDir("/law/.canon/journal"); err != nil || len(entries) != 0 {
		t.Errorf("API.md N10: the journal is left: %v, %v", entries, err)
	}
	if out.Checked.Summary.Packages != 2 {
		t.Errorf("API.md E17: packages a and b re-checked, not c: %+v", out.Checked.Summary)
	}
}

// API.md E19, API.md V13: DryRun and a draft make the edit in memory only: After reads the new
// content, the published snapshot and the disk the old; nothing is published.
func TestEditInMemory(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	published := 0
	defer p.Subscribe(func(workspace.Event) { published++ })()
	out, err := p.Edit(context.Background(), workspace.EditRequest{Changes: twoFiles(), DryRun: true, Normalize: true})
	if err != nil || out.Applied || out.After == out.Before {
		t.Fatalf("DryRun: %+v, %v", out, err)
	}
	s := read(t, p)
	d, err := workspace.Draft(context.Background(), s, analyze(t, s), twoFiles())
	if err != nil {
		t.Fatal(err)
	}
	for _, after := range []*workspace.Snapshot{out.After, d.Snapshot} {
		if data, _ := after.Build().FS().ReadFile("/law/b/b.canon"); !slices.Contains(strings.Split(string(data), "\n"), "const N = 2") {
			t.Errorf("in memory: %q", data)
		}
	}
	old, _ := s.Build().FS().ReadFile("/law/b/b.canon")
	disk, _ := fsys.ReadFile("/law/b/b.canon")
	if string(old) != srcB || string(disk) != srcB || published != 0 || len(d.Plan.Touched) != 2 {
		t.Errorf("a DryRun or a draft wrote or published: %q %q %d %v", old, disk, published, d.Plan.Touched)
	}
	if len(d.Analysis.Result().List) != 0 {
		t.Errorf("the draft's analysis: %v", d.Analysis.Result().List)
	}
}

// API.md S12, API.md M9: a file with an overlay is refused by name, and one not in canonical
// layout without Normalize; nothing is written.
func TestEditRefused(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	_, err := p.Edit(context.Background(), workspace.EditRequest{Changes: twoFiles()})
	var ne *workspace.NotCanonicalError
	if !errors.As(err, &ne) || !slices.Equal(ne.Files, []string{"a/a.json"}) {
		t.Errorf("API.md M9: %v", err)
	}
	if err := p.SetOverlay("b/b.canon", []byte(srcB)); err != nil {
		t.Fatal(err)
	}
	_, err = p.Edit(context.Background(), workspace.EditRequest{Changes: twoFiles(), Normalize: true})
	var oe *workspace.OverlayError
	if !errors.As(err, &oe) || oe.File != "b/b.canon" {
		t.Errorf("API.md S12: %v", err)
	}
	if data, _ := fsys.ReadFile("/law/a/a.json"); string(data) != "[1, 2]\n" {
		t.Errorf("a refused edit wrote %q", data)
	}
}
