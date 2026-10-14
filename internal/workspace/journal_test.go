package workspace_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
	"github.com/fantasim/canonlang/internal/writelock"
)

// API.md N11 (DECISIONS 375: J6): an edit of b applies while another process's commit of a
// stalls, its journal present: a journal whose locks are disjoint from the edit's does not block it.
func TestEditOtherPackageWhileCommitStalls(t *testing.T) {
	dir, p := onDisk(t, rowFiles)
	startStall(t, dir, "a:r.x", "5")
	if _, err := p.Edit(context.Background(), setOps(3, "b:r.x")); err != nil {
		t.Fatalf("%sthe edit of b beside a stalled commit of a: %v", journalRule, err)
	}
	if text := fileText(t, dir, "b/b.canon"); !strings.Contains(text, "x: 3") {
		t.Errorf("%sb.canon:\n%s", journalRule, text)
	}
	if got := journalsOf(t, dir); len(got) != 1 {
		t.Errorf("%sthe stalled commit's journal: %v", journalRule, got)
	}
}

// API.md O5, N11 (DECISIONS 375: J3): a dead commit's journal whose lock another writer holds is
// left by Open's recovery, which succeeds; once the lock is free, recovery rolls it back.
func TestOpenLeavesJournalWhileLockHeld(t *testing.T) {
	dir, _ := onDisk(t, rowFiles)
	before := fileText(t, dir, "a/a.canon")
	startStall(t, dir, "a:r.x", "5").kill()
	torn := fileText(t, dir, "a/a.canon")
	held := holdWriters(t, dir, "a")
	b := openBuild(t, dir)
	if err := workspace.Recover(b, nil); err != nil {
		t.Fatalf("%sRecover while a is held: %v", journalRule, err)
	}
	if got := journalsOf(t, dir); len(got) != 1 || fileText(t, dir, "a/a.canon") != torn || torn == before {
		t.Fatalf("%sRecover while a is held rolled back: journals %v", journalRule, got)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Recover(b, nil); err != nil {
		t.Fatalf("%sRecover once a is free: %v", journalRule, err)
	}
	if got := journalsOf(t, dir); len(got) != 0 || fileText(t, dir, "a/a.canon") != before {
		t.Errorf("%sRecover once a is free: journals %v, a.canon:\n%s", journalRule, got, fileText(t, dir, "a/a.canon"))
	}
}

// API.md N11 (DECISIONS 375: J4, J5): an edit meeting a dead commit's journal rolls it back under
// its locks and is a *StaleError naming the files restored, writing nothing; the retry applies.
func TestEditRecoversAbandonedJournal(t *testing.T) {
	dir, _ := onDisk(t, rowFiles)
	before := fileText(t, dir, "a/a.canon")
	startStall(t, dir, "a:r.x", "5").kill()
	p := openOS(t, dir)
	_, err := p.Edit(context.Background(), setOps(4, "a:r.y"))
	if files, ok := staleFiles(err); !ok || !slices.Equal(files, []string{"a/a.canon"}) {
		t.Fatalf("%sthe first edit: %v, want stale naming a/a.canon", journalRule, err)
	}
	if got := journalsOf(t, dir); len(got) != 0 || fileText(t, dir, "a/a.canon") != before {
		t.Fatalf("%sthe journal was not rolled back: %v", journalRule, got)
	}
	if _, err := p.Edit(context.Background(), setOps(4, "a:r.y")); err != nil {
		t.Fatalf("%sthe retry: %v", journalRule, err)
	}
	if text := fileText(t, dir, "a/a.canon"); !strings.Contains(text, "y: 4") || strings.Contains(text, "x: Int = 5") {
		t.Errorf("%sa.canon after the retry:\n%s", journalRule, text)
	}
}

// API.md N11 (DECISIONS 375: J1, J3): Open's recovery through a symbolic link to the project
// leaves the journal of a commit of this process whose lock is held: liveness is the lock.
func TestRecoverThroughAlias(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	dir, _ := onDisk(t, rowFiles)
	before := fileText(t, dir, "a/a.canon")
	tornCommit(t, dir, []string{"a"}, 1, []edit.Change{rowChange(t, dir, "a/a.canon", 5)})
	held := holdWriters(t, dir, "a")
	torn := fileText(t, dir, "a/a.canon")
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(filepath.FromSlash(dir), alias); err != nil {
		t.Fatal(err)
	}
	b := openBuild(t, filepath.ToSlash(alias))
	if err := workspace.Recover(b, nil); err != nil {
		t.Fatalf("%sRecover through the alias: %v", journalRule, err)
	}
	if got := journalsOf(t, dir); len(got) != 1 || fileText(t, dir, "a/a.canon") != torn {
		t.Fatalf("%sRecover through the alias rolled back the live commit: journals %v", journalRule, got)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Recover(b, nil); err != nil {
		t.Fatalf("%sRecover through the alias once a is free: %v", journalRule, err)
	}
	if got := journalsOf(t, dir); len(got) != 0 || fileText(t, dir, "a/a.canon") != before {
		t.Errorf("%sRecover through the alias once a is free: journals %v", journalRule, got)
	}
}

// jsonFiles are packages a and d, both loading a/a.json.
var jsonFiles = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
	"a/a.canon":     "/// A.\npackage a\n\n/// Xs.\nlet xs: [Int] = load(\"a.json\")\n",
	"a/a.json":      "[1, 2]\n",
	"d/d.canon":     "/// D.\npackage d\n\n/// Ys.\nlet ys: [Int] = load(\"../a/a.json\")\n",
}

// CLI.md §3.15, DECISIONS 375 J7: an edit of a JSON file d reads too waits for d's lock.
func TestEditLocksJSONReaders(t *testing.T) {
	dir, p := onDisk(t, jsonFiles)
	held := holdWriters(t, dir, "d")
	waits, errs := make(chan writelock.Wait, 1), make(chan error, 1)
	req := workspace.EditRequest{Changes: workspace.Changes{Host: build.EditHost, Ops: []edit.Operation{{Kind: edit.OpSet, Path: "a:xs[0]", Value: edit.Int(5)}}}, Normalize: true}
	go func() {
		_, err := p.Edit(waitsOn(context.Background(), waits), req)
		errs <- err
	}()
	if w := waited(t, waits); w.Package != "d" {
		t.Fatalf("%sthe edit waited for %+v, want d", journalRule, w)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := <-errs; err != nil {
		t.Fatalf("%sthe edit: %v", journalRule, err)
	}
	if text := fileText(t, dir, "a/a.json"); !strings.Contains(text, "5") {
		t.Errorf("%sa.json: %s", journalRule, text)
	}
}

// API.md N11 (DECISIONS 375: J3, J4): while one recovery rolls back a dead commit's journal under
// its locks, Open's recovery leaves it and an edit of its package waits for the first.
func TestTwoRecoveriesSerialize(t *testing.T) {
	dir, p := onDisk(t, rowFiles)
	tornCommit(t, dir, []string{"a"}, 1, []edit.Change{rowChange(t, dir, "a/a.canon", 5)})
	b := openBuild(t, dir)
	reached, gate := make(chan struct{}), make(chan struct{})
	fsys := cutOS(dir, 1, func() { close(reached); <-gate })
	site := edit.Site{FS: fsys, Layout: b, Self: edit.Process{Host: hostName(), PID: os.Getpid()}, Locks: workspace.LockerFor(build.OS(), dir)}
	first := make(chan error, 1)
	go func() { first <- edit.Recover(site, nil) }()
	<-reached
	if err := workspace.Recover(b, nil); err != nil || len(journalsOf(t, dir)) != 1 {
		t.Fatalf("%sthe second recovery: %v, journals %v", journalRule, err, journalsOf(t, dir))
	}
	waits, errs := make(chan writelock.Wait, 1), make(chan error, 1)
	go func() {
		_, err := p.Edit(waitsOn(context.Background(), waits), setOps(6, "a:r.y"))
		errs <- err
	}()
	if w := waited(t, waits); w.Package != "a" {
		t.Errorf("%sthe edit waited for %+v, want a", journalRule, w)
	}
	close(gate)
	if err := <-first; err != nil {
		t.Fatalf("%sthe first recovery: %v", journalRule, err)
	}
	if err := <-errs; err != nil && !errors.Is(err, edit.ErrStale) && !errors.Is(err, workspace.ErrStale) {
		t.Errorf("%sthe edit after the recovery: %v", journalRule, err)
	}
	if got := journalsOf(t, dir); len(got) != 0 {
		t.Errorf("%sjournals left: %v", journalRule, got)
	}
}
