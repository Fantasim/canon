package canon_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/writelock"
)

const writersRule = "DECISIONS 352: "

// valueSource is package a holding n, emitted as JSON into a/out/.
func valueSource(n int) string {
	return fmt.Sprintf("/// A.\npackage a\n\n/// V.\nlet v: Int = %d\n\nemit json { out: \"out/\" }\n", n)
}

// valueProject is a project of package a holding n on the OS's file system, opened.
func valueProject(t *testing.T, n int) (string, *canon.Project) {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{"project.canon": "project acme {\n  canon: \"0.1\"\n}\n", "a/a.canon": valueSource(n)} { //canon:unordered each file on its own
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o700); err != nil || os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600) != nil {
			t.Fatal(name, err)
		}
	}
	p, err := canon.Open(dir, canon.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return dir, p
}

// emitted is the value a/out/v.json holds.
func emitted(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "a", "out", "v.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// DECISIONS 352: the re-read under the locks goes through the snapshot's view, overlays
// included: a writing Build with an overlay is not stale and emits the overlaid value.
func TestBuildOverlayNotStale(t *testing.T) {
	dir, p := valueProject(t, 1)
	ctx := context.Background()
	if err := p.SetOverlay(ctx, "a/a.canon", []byte(valueSource(2))); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Build(ctx, canon.BuildOptions{}); err != nil || !strings.Contains(emitted(t, dir), `"value": 2`) {
		t.Errorf("%soverlay: %v", writersRule, err)
	}
}

// DECISIONS 352: a source edited while the build waits for its lock makes the build run once
// more on a refreshed snapshot, which emits the edited value.
func TestBuildRetriesOnRefreshedSnapshot(t *testing.T) {
	dir, p := valueProject(t, 1)
	held, err := writelock.Acquire(context.Background(), dir, []writelock.Request{{Package: "a", Mode: writelock.Exclusive}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := writelock.WithWaiting(context.Background(), func(writelock.Wait) {
		if err := os.WriteFile(filepath.Join(dir, "a", "a.canon"), []byte(valueSource(2)), 0o600); err != nil {
			t.Error(err)
		}
		_ = held.Release() // the other writer is done
	})
	if _, err := p.Build(ctx, canon.BuildOptions{}); err != nil || !strings.Contains(emitted(t, dir), `"value": 2`) {
		t.Errorf("%sretry: %v", writersRule, err)
	}
}

// importingSource is package a holding a z.Z of n: an a that loads z too, so its build takes z's lock.
func importingSource(n int) string {
	return fmt.Sprintf("/// A.\npackage a\n\nimport z { Z }\n\n/// V.\nlet v: Z = { n: %d }\n\nemit json { out: \"out/\" }\n", n)
}

// heldBoth holds the writer locks of a and z in dir, as another writer would.
func heldBoth(t *testing.T, dir string) map[string]*writelock.Held {
	t.Helper()
	held := map[string]*writelock.Held{}
	for _, pkg := range []string{"a", "z"} {
		h, err := writelock.Acquire(context.Background(), dir, []writelock.Request{{Package: pkg, Mode: writelock.Exclusive}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = h.Release() })
		held[pkg] = h
	}
	return held
}

// twiceStale is a project where `build a` meets a source changed under its locks on its run and
// on its redo, deterministically: the run waits for a's lock, whose waiter rewrites a to import z
// and frees a; the redo, which now loads z, waits for z's lock, whose waiter rewrites a again.
func twiceStale(t *testing.T) (string, context.Context, *int) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n", "a/a.canon": valueSource(1),
		"z/z.canon": "/// Z.\npackage z\n\n/// A z.\nrecord Z {\n  /// N.\n  n: Int\n}\n",
	}
	for name, text := range files { //canon:unordered each file on its own
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o700); err != nil || os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600) != nil {
			t.Fatal(name, err)
		}
	}
	held, waits := heldBoth(t, dir), new(int)
	return dir, writelock.WithWaiting(context.Background(), func(w writelock.Wait) {
		*waits++
		if err := os.WriteFile(filepath.Join(dir, "a", "a.canon"), []byte(importingSource(*waits)), 0o600); err != nil {
			t.Error(err)
		}
		_ = held[w.Package].Release() // the other writer of that package is done
	}), waits
}

// DECISIONS 352, API.md N9, §15: sources changing under the locks twice are a *StaleError.
func TestBuildStaleTwice(t *testing.T) {
	dir, ctx, waits := twiceStale(t)
	p, err := canon.Open(dir, canon.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	_, err = p.Build(ctx, canon.BuildOptions{Packages: []string{"a"}})
	var se *canon.StaleError
	if !errors.Is(err, canon.ErrStale) || !errors.As(err, &se) || !slices.Contains(se.Files, "a/a.canon") || *waits != 2 {
		t.Errorf("%s%v", writersRule, err)
	}
}
