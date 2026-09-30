package workspace

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// plannedFS is a project of two sources in memory.
func plannedFS() fstest.MapFS {
	return fstest.MapFS{
		"law/project.canon": {Data: []byte("project a {\n  canon: \"0.1\"\n}\n")},
		"law/x/x.canon":     {Data: []byte("package x\n")},
		"law/x/y.canon":     {Data: []byte("package x\n")},
	}
}

// API.md E18, API.md V13, API.md N8: an edit made in memory reads a deleted file as absent, a
// renamed one at its new name, lists them so, and leaves the snapshot it was made on as it was.
func TestPlannedSnapshot(t *testing.T) {
	b, err := build.Open(&clockFS{MapFS: plannedFS()}, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(b)
	defer p.Close()
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	after := s.planned([]edit.Change{
		{Kind: edit.ChangeDeleted, Path: "x/x.canon"},
		{Kind: edit.ChangeRenamed, Path: "x/z.canon", OldPath: "x/y.canon", After: []byte("package x\n")},
		{Kind: edit.ChangeRemovedDir, Path: "x/gone"},
	}, nil)
	if _, err := after.fs.ReadFile("/law/x/x.canon"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a deleted file reads: %v", err)
	}
	if _, err := after.fs.Stat("/law/x/y.canon"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a file renamed away stats: %v", err)
	}
	var names []string
	entries, err := after.fs.ReadDir("/law/x")
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if err != nil || !slices.Equal(names, []string{"z.canon"}) {
		t.Errorf("listing %v, %v", names, err)
	}
	if data, err := s.fs.ReadFile("/law/x/x.canon"); err != nil || string(data) != "package x\n" {
		t.Errorf("the base snapshot changed: %q, %v", data, err)
	}
	if !slices.Contains(s.fs.lineage(), after.fs) {
		t.Error("API.md S9: a writer would not keep what the planned snapshot reads")
	}
}

// API.md O5: this process is a journal's running writer only while it commits in that directory;
// another pid of this host is judged by the system.
func TestSelfLiveness(t *testing.T) {
	me := self("/law")
	if me.PID != os.Getpid() || me.Alive(me.PID) {
		t.Fatalf("self %+v: alive while idle", me)
	}
	done := committing("/law")
	if !me.Alive(me.PID) || self("/other").Alive(me.PID) {
		t.Error("committing: not alive in its directory, or alive in another")
	}
	done()
	done()
	if me.Alive(me.PID) || !alive(os.Getpid()) {
		t.Error("a commit ended is still running, or this process is dead")
	}
}

// API.md S9, API.md N10: a commit's journal, which no snapshot reads, is written without
// pinning, so no snapshot keeps an entry for it.
func TestJournalUnpinned(t *testing.T) {
	fsys := &writeMapFS{clockFS: clockFS{MapFS: plannedFS()}}
	b, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(b)
	defer p.Close()
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	w := through(s.fs, fsys)
	for _, step := range []error{
		w.MkdirAll("/law/.canon/journal"),
		w.WriteFile("/law/.canon/journal/j.json", []byte("{}")),
		w.Remove("/law/.canon/journal/j.json"),
		w.WriteFile("/law/x/x.canon", []byte("package x\n\n")),
	} {
		if step != nil {
			t.Fatal(step)
		}
	}
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()
	pinned := false
	//canon:unordered a membership test
	for n := range s.fs.ents {
		if strings.HasPrefix(n.abs, "/law/.canon/") {
			t.Errorf("pinned %v", n)
		}
		pinned = pinned || n.abs == "/law/x/x.canon"
	}
	if !pinned {
		t.Error("API.md S9: a source written was not pinned")
	}
}

// writeMapFS is clockFS with writes, which keep a file's modification time.
type writeMapFS struct {
	clockFS
}

func (w *writeMapFS) WriteFile(name string, data []byte) error {
	f := &fstest.MapFile{Data: data}
	if old := w.MapFS[name[1:]]; old != nil {
		f.ModTime = old.ModTime
	}
	w.MapFS[name[1:]] = f
	return nil
}

// API.md W15, API.md S10 (log-2026-09-29 M4 U5b-r): an edit whose writes the refresh cannot see
// (same size, same time) still publishes one edit event, its files read again from the disk.
func TestEditUnseenWrite(t *testing.T) {
	files := plannedFS()
	files["law/x/x.canon"].ModTime = time.Unix(1, 0)
	fsys := &writeMapFS{clockFS: clockFS{MapFS: files}}
	b, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(b)
	defer p.Close()
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.fs.ReadFile("/law/x/x.canon"); err != nil {
		t.Fatal(err)
	}
	var events []Event
	defer p.Subscribe(func(e Event) { events = append(events, e) })()
	after := func() wrote { return wrote{cause: CauseEdit, files: []string{"/law/x/x.canon"}} }
	next, err := p.writeAs(context.Background(), after, func(context.Context, *Snapshot) error {
		return fsys.WriteFile("/law/x/x.canon", []byte("package y\n"))
	})
	data, _ := next.fs.ReadFile("/law/x/x.canon")
	if err != nil || string(data) != "package y\n" || len(events) != 1 || events[0].Cause != CauseEdit || events[0].Snapshot != next {
		t.Fatalf("next reads %q, events %+v, %v", data, events, err)
	}
}

// API.md E17, API.md X2 (log-2026-09-29 M4 U5b-r): changes no package owns are never written
// unchecked: the re-check refuses them as a compiler bug.
func TestRecheckUnowned(t *testing.T) {
	b, err := build.Open(&clockFS{MapFS: plannedFS()}, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(b)
	defer p.Close()
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	o := &EditOutcome{Plan: &edit.Plan{}, Changes: []edit.Change{{Kind: edit.ChangeModified, Path: "x/x.canon"}}, Before: s}
	if err := o.recheck(context.Background(), a, o.Plan.Touched); !errors.Is(err, edit.ErrInternal) {
		t.Errorf("recheck: %v", err)
	}
}

func (w *writeMapFS) Rename(oldname, newname string) error {
	w.MapFS[newname[1:]] = w.MapFS[oldname[1:]]
	delete(w.MapFS, oldname[1:])
	return nil
}

func (w *writeMapFS) Remove(name string) error {
	delete(w.MapFS, name[1:])
	return nil
}

func (w *writeMapFS) MkdirAll(string) error { return nil }
