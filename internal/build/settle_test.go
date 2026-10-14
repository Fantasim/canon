package build_test

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/writelock"
)

// settleProject writes a project of one package a emitting one value, in a new directory.
func settleProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{ //canon:unordered each file written on its own
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"a/a.canon":     "/// A.\npackage a\n\n/// V.\nlet v: Int = 1\n\nemit json { out: \"out/\", values: [v] }\n",
	} {
		abs := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil || os.WriteFile(abs, []byte(text), 0o600) != nil {
			t.Fatal(name, err)
		}
	}
	return dir
}

// recordingSettler records what each Settle held; no journal meets it.
type recordingSettler struct {
	mu      sync.Mutex
	settles [][]writelock.Request
}

func (s *recordingSettler) Meeting(held []writelock.Request) ([]writelock.Request, error) {
	return held, nil
}

func (s *recordingSettler) Settle(held []writelock.Request) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settles = append(s.settles, slices.Clone(held))
	return nil, nil
}

// ADR-0021 J10: an outside Settler settles a writing build under its locks, then it writes.
func TestWithSettleRunsForWritingBuild(t *testing.T) {
	dir := settleProject(t)
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := &recordingSettler{}
	res, err := p.WithSettle(s).Build(context.Background(), build.BuildOptions{})
	want := [][]writelock.Request{{{Package: "a", Mode: writelock.Exclusive}}}
	if err != nil || res.Summary.Errors != 0 || !slices.EqualFunc(s.settles, want, slices.Equal) {
		t.Errorf("ADR-0021 J10: %v; settled under %v", err, s.settles)
	}
}

// ADR-0021 J10: OSReading reports each file read and keeps locks, SyncDir, Chmod and Readlink.
func TestOSReadingKeepsCapabilities(t *testing.T) {
	dir := settleProject(t)
	var mu sync.Mutex
	read := map[string][]byte{}
	fsys := build.OSReading(func(name string, data []byte) {
		mu.Lock()
		defer mu.Unlock()
		read[name] = data
	})
	_, syncs := fsys.(build.DirSyncer)
	_, links := fsys.(build.LinkReader)
	_, chmods := fsys.(build.ModeFS)
	if !build.LocksWriters(fsys) || !syncs || !links || !chmods {
		t.Fatalf("ADR-0021 J10: locks %v, SyncDir %v, Readlink %v, Chmod %v", build.LocksWriters(fsys), syncs, links, chmods)
	}
	p, err := build.Open(fsys, filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Build(context.Background(), build.BuildOptions{}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if got := string(read[filepath.ToSlash(filepath.Join(dir, "a", "a.canon"))]); got == "" {
		t.Errorf("ADR-0021 J10: a/a.canon not reported read: %v", slices.Sorted(maps.Keys(read)))
	}
}
