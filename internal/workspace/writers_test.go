package workspace_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
	"github.com/fantasim/canonlang/internal/writelock"
)

const writersRule = "DECISIONS 352, API.md N9: "

// rowSource is package pkg holding r, a record of two counts; a emits it as JSON into a/out/.
func rowSource(pkg string) string {
	text := "/// " + pkg + ".\npackage " + pkg + "\n\n/// A row.\nrecord Row {\n  /// X.\n  x: Int = 0\n  /// Y.\n  y: Int = 0\n}\n\n/// R.\nlet r: Row = {}\n"
	if pkg == "a" {
		text += "\nemit json { out: \"out/\" }\n"
	}
	return text
}

// rowFiles are packages a and b, each its own row.
var rowFiles = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
	"a/a.canon":     rowSource("a"),
	"b/b.canon":     rowSource("b"),
}

// setOps is a Set of each path to n.
func setOps(n int, paths ...string) workspace.EditRequest {
	var ops []edit.Operation
	for _, p := range paths {
		ops = append(ops, edit.Operation{Kind: edit.OpSet, Path: p, Value: edit.Int(n)})
	}
	return workspace.EditRequest{Changes: workspace.Changes{Host: build.EditHost, Ops: ops}}
}

// holdWriters takes pkgs' writer locks as another writer, a build, would; the test releases them.
func holdWriters(t *testing.T, dir string, pkgs ...string) *writelock.Held {
	t.Helper()
	held, err := writelock.Acquire(context.Background(), filepath.FromSlash(dir), writelock.ExclusiveOf(pkgs...))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Release() })
	return held
}

// waitsOn is ctx telling waits on, once a writer waits for a lock.
func waitsOn(ctx context.Context, waits chan<- writelock.Wait) context.Context {
	return writelock.WithWaiting(ctx, func(w writelock.Wait) { waits <- w })
}

// waited is the next wait told on waits, failing the test past a deadline.
func waited(t *testing.T, waits <-chan writelock.Wait) writelock.Wait {
	t.Helper()
	select {
	case w := <-waits:
		return w
	case <-time.After(patience):
		t.Fatal("no writer waited")
	}
	return writelock.Wait{}
}

// fileText is the file rel of dir.
func fileText(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// patience is how long a test waits for a writer to wait or finish.
const patience = 30 * time.Second

// Two edits of one package from two projects (two writers of one directory) planned on the same
// sources: each waits for the lock the test holds, then one commits and N9 makes the other stale;
// the file holds the first's value and never loses it to the second.
func TestEditsOfOnePackageWait(t *testing.T) {
	dir, p1 := onDisk(t, rowFiles)
	p2 := openOS(t, dir)
	held := holdWriters(t, dir, "b")
	waits := make(chan writelock.Wait, 2)
	errs := make(chan error, 2)
	for i, p := range []*workspace.Project{p1, p2} {
		req := setOps(i+1, []string{"b:r.x", "b:r.y"}[i])
		go func() {
			_, err := p.Edit(waitsOn(context.Background(), waits), req)
			errs <- err
		}()
	}
	for range 2 {
		if w := waited(t, waits); w.Package != "b" {
			t.Errorf("%swaited for %+v, want package b", writersRule, w)
		}
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	applied, stale := 0, 0
	for range 2 {
		switch err := <-errs; {
		case err == nil:
			applied++
		case errors.Is(err, edit.ErrStale):
			stale++
		default:
			t.Errorf("%san edit failed: %v", writersRule, err)
		}
	}
	text := fileText(t, dir, "b/b.canon")
	if applied != 1 || stale != 1 || strings.Contains(text, "x: 1") == strings.Contains(text, "y: 2") {
		t.Errorf("%sapplied %d, stale %d, b.canon:\n%s", writersRule, applied, stale, text)
	}
}

// DECISIONS 352: an edit and a build of one package exclude each other. The edit writes a and b:
// it takes a, then waits for b, held by the test; a build then waits for a, the edit's, and runs
// once the edit committed, emitting its value.
func TestEditAndBuildExclude(t *testing.T) {
	dir, p := onDisk(t, rowFiles)
	held := holdWriters(t, dir, "b")
	editWaits, buildWaits := make(chan writelock.Wait, 1), make(chan writelock.Wait, 1)
	editErr := make(chan error, 1)
	go func() {
		_, err := p.Edit(waitsOn(context.Background(), editWaits), setOps(7, "a:r.x", "b:r.x"))
		editErr <- err
	}()
	if w := waited(t, editWaits); w.Package != "b" {
		t.Fatalf("%sthe edit waited for %+v, want b", writersRule, w)
	}
	b, err := build.Open(build.OS(), dir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	buildErr := make(chan error, 1)
	go func() {
		_, err := b.Build(waitsOn(context.Background(), buildWaits), build.BuildOptions{Packages: []string{"a"}})
		buildErr <- err
	}()
	if w := waited(t, buildWaits); w.Package != "a" {
		t.Fatalf("%sthe build waited for %+v, want a, the edit's", writersRule, w)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := <-editErr; err != nil {
		t.Fatalf("%sthe edit: %v", writersRule, err)
	}
	if err := <-buildErr; err != nil {
		t.Fatalf("%sthe build: %v", writersRule, err)
	}
	if out := fileText(t, dir, "a/out/r.json"); !strings.Contains(out, "7") {
		t.Errorf("%sthe build after the edit emitted %s", writersRule, out)
	}
}

// DECISIONS 352: a writer waiting for a lock honours its context: cancelled, the edit returns
// its context's error and writes nothing.
func TestEditCancelWhileWaiting(t *testing.T) {
	dir, p := onDisk(t, rowFiles)
	holdWriters(t, dir, "b")
	before := fileText(t, dir, "b/b.canon")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waits := make(chan writelock.Wait, 1)
	errs := make(chan error, 1)
	go func() {
		_, err := p.Edit(waitsOn(ctx, waits), setOps(3, "b:r.x"))
		errs <- err
	}()
	waited(t, waits)
	cancel()
	select {
	case err := <-errs:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%scancelled while waiting: %v", writersRule, err)
		}
	case <-time.After(patience):
		t.Fatal("the cancelled edit still waits")
	}
	if fileText(t, dir, "b/b.canon") != before {
		t.Errorf("%sa cancelled edit wrote", writersRule)
	}
}
