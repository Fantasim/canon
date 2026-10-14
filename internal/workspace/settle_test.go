package workspace_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
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

// warnings counts the Warn lines written to buf.
func warnings(buf *bytes.Buffer) int {
	return strings.Count(buf.String(), "level=WARN")
}

// textLogger logs to buf, every level.
func textLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, nil))
}

// API.md O5 (DECISIONS 375: J11): an edit that rolls back a dead commit's journal logs one Warn
// line on NewWith's logger.
func TestEditSettleWarnsOnce(t *testing.T) {
	dir, _ := onDisk(t, rowFiles)
	tornCommit(t, dir, []string{"a"}, 1, []edit.Change{rowChange(t, dir, "a/a.canon", 5)})
	var buf bytes.Buffer
	p := workspace.NewWith(openBuild(t, dir), textLogger(&buf))
	t.Cleanup(p.Close)
	if _, err := p.Edit(context.Background(), setOps(4, "a:r.y")); !errors.Is(err, workspace.ErrStale) {
		t.Fatalf("%sthe edit: %v, want stale", journalRule, err)
	}
	if n := warnings(&buf); n != 1 {
		t.Errorf("%s%d Warn lines, want 1:\n%s", journalRule, n, buf.String())
	}
}

// ADR-0021 J10: a NewWith project's writing build rolls back a dead commit's journal, then emits.
func TestBuildSettlesThroughWorkspace(t *testing.T) {
	dir, _ := onDisk(t, rowFiles)
	before := fileText(t, dir, "a/a.canon")
	tornCommit(t, dir, []string{"a"}, 1, []edit.Change{rowChange(t, dir, "a/a.canon", 5)})
	var buf bytes.Buffer
	p := workspace.NewWith(openBuild(t, dir), textLogger(&buf))
	t.Cleanup(p.Close)
	_, err := p.Write(context.Background(), workspace.CauseEdit, func(ctx context.Context, s *workspace.Snapshot) error {
		_, err := s.Build().Build(ctx, build.BuildOptions{Packages: []string{"a"}})
		return err
	})
	if err != nil {
		t.Fatalf("%sthe build: %v", journalRule, err)
	}
	if got := journalsOf(t, dir); len(got) != 0 || fileText(t, dir, "a/a.canon") != before {
		t.Errorf("%sthe build left journals %v", journalRule, got)
	}
	if out := fileText(t, dir, "a/out/r.json"); strings.Contains(out, "5") {
		t.Errorf("%sthe build emitted the torn value: %s", journalRule, out)
	}
	if n := warnings(&buf); n != 1 {
		t.Errorf("%s%d Warn lines, want 1", journalRule, n)
	}
}

// busy reports req's lock held by another writer.
func busy(t *testing.T, dir string, req writelock.Request) bool {
	t.Helper()
	h, ok, err := writelock.TryAcquire(filepath.FromSlash(dir), []writelock.Request{req})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		_ = h.Release()
	}
	return !ok
}

// DECISIONS 375 (J4): LockAndSettle takes a package asked shared exclusive when a journal records
// it, and every package of the journals its packages meet, then rolls them back.
func TestLockAndSettleGrows(t *testing.T) {
	cases := []struct {
		name    string
		journal []string
		reqs    []writelock.Request
	}{
		{"shared upgraded", []string{"a"}, []writelock.Request{{Package: "a", Mode: writelock.Shared}}},
		{"closure", []string{"a", "b"}, writelock.ExclusiveOf("a")},
	}
	for _, c := range cases {
		dir, _ := onDisk(t, rowFiles)
		tornCommit(t, dir, c.journal, 1, []edit.Change{rowChange(t, dir, "a/a.canon", 5)})
		release, changed, err := workspace.LockAndSettle(context.Background(), openBuild(t, dir), c.reqs, nil)
		if err != nil {
			t.Fatalf("%s%s: %v", journalRule, c.name, err)
		}
		for _, pkg := range c.journal {
			if !busy(t, dir, writelock.Request{Package: pkg, Mode: writelock.Shared}) {
				t.Errorf("%s%s: %s is not held exclusive", journalRule, c.name, pkg)
			}
		}
		if err := release(); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(changed, []string{"a/a.canon"}) || len(journalsOf(t, dir)) != 0 {
			t.Errorf("%s%s: changed %v, journals %v", journalRule, c.name, changed, journalsOf(t, dir))
		}
	}
}

// DECISIONS 375 (J6, J10): LockAndSettle leaves a journal whose locks are disjoint from its own,
// and on a file system that takes no writer lock settles nothing.
func TestLockAndSettleLeaves(t *testing.T) {
	dir, _ := onDisk(t, rowFiles)
	tornCommit(t, dir, []string{"a"}, 1, []edit.Change{rowChange(t, dir, "a/a.canon", 5)})
	release, changed, err := workspace.LockAndSettle(context.Background(), openBuild(t, dir), writelock.ExclusiveOf("b"), nil)
	if err != nil || changed != nil || len(journalsOf(t, dir)) != 1 {
		t.Errorf("%sa disjoint journal: changed %v, %v", journalRule, changed, err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	b, err := build.Open(newMemFS(lawFiles()), "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	release, changed, err = workspace.LockAndSettle(context.Background(), b, writelock.ExclusiveOf("a"), nil)
	if err != nil || changed != nil || release() != nil {
		t.Errorf("%san FS without locks: changed %v, %v", journalRule, changed, err)
	}
}

// tornTwo is rowFiles with a/sub/z.json, after a dead commit of a that rewrote a/a.canon and
// a/sub/z.json, and a/sub made read-only, so rolling it back restores a/a.canon, then fails.
func tornTwo(t *testing.T) string {
	t.Helper()
	files := map[string]string{"a/sub/z.json": "[1]\n"}
	for k, v := range rowFiles {
		files[k] = v
	}
	dir, _ := onDisk(t, files)
	z := edit.Change{Kind: edit.ChangeModified, Path: "a/sub/z.json", Before: []byte("[1]\n"), After: []byte("[2]\n")}
	tornCommit(t, dir, []string{"a"}, 2, []edit.Change{rowChange(t, dir, "a/a.canon", 5), z})
	sub := filepath.Join(filepath.FromSlash(dir), "a", "sub")
	if err := os.Chmod(sub, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })
	return dir
}

// API.md O5, N11 (DECISIONS 375: J5, J11): a Settle that restores a file then fails returns it
// with an error naming the journal, kept, and logs no Warn; an edit meeting the journal is stale.
func TestSettleChangedThenFailed(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a read-only directory refuses no write here")
	}
	dir := tornTwo(t)
	var buf bytes.Buffer
	b := openBuild(t, dir)
	held := holdWriters(t, dir, "a")
	site := edit.Site{FS: build.OS(), Layout: b, Self: edit.Process{Host: hostName(), PID: os.Getpid()}, Locks: anyLocker{}, Held: []string{"a"}}
	changed, err := edit.Settle(site, textLogger(&buf))
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	kept := journalsOf(t, dir)
	if len(kept) != 1 || warnings(&buf) != 0 {
		t.Fatalf("%sSettle logged %q, journals %v", journalRule, buf.String(), kept)
	}
	if err == nil || !strings.Contains(err.Error(), kept[0]) || !slices.Equal(changed, []string{"a/a.canon"}) {
		t.Errorf("%sSettle: changed %v, %v", journalRule, changed, err)
	}
	dir = tornTwo(t)
	buf.Reset()
	p := workspace.NewWith(openBuild(t, dir), textLogger(&buf))
	t.Cleanup(p.Close)
	_, err = p.Edit(context.Background(), setOps(4, "a:r.y"))
	if files, ok := staleFiles(err); !ok || !slices.Equal(files, []string{"a/a.canon"}) {
		t.Errorf("%sthe edit: %v, want stale naming a/a.canon", journalRule, err)
	}
	if warnings(&buf) != 0 {
		t.Errorf("%sthe edit logged %q", journalRule, buf.String())
	}
	failedAgain(t, dir, p)
}

// failedAgain proves J6 on dir, whose journal's rollback fails: the edit's retry, LockAndSettle
// and a writing build each fail with an error wrapping edit.ErrJournal that names the journal.
func failedAgain(t *testing.T, dir string, p *workspace.Project) {
	t.Helper()
	kept := journalsOf(t, dir)
	_, editErr := p.Edit(context.Background(), setOps(4, "a:r.y"))
	release, _, lockErr := workspace.LockAndSettle(context.Background(), openBuild(t, dir), writelock.ExclusiveOf("a"), nil)
	_, buildErr := p.Write(context.Background(), workspace.CauseEdit, func(ctx context.Context, s *workspace.Snapshot) error {
		_, err := s.Build().Build(ctx, build.BuildOptions{Packages: []string{"a"}})
		return err
	})
	for name, err := range map[string]error{"the retry": editErr, "LockAndSettle": lockErr, "the build": buildErr} {
		if !errors.Is(err, edit.ErrJournal) || len(kept) != 1 || !strings.Contains(err.Error(), kept[0]) {
			t.Errorf("%s%s: %v, want ErrJournal naming %v", journalRule, name, err, kept)
		}
	}
	if release != nil {
		t.Errorf("%sLockAndSettle failed but returned a release", journalRule)
	}
}
