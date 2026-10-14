package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

// journalSettler is a dead edit's journal on journal's packages, rolled back by restore: it
// records each Meeting and Settle, and the journal packages a shared try-lock found free at Settle.
type journalSettler struct {
	dir      string
	journal  []string
	restore  func() error
	failMeet error
	failSet  error
	meetings [][]writelock.Request
	settles  [][]writelock.Request
	free     []string
}

func (s *journalSettler) Meeting(held []writelock.Request) ([]writelock.Request, error) {
	s.meetings = append(s.meetings, slices.Clone(held))
	if s.failMeet != nil {
		return nil, s.failMeet
	}
	out := slices.Clone(held)
	if slices.ContainsFunc(held, func(r writelock.Request) bool { return slices.Contains(s.journal, r.Package) }) {
		for _, pkg := range s.journal {
			if !slices.Contains(held, req(pkg, writelock.Exclusive)) {
				out = append(out, req(pkg, writelock.Exclusive))
			}
		}
	}
	if len(out) == len(held) {
		return held, nil
	}
	return out, nil
}

func (s *journalSettler) Settle(held []writelock.Request) ([]string, error) {
	s.settles = append(s.settles, slices.Clone(held))
	if s.failSet != nil {
		return nil, s.failSet
	}
	if s.journal == nil {
		return nil, nil
	}
	for _, pkg := range s.journal {
		h, ok, err := writelock.TryAcquire(s.dir, []writelock.Request{req(pkg, writelock.Shared)})
		if err != nil || ok {
			s.free = append(s.free, pkg)
		}
		if ok {
			_ = h.Release() // the probe's; the test fails on s.free
		}
	}
	s.journal = nil
	return []string{"b/b.canon"}, s.restore()
}

// probing checks before each acquire that the last one's locks are free (ADR-0021 J4).
type probing struct {
	WriteFS
	t    *testing.T
	last []writelock.Request
}

func (p *probing) lockWriters(ctx context.Context, dir string, reqs []writelock.Request) (*writelock.Held, error) {
	if len(p.last) > 0 {
		h, ok, err := writelock.TryAcquire(filepath.FromSlash(dir), writelock.ExclusiveOf(packagesOf(p.last)...))
		if err != nil || !ok {
			p.t.Errorf("ADR-0021 J4: acquiring %v while holding %v: %v", reqs, p.last, err)
		} else if err := h.Release(); err != nil {
			p.t.Error(err)
		}
	}
	p.last = reqs
	return writelock.Acquire(ctx, filepath.FromSlash(dir), reqs)
}

func packagesOf(reqs []writelock.Request) []string {
	out := make([]string, len(reqs))
	for i, r := range reqs {
		out[i] = r.Package
	}
	return out
}

// settleTree writes package a, importing b, emitting the value b holds, at 2.
func settleTree(t *testing.T, dir string) {
	t.Helper()
	for name, text := range map[string]string{ //canon:unordered each file written on its own
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"a/a.canon":     "/// A.\npackage a\n\nimport b\n\n/// W.\nlet w: Int = b.v\n\nemit json { out: \"out/\", values: [w] }\n",
		"b/b.canon":     bSource(2),
	} {
		abs := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil || os.WriteFile(abs, []byte(text), 0o600) != nil {
			t.Fatal(name, err)
		}
	}
}

func bSource(v int) string { return fmt.Sprintf("/// B.\npackage b\n\n/// V.\nlet v: Int = %d\n", v) }

func req(pkg string, mode writelock.Mode) writelock.Request {
	return writelock.Request{Package: pkg, Mode: mode}
}

// ADR-0021 J4, J10: a journal on {b, z} met through shared b is settled exclusively, then re-read.
func TestBuildRecoversAbandonedJournal(t *testing.T) {
	dir := t.TempDir()
	settleTree(t, dir)
	s := &journalSettler{dir: dir, journal: []string{"b", "z"}, restore: func() error { return writeIn(dir, "b/b.canon", bSource(1)) }}
	p, err := Open(&probing{WriteFS: OS(), t: t}, filepath.ToSlash(dir), Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.WithSettle(s).Build(context.Background(), BuildOptions{Packages: []string{"a"}})
	if err != nil || res.Summary.Errors != 0 {
		t.Fatalf("ADR-0021 J10: %v %+v", err, res)
	}
	union := []writelock.Request{req("a", writelock.Exclusive), req("b", writelock.Exclusive), req("z", writelock.Exclusive)}
	first := []writelock.Request{req("a", writelock.Exclusive), req("b", writelock.Shared)}
	if len(s.meetings) < 3 || !slices.Equal(s.meetings[0], first) || !slices.Equal(s.meetings[1], union) {
		t.Errorf("ADR-0021 J4: meetings %v", s.meetings)
	}
	if len(s.settles) == 0 || !slices.Equal(s.settles[0], union) || len(s.free) > 0 {
		t.Errorf("ADR-0021 J4: settled under %v, journal packages free %v", s.settles, s.free)
	}
	if values := emitted(t, dir); !slices.Equal(values, []int{1}) {
		t.Errorf("ADR-0021 J10: the build wrote %v over the rolled-back edit", values)
	}
}

// emitted is the value of each data file a/out holds.
func emitted(t *testing.T, dir string) []int {
	t.Helper()
	out := filepath.Join(dir, "a", "out")
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	var values []int
	for _, e := range entries {
		var file struct{ Value int }
		data, err := os.ReadFile(filepath.Join(out, e.Name()))
		if err != nil || json.Unmarshal(data, &file) != nil {
			t.Fatal(e.Name(), err)
		}
		values = append(values, file.Value)
	}
	return values
}

// ADR-0021 J4: a Meeting or Settle that fails fails the build, every lock it took freed.
func TestBuildSettleFailureReleases(t *testing.T) {
	broken := errors.New("broken journal")
	for _, s := range []*journalSettler{{failMeet: broken}, {failSet: broken}} {
		dir := t.TempDir()
		settleTree(t, dir)
		s.dir = dir
		p, err := Open(OS(), filepath.ToSlash(dir), Options{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.WithSettle(s).Build(context.Background(), BuildOptions{}); !errors.Is(err, broken) {
			t.Errorf("ADR-0021 J4: %v", err)
		}
		h, ok, err := writelock.TryAcquire(dir, writelock.ExclusiveOf("a", "b"))
		if err != nil || !ok {
			t.Fatalf("ADR-0021 J4: locks kept after a failed settle: %v", err)
		}
		if err := h.Release(); err != nil {
			t.Fatal(err)
		}
	}
}

// ADR-0021 J10: a --check build settles nothing; Over keeps the Settler.
func TestBuildCheckSettlesNothing(t *testing.T) {
	dir := t.TempDir()
	settleTree(t, dir)
	s := &journalSettler{dir: dir, journal: []string{"b"}}
	p, err := Open(OS(), filepath.ToSlash(dir), Options{})
	if err != nil {
		t.Fatal(err)
	}
	q := p.WithSettle(s)
	if q.Over(q.FS()).settle != s || p.settle != nil {
		t.Error("ADR-0021 J10: Over drops the Settler, or WithSettle changed p")
	}
	if _, err := q.Build(context.Background(), BuildOptions{Check: true}); err != nil || len(s.meetings)+len(s.settles) > 0 {
		t.Errorf("ADR-0021 J10: a --check build settled: %v %v %v", err, s.meetings, s.settles)
	}
}

// keptJournal is a journal on b no Settle removes, met by builds on several goroutines: it
// counts the Settles running at once and those that found b free to a shared try-lock.
type keptJournal struct {
	dir     string
	running atomic.Int32
	overlap atomic.Int32
	free    atomic.Int32
	settled atomic.Int32
}

func (k *keptJournal) Meeting(held []writelock.Request) ([]writelock.Request, error) {
	if !slices.ContainsFunc(held, func(r writelock.Request) bool { return r.Package == "b" }) || slices.Contains(held, req("b", writelock.Exclusive)) {
		return held, nil
	}
	return append(slices.Clone(held), req("b", writelock.Exclusive)), nil
}

func (k *keptJournal) Settle([]writelock.Request) ([]string, error) {
	if k.running.Add(1) > 1 {
		k.overlap.Add(1)
	}
	defer k.running.Add(-1)
	k.settled.Add(1)
	h, ok, err := writelock.TryAcquire(k.dir, []writelock.Request{req("b", writelock.Shared)})
	if err != nil || ok {
		k.free.Add(1)
	}
	if ok {
		_ = h.Release() // the probe's; the test fails on k.free
	}
	time.Sleep(settleHold)
	return nil, nil
}

// settleHold keeps a Settle running long enough for another to overlap it, were b not exclusive.
const settleHold = 20 * time.Millisecond

// ADR-0021 J4: builds of a and c, both reading b shared, settle a journal on b one at a time.
func TestBuildSettlesSerialize(t *testing.T) {
	dir := t.TempDir()
	settleTree(t, dir)
	if err := os.MkdirAll(filepath.Join(dir, "c"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeIn(dir, "c/c.canon", "/// C.\npackage c\n\nimport b\n\n/// U.\nlet u: Int = b.v\n\nemit json { out: \"out/\", values: [u] }\n"); err != nil {
		t.Fatal(err)
	}
	k := &keptJournal{dir: dir}
	p, err := Open(OS(), filepath.ToSlash(dir), Options{})
	if err != nil {
		t.Fatal(err)
	}
	p = p.WithSettle(k)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, pkg := range []string{"a", "c"} {
		wg.Go(func() {
			_, errs[i] = p.Build(context.Background(), BuildOptions{Packages: []string{pkg}})
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil || k.settled.Load() != 2 || k.overlap.Load() > 0 || k.free.Load() > 0 {
		t.Errorf("ADR-0021 J4: %v; %d settles, %d overlapping, %d with b free", err, k.settled.Load(), k.overlap.Load(), k.free.Load())
	}
}
