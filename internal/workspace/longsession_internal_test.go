package workspace

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

const (
	longRule     = "API.md S4, DECISIONS 445: "
	longEdits    = 300 // edits with distinct values on one Project
	longSettled  = 10  // the edits after which the sizes are taken as the session's own
	longSlack    = 8   // what the entries and the history tip may gain past them
	longSrcLeft  = "/// a.\npackage a\n\n/// N.\nconst N = "
	longSrcRight = "\n"
	pinData      = "x\n"
)

// longSession is a project of one package on the OS's files, its constant N set to 0.
func longSession(t *testing.T) *Project {
	t.Helper()
	dir := filepath.ToSlash(t.TempDir())
	files := map[string]string{"project.canon": "project acme {\n  canon: \"0.1\"\n}\n", "a/a.canon": longSrcLeft + "0" + longSrcRight}
	for rel, text := range files {
		name := filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := build.Open(build.OS(), dir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(b)
	t.Cleanup(p.Close)
	return p
}

// setTo is the edit setting a's N to n, a value no earlier edit wrote.
func setTo(n int64) EditRequest {
	return EditRequest{Changes: Changes{Host: build.EditHost, Ops: []edit.Operation{{Kind: edit.OpSet, Path: "a:N", Value: edit.Int(n)}}}}
}

// sizes is how many entries the current snapshot holds and how many the history's tip.
func (p *Project) sizes() (int, int) {
	p.mu.Lock()
	cur := p.cur
	tip := len(p.hist.tip)
	p.mu.Unlock()
	cur.fs.mu.Lock()
	defer cur.fs.mu.Unlock()
	return len(cur.fs.ents), tip
}

// editRead is one edit of the session, then what a client does with its result: the revision it
// reads and the next call's refresh.
func editRead(t *testing.T, p *Project, n int64) {
	t.Helper()
	ctx := context.Background()
	out, err := p.Edit(ctx, setTo(n))
	if err != nil || !out.Applied {
		t.Fatalf("edit %d: %+v, %v", n, out, err)
	}
	if _, err := out.After.Revision(ctx); err != nil {
		t.Fatal(err)
	}
	s, err := p.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Revision(ctx); err != nil {
		t.Fatal(err)
	}
}

// reading reads the project and its revision until stop closes, then reports on done: a reader
// beside the writer, as a studio's server has (API.md S7, S8).
func reading(p *Project, stop <-chan struct{}, done chan<- error) {
	ctx := context.Background()
	for {
		select {
		case <-stop:
			done <- nil
			return
		default:
		}
		s, err := p.Read(ctx)
		if err == nil {
			_, err = s.Revision(ctx)
		}
		if err != nil {
			done <- err
			return
		}
	}
}

// API.md S4, DECISIONS 445: a long-lived Project editing with distinct values, every edit staged
// under a new name (DECISIONS 351), holds no more entries, and remembers no larger tip, after
// longEdits edits than after longSettled, give or take longSlack, a reader running beside it.
func TestLongSessionStaysBounded(t *testing.T) {
	p := longSession(t)
	stop, done := make(chan struct{}), make(chan error, 1)
	go reading(p, stop, done)
	defer func() {
		close(stop)
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	var ents, tip int
	for n := int64(1); n <= longEdits; n++ {
		editRead(t, p, n)
		if n == longSettled {
			ents, tip = p.sizes()
		}
	}
	gotEnts, gotTip := p.sizes()
	t.Logf("after %d edits: %d entries, tip %d; after %d: %d and %d", longEdits, gotEnts, gotTip, longSettled, ents, tip)
	if gotEnts > ents+longSlack || gotTip > tip+longSlack {
		t.Errorf("%safter %d edits the snapshot holds %d entries and the tip %d; after %d, %d and %d", longRule, longEdits, gotEnts, gotTip, longSettled, ents, tip)
	}
	if got := osText(p.tmpl.Dir(), "a/a.canon"); got != longSrcLeft+strconv.Itoa(longEdits)+longSrcRight {
		t.Errorf("the last edit wrote %q", got)
	}
}

// API.md S4, DECISIONS 445: with a Watch active, whose refreshes and diffs read every snapshot,
// the session stays as bounded.
func TestLongSessionWatchedStaysBounded(t *testing.T) {
	p := longSession(t)
	fake := &fakeNotifier{ev: make(chan fsnotify.Event), er: make(chan error)}
	ctx, cancel := context.WithCancel(context.Background())
	w, err := p.Watch(ctx, func(context.Context, Change) {}, WatchOptions{OS: true, notify: func() (notifier, error) { return fake, nil }})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	defer func() { cancel(); <-w.Done() }()
	var ents, tip int
	for n := int64(1); n <= longEdits; n++ {
		editRead(t, p, n)
		if n == longSettled {
			ents, tip = p.sizes()
		}
	}
	gotEnts, gotTip := p.sizes()
	t.Logf("watched, after %d edits: %d entries, tip %d; after %d: %d and %d", longEdits, gotEnts, gotTip, longSettled, ents, tip)
	if gotEnts > ents+longSlack || gotTip > tip+longSlack {
		t.Errorf("%swatched, after %d edits the snapshot holds %d entries and the tip %d; after %d, %d and %d", longRule, longEdits, gotEnts, gotTip, longSettled, ents, tip)
	}
}

// pinDir is an empty temporary directory and a snapshot's file system over the OS's files there.
func pinDir(t *testing.T) (string, *snapFS) {
	t.Helper()
	dir := filepath.ToSlash(t.TempDir())
	s := newSnapFS(build.OS(), nil, time.Now)
	s.root = dir
	return dir, s
}

// writeGone writes abs through s, as a writer stages a file, and removes it.
func writeGone(t *testing.T, s *snapFS, abs string) {
	t.Helper()
	if err := s.WriteFile(abs, []byte(pinData)); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(abs); err != nil {
		t.Fatal(err)
	}
}

// API.md S4, S9, DECISIONS 445: a name only a writer pinned that names nothing at the refresh is
// not in the next snapshot and is no change; a name a reader asked for, before the writer or
// after it, stays; a name the writer left existing stays; the older snapshot keeps every entry.
func TestPinnedNameLeaves(t *testing.T) {
	dir, s := pinDir(t)
	stage, before, after, out := dir+"/stage", dir+"/before", dir+"/after", dir+"/out"
	if _, err := s.ReadFile(before); err == nil {
		t.Fatal("before exists")
	}
	for _, abs := range []string{stage, before, after} {
		writeGone(t, s, abs)
	}
	if _, err := s.Stat(after); err == nil {
		t.Fatal("after exists")
	}
	if err := s.WriteFile(out, []byte(pinData)); err != nil {
		t.Fatal(err)
	}
	next, changed, err := s.refresh(context.Background(), nil)
	if err != nil || next == nil {
		t.Fatalf("the output written is no change: %v", err)
	}
	for _, n := range changed {
		if n.abs == stage {
			t.Errorf("%sthe stage left is a change: %v", longRule, changed)
		}
	}
	for _, n := range []name{{kind: kindFile, abs: stage}, {kind: kindStat, abs: stage}} {
		if next.holds(n) {
			t.Errorf("%sthe next snapshot holds %v, which only a writer pinned and names nothing", longRule, n)
		}
		if !s.holds(n) {
			t.Errorf("API.md S9: the older snapshot lost %v", n)
		}
	}
	for _, n := range []name{{kind: kindFile, abs: before}, {kind: kindStat, abs: after}, {kind: kindFile, abs: out}} {
		if !next.holds(n) {
			t.Errorf("%sthe next snapshot lost %v, a name read or existing", longRule, n)
		}
	}
}

// API.md S1, S4, DECISIONS 445: a refresh that finds only such a name is no new snapshot; the
// next snapshot forked leaves it, and the history records it gone.
func TestPinnedNameLeavesAtNextFork(t *testing.T) {
	dir, s := pinDir(t)
	stage := dir + "/stage"
	var h history
	h.add("r0", s)
	writeGone(t, s, stage)
	h.add("r0", s)
	n := name{kind: kindFile, abs: stage}
	if _, ok := h.tip[n]; !ok {
		t.Fatal("the tip lacks the stage pinned")
	}
	if next, changed, err := s.refresh(context.Background(), nil); next != nil || len(changed) != 0 || err != nil {
		t.Fatalf("API.md S1: the stage left made a snapshot: %v, %v", changed, err)
	}
	next := s.fork(s.over, nil)
	if next.holds(n) || !s.holds(n) {
		t.Errorf("%sthe fork holds the stage %v, its parent %v", longRule, next.holds(n), s.holds(n))
	}
	h.add("r1", next)
	if _, ok := h.tip[n]; ok {
		t.Errorf("%sthe history's tip still holds the stage", longRule)
	}
}

// API.md S1, S4, DECISIONS 445: a file only a writer pinned, there after the write and removed
// since, is still a change the refresh names, and the next snapshot leaves it; the watcher's
// fingerprint of that refresh reads it as missing.
func TestPinnedNameRemovedIsAChange(t *testing.T) {
	dir, s := pinDir(t)
	out := dir + "/out"
	if err := s.WriteFile(out, []byte(pinData)); err != nil {
		t.Fatal(err)
	}
	next, _, err := s.refresh(context.Background(), nil)
	if err != nil || next == nil {
		t.Fatalf("the output written is no change: %v", err)
	}
	if err := os.Remove(filepath.FromSlash(out)); err != nil {
		t.Fatal(err)
	}
	last, changed, err := next.refresh(context.Background(), nil)
	n := name{kind: kindFile, abs: out}
	if err != nil || last == nil || !slices.Contains(changed, n) {
		t.Fatalf("API.md S1: the output removed is no change: %v, %v", changed, err)
	}
	if last.holds(n) || !next.holds(n) {
		t.Errorf("%sthe next snapshot holds the output removed %v, the older %v", longRule, last.holds(n), next.holds(n))
	}
	if fingerprint(last, changed) == fingerprint(next, nil) {
		t.Error("the fingerprint of a change is that of none")
	}
}

// checked is every entry s holds but the overlays, and each checked against the disk as a
// refresh checks it (compare): the first half of a refresh, before it decides.
func checked(s *snapFS) (names []name, olds, fresh []*entry) {
	s.mu.Lock()
	for n, e := range s.ents {
		if !e.over {
			names, olds = append(names, n), append(olds, e)
		}
	}
	s.mu.Unlock()
	for i, n := range names {
		fresh = append(fresh, rechecks[n.kind](s, n, olds[i], false))
	}
	return names, olds, fresh
}

// API.md S4, DECISIONS 445: a name a refresh found naming nothing that a reader reads before the
// refresh forks is a read name, which the fork carries.
func TestPinnedNameReadBeforeFork(t *testing.T) {
	dir, s := pinDir(t)
	stage, out := dir+"/stage", dir+"/out"
	writeGone(t, s, stage)
	if err := s.WriteFile(out, []byte(pinData)); err != nil {
		t.Fatal(err)
	}
	names, olds, fresh := checked(s)
	gone := s.goneOf(names, olds, fresh)
	n := name{kind: kindFile, abs: stage}
	if !slices.Contains(gone, n) {
		t.Fatalf("the refresh did not find the stage naming nothing: %v", gone)
	}
	if _, err := s.ReadFile(stage); err == nil {
		t.Fatal("the stage exists")
	}
	next, _ := s.successor(map[name]*entry{{kind: kindFile, abs: out}: s.readFile(out)}, gone)
	if s.onlyPinned(n) || !next.holds(n) {
		t.Errorf("%sa stage read before the fork: only pinned %v, the fork holds it %v", longRule, s.onlyPinned(n), next.holds(n))
	}
}
