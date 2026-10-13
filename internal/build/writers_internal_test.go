package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/writelock"
)

// touching is the OS's file system whose every writer lock taken, up to touches, first changes
// the project with touch: another writer between a build's snapshot and its locks.
type touching struct {
	WriteFS
	dir     string
	touch   func(dir string, n int) error
	touches int
	count   int
}

func (t *touching) lockWriters(ctx context.Context, dir string, reqs []writelock.Request) (*writelock.Held, error) {
	if t.count < t.touches {
		t.count++
		if err := t.touch(t.dir, t.count); err != nil {
			return nil, err
		}
	}
	return writelock.Acquire(ctx, filepath.FromSlash(dir), reqs)
}

// touchedSource is package a holding n, reading a directory of rows and checking an asset.
func touchedSource(n int) string {
	return fmt.Sprintf("/// A.\npackage a\n\n/// Row.\nrecord Row {\n  /// N.\n  n: Int\n}\n\n"+
		"/// Rows.\nlet rows: [Row] = load.dir(\"data/*.json\")\n\n"+
		"/// Item.\nrecord Item {\n  /// Icon.\n  icon: asset(\"icons\", ext: [png])\n}\n\n"+
		"/// Items.\nlet items: [Item] = [{ icon: \"a.png\" }]\n\n"+
		"/// V.\nlet v: Int = %d\n\nemit json { out: \"out/\", values: [v] }\n", n)
}

// touchTree writes the project touchedSource(0) builds in dir.
func touchTree(t *testing.T, dir string) {
	t.Helper()
	for name, text := range map[string]string{ //canon:unordered each file written on its own
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n", "a/a.canon": touchedSource(0),
		"a/data/r1.json": "{\"n\": 1}\n", "a/icons/a.png": "",
	} {
		abs := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil || os.WriteFile(abs, []byte(text), 0o600) != nil {
			t.Fatal(name, err)
		}
	}
}

// writeIn writes text at name in dir.
func writeIn(dir, name, text string) error {
	return os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(text), 0o600)
}

// DECISIONS 352: what a build depends on changed between the snapshot and the locks (a source,
// a new source of the package, a new match of a load.dir, a listing an asset check used) runs
// the build once more; changed again, the build is a *StaleError naming it.
func TestBuildRereadsUnderLocks(t *testing.T) {
	for _, c := range []struct {
		name, moved string
		touch       func(dir string, n int) error
	}{
		{"source", "a/a.canon", func(dir string, n int) error { return writeIn(dir, "a/a.canon", touchedSource(n)) }},
		{"new source", "a/b", func(dir string, n int) error {
			return writeIn(dir, fmt.Sprintf("a/b%d.canon", n), "/// B.\npackage a\n")
		}},
		{"new match", "a/data/r", func(dir string, n int) error {
			return writeIn(dir, fmt.Sprintf("a/data/r%d.json", n+1), "{\"n\": 2}\n")
		}},
		{"listing", "a/icons", func(dir string, n int) error { return writeIn(dir, fmt.Sprintf("a/icons/n%d.png", n), "") }},
	} {
		for _, touches := range []int{1, 2} {
			dir := t.TempDir()
			touchTree(t, dir)
			p, err := Open(&touching{WriteFS: OS(), dir: dir, touch: c.touch, touches: touches}, filepath.ToSlash(dir), Options{})
			if err != nil {
				t.Fatal(err)
			}
			res, err := p.Build(context.Background(), BuildOptions{})
			var se *StaleError
			switch {
			case touches == 2 && (!errors.As(err, &se) || !errors.Is(err, ErrStale) || !strings.Contains(strings.Join(se.Files, " "), c.moved)):
				t.Errorf("DECISIONS 352: %s touched twice: %v", c.name, err)
			case touches == 1 && (err != nil || res.Summary.Errors != 0):
				t.Errorf("DECISIONS 352: %s touched once: %v %v", c.name, err, res)
			}
		}
	}
}
