package edit_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// fakeLocker is a Locker whose packages others hold (busy) or this site took (taken); onLock runs
// once a TryLock succeeded, before it returns.
type fakeLocker struct {
	mu     sync.Mutex
	busy   map[string]bool
	taken  map[string]bool
	calls  [][]string
	onLock func()
}

func newLocker(busy ...string) *fakeLocker {
	f := &fakeLocker{busy: map[string]bool{}, taken: map[string]bool{}}
	for _, p := range busy {
		f.busy[p] = true
	}
	return f
}

func (f *fakeLocker) TryLock(pkgs []string) (func() error, bool, error) {
	f.mu.Lock()
	f.calls = append(f.calls, slices.Clone(pkgs))
	held := func(p string) bool { return f.busy[p] || f.taken[p] }
	if slices.ContainsFunc(pkgs, held) {
		f.mu.Unlock()
		return nil, false, nil
	}
	for _, p := range pkgs {
		f.taken[p] = true
	}
	f.mu.Unlock()
	if f.onLock != nil {
		f.onLock()
	}
	return func() error {
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, p := range pkgs {
			delete(f.taken, p)
		}
		return nil
	}, true, nil
}

// lockSite is siteOf(fsys) taking writer locks through l, holding held exclusive.
func lockSite(fsys build.WriteFS, l edit.Locker, held ...string) edit.Site {
	s := siteOf(fsys)
	s.Locks, s.Held = l, held
	return s
}

// crashedHolding is the crashed fixture, its commit holding held: a version-2 journal recording them.
func crashedHolding(t *testing.T, held ...string) (d *diskFS, before string) {
	t.Helper()
	d, changes := commitFixture()
	before = d.state()
	site := lockSite(&faultFS{diskFS: d, dieAfterRename: 2}, newLocker(), held...)
	if err := edit.Commit(t.Context(), site, testRev, changes); !errors.Is(err, errDead) {
		t.Fatalf("err = %v", err)
	}
	if len(journalsIn(d)) != 1 {
		t.Fatal("the crash left no journal")
	}
	return d, before
}

// rewrite changes the journal at name as mutate says.
func rewrite(t *testing.T, d *diskFS, name string, mutate func(map[string]any)) {
	t.Helper()
	var j map[string]any
	if err := json.Unmarshal(d.files[name], &j); err != nil {
		t.Fatal(err)
	}
	mutate(j)
	data, _ := json.Marshal(j)
	d.files[name] = data
}

// API.md O5, N11: a version-2 journal on {a}, its writer's pid dead, is left alone while a is
// held, nothing logged and Open's error nil; once a is free it is rolled back.
func TestRecoverLeavesHeldJournal(t *testing.T) {
	d, before := crashedHolding(t, "a")
	crash := d.state()
	l := newLocker("a")
	lines, err := warningsAt(t, lockSite(d, l))
	if err != nil || len(lines) != 0 {
		t.Fatalf("logged %v, err %v", lines, err)
	}
	mustState(t, d.state(), crash)
	delete(l.busy, "a")
	if lines, err = warningsAt(t, lockSite(d, l)); err != nil || len(lines) != 1 {
		t.Fatalf("logged %v, err %v", lines, err)
	}
	mustState(t, d.state(), before)
	if len(l.taken) != 0 || !slices.Equal(l.calls[1], []string{"a"}) {
		t.Fatalf("taken %v, calls %v", l.taken, l.calls)
	}
}

// API.md N11 (DECISIONS 375: J3): a journal on {a, b} with b held is left; the locks are asked for all
// at once and none stays taken.
func TestRecoverAllOrNoneLocks(t *testing.T) {
	d, _ := crashedHolding(t, "b", "a")
	crash := d.state()
	l := newLocker("b")
	if err := edit.Recover(lockSite(d, l), nil); err != nil {
		t.Fatal(err)
	}
	mustState(t, d.state(), crash)
	if len(l.taken) != 0 || len(l.calls) != 1 || !slices.Equal(l.calls[0], []string{"a", "b"}) {
		t.Fatalf("taken %v, calls %v", l.taken, l.calls)
	}
}

// API.md N11 (DECISIONS 375: J3): Recover reads the journal again under its locks: one removed
// meanwhile (another recovery finished) is skipped, one whose locks changed is left.
func TestRecoverRereadsUnderLocks(t *testing.T) {
	cases := []struct {
		name   string
		onLock func(d *diskFS)
	}{
		{"gone", func(d *diskFS) { _ = d.Remove(journalFile) }},
		{"locks changed", func(d *diskFS) {
			rewriteLocked(d, func(j map[string]any) { j["locks"] = []any{"a", "c"} })
		}},
		{"host changed", func(d *diskFS) {
			rewriteLocked(d, func(j map[string]any) { j["host"] = "host-b" })
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, _ := crashedHolding(t, "a")
			l := newLocker()
			l.onLock = func() { tc.onLock(d) }
			crash := d.state()
			lines, err := warningsAt(t, lockSite(d, l))
			if err != nil || len(lines) != 0 {
				t.Fatalf("logged %v, err %v", lines, err)
			}
			if got := d.state(); !strings.Contains(crash, "a new") || !strings.Contains(got, "a new") {
				t.Fatalf("rolled back:\n%s", got)
			}
		})
	}
}

// rewriteLocked is rewrite under d's own lock, for a locker callback.
func rewriteLocked(d *diskFS, mutate func(map[string]any)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var j map[string]any
	_ = json.Unmarshal(d.files[journalFile], &j)
	mutate(j)
	d.files[journalFile], _ = json.Marshal(j)
}

// API.md N11 (DECISIONS 375: J2): a lock list not of sorted unique package names, empty, null, or in a
// version-1 journal refuses the journal whole: ErrJournal, kept, the locker never asked.
func TestRecoverBadLockList(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"empty", setKey("locks", []any{})},
		{"null", setKey("locks", nil)},
		{"unsorted", setKey("locks", []any{"b", "a"})},
		{"twice", setKey("locks", []any{"a", "a"})},
		{"hidden", setKey("locks", []any{".a"})},
		{"slash", setKey("locks", []any{"a/b"})},
		{"version 1", setKey("version", 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, _ := crashedHolding(t, "a")
			rewrite(t, d, journalFile, tc.mutate)
			before := d.state()
			l := newLocker()
			lines, err := warningsAt(t, lockSite(d, l))
			if !errors.Is(err, edit.ErrJournal) || !strings.Contains(err.Error(), "invalid lock list") || len(lines) != 0 {
				t.Fatalf("logged %v, err %v", lines, err)
			}
			mustState(t, d.state(), before)
			if len(l.calls) != 0 {
				t.Fatalf("locker asked %v", l.calls)
			}
		})
	}
}

// DECISIONS 375 (J2, J9): a journal recording no lock (version 1, or 2 without the field) follows the
// pid rule as before, whatever locker the site has: left while its writer runs, else rolled back.
func TestRecoverVersion1ByPID(t *testing.T) {
	for _, version := range []int{1, 2} {
		d, before := crashed(t)
		rewrite(t, d, journalFile, setKey("version", version))
		crash := d.state()
		l := newLocker()
		running := lockSite(d, l)
		running.Self.Alive = func(pid int) bool { return pid == testPID }
		if err := edit.Recover(running, nil); err != nil {
			t.Fatal(err)
		}
		mustState(t, d.state(), crash)
		if err := edit.Recover(lockSite(d, l), nil); err != nil {
			t.Fatal(err)
		}
		mustState(t, d.state(), before)
		if len(l.calls) != 0 {
			t.Fatalf("version %d: locker asked %v", version, l.calls)
		}
	}
}

// DECISIONS 375 (J2a): a site with no Locker never applies a journal recording locks: Recover leaves it.
func TestRecoverNoLockerLeavesLocked(t *testing.T) {
	d, _ := crashedHolding(t, "a")
	crash := d.state()
	if err := edit.Recover(siteOf(d), nil); err != nil {
		t.Fatal(err)
	}
	mustState(t, d.state(), crash)
}

// API.md N11 (DECISIONS 375: J3): two recoveries of one journal at once, under one set of locks: one
// rolls it back, the other leaves or skips it; neither fails.
func TestRecoverConcurrentOnce(t *testing.T) {
	for range 20 {
		d, before := crashedHolding(t, "a", "b")
		l := newLocker()
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i := range errs {
			wg.Go(func() { errs[i] = edit.Recover(lockSite(d, l), nil) })
		}
		wg.Wait()
		if err := errors.Join(errs...); err != nil {
			t.Fatal(err)
		}
		if err := edit.Recover(lockSite(d, l), nil); err != nil {
			t.Fatal(err)
		}
		mustState(t, d.state(), before)
	}
}
