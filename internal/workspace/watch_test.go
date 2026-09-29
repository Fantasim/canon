package workspace_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/workspace"
	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// poll is the polling interval of these tests, half the quiet time of API.md W14.
const poll = 50 * time.Millisecond

// published records every event the project publishes, on the publishing goroutine.
type published struct {
	mu     sync.Mutex
	events []workspace.Event
}

func record(t *testing.T, p *workspace.Project) *published {
	t.Helper()
	r := &published{}
	t.Cleanup(p.Subscribe(func(e workspace.Event) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.events = append(r.events, e)
	}))
	return r
}

func (r *published) all() []workspace.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

// watched is the law project read, analyzed and revised, as a client leaves it before watching.
func watched(t *testing.T) (*memFS, *workspace.Project, *published) {
	t.Helper()
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	s := read(t, p)
	analyze(t, s)
	revision(t, s)
	return fsys, p, record(t, p)
}

// startWatch watches p on clk, every change sent to the channel returned; the watch is stopped
// and waited for when the test ends.
func startWatch(t *testing.T, p *workspace.Project, clk workspace.Clock) (<-chan workspace.Change, context.CancelFunc, *workspace.Watcher) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	changes := make(chan workspace.Change, 64)
	w, err := p.Watch(ctx, func(_ context.Context, c workspace.Change) { changes <- c }, workspace.WatchOptions{Clock: clk, Poll: poll})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		stopped(t, w)
	})
	return changes, cancel, w
}

func stopped(t *testing.T, w *workspace.Watcher) {
	t.Helper()
	select {
	case <-w.Done():
	case <-time.After(waitLimit):
		t.Fatal("the watch never stopped")
	}
}

func next(t *testing.T, changes <-chan workspace.Change) workspace.Change {
	t.Helper()
	select {
	case c := <-changes:
		return c
	case <-time.After(waitLimit):
		t.Fatal("no change delivered")
	}
	return workspace.Change{}
}

// shown is the display path of every file of c.
func shown(c workspace.Change) []string {
	var out []string
	for _, f := range c.Files {
		out = append(out, f.Display)
	}
	return out
}

func write(t *testing.T, fsys *memFS, name, data string) {
	t.Helper()
	if err := fsys.WriteFile(name, []byte(data)); err != nil {
		t.Fatal(err)
	}
}

func content(t *testing.T, s *workspace.Snapshot, name string) string {
	t.Helper()
	data, err := s.Build().FS().ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// API.md W14: changes less than 100 ms apart are one refresh, 100 ms after the last of them.
func TestWatchCoalescesBurst(t *testing.T) {
	fsys, p, pub := watched(t)
	clk := newFakeClock()
	changes, _, _ := startWatch(t, p, clk)
	clk.idle(t)
	write(t, fsys, "/law/a/a.json", "[3]\n")
	clk.step(t, poll)
	write(t, fsys, "/law/a/a.json", "[4]\n")
	clk.step(t, poll)
	write(t, fsys, "/law/b/b.canon", srcB+"\n")
	clk.step(t, poll)
	clk.step(t, poll)
	if n := len(pub.all()); n != 0 {
		t.Fatalf("%d refreshes before 100 ms without a change, want 0", n)
	}
	clk.step(t, poll)
	c := next(t, changes)
	if got := pub.all(); len(got) != 1 || got[0].Snapshot != c.Snapshot {
		t.Fatalf("%d refreshes for one burst, want 1", len(got))
	}
	if c.Cause != workspace.CauseExternal || !slices.Equal(shown(c), []string{"a/a.json", "b/b.canon"}) ||
		content(t, c.Snapshot, "/law/a/a.json") != "[4]\n" {
		t.Errorf("change %s %v", c.Cause, shown(c))
	}
	if !slices.Contains(c.Changed, "/law/a/a.json") || !slices.Contains(c.Changed, "/law/b/b.canon") {
		t.Errorf("changed names %v", c.Changed)
	}
}

// API.md W14: a change that never goes quiet is refreshed 1 s after its first change, then the
// next window starts.
func TestWatchCapsStream(t *testing.T) {
	fsys, p, pub := watched(t)
	clk := newFakeClock()
	changes, _, _ := startWatch(t, p, clk)
	clk.idle(t)
	steps := int(time.Second / poll)
	for i := range steps + 1 {
		if n := len(pub.all()); n != 0 {
			t.Fatalf("refreshed %d times after %d polls of a stream, want none before 1 s", n, i)
		}
		write(t, fsys, "/law/a/a.json", fmt.Sprintf("[%d]\n", i))
		clk.step(t, poll)
	}
	c := next(t, changes)
	if n := len(pub.all()); n != 1 || content(t, c.Snapshot, "/law/a/a.json") != fmt.Sprintf("[%d]\n", steps) {
		t.Errorf("%d refreshes at 1 s, snapshot holds %q", n, content(t, c.Snapshot, "/law/a/a.json"))
	}
	write(t, fsys, "/law/a/a.json", "[]\n")
	clk.step(t, poll)
	clk.step(t, poll)
	clk.step(t, poll)
	if c := next(t, changes); len(pub.all()) != 2 || content(t, c.Snapshot, "/law/a/a.json") != "[]\n" {
		t.Errorf("the stream after the cap: %d refreshes", len(pub.all()))
	}
}

// API.md W13: changes arrive one at a time, in publishing order, even while fn is slow; an
// external change queued behind another joins it, an edit never does (W15).
func TestWatchRevisionOrder(t *testing.T) {
	fsys, p, pub := watched(t)
	clk := newFakeClock()
	release := make(chan struct{})
	var busy, overlap atomic.Int32
	changes := make(chan workspace.Change, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w, err := p.Watch(ctx, func(_ context.Context, c workspace.Change) {
		if busy.Add(1) > 1 {
			overlap.Add(1)
		}
		defer busy.Add(-1)
		changes <- c
		<-release
	}, workspace.WatchOptions{Clock: clk, Poll: poll})
	if err != nil {
		t.Fatal(err)
	}
	clk.idle(t)
	settle := func(name, data string) {
		write(t, fsys, name, data)
		clk.step(t, poll)
		clk.step(t, poll)
		clk.step(t, poll)
	}
	settle("/law/a/a.json", "[5]\n")
	first := next(t, changes)
	if err := p.SetOverlay("b/b.canon", []byte(srcB+"\n")); err != nil {
		t.Fatal(err)
	}
	settle("/law/data/c.json", "[6]\n")
	settle("/law/a/a.json", "[7]\n")
	for _, data := range []string{"[8]\n", "[9]\n"} {
		if _, err := p.Write(ctx, workspace.CauseEdit, func(_ context.Context, s *workspace.Snapshot) error {
			return s.Build().FS().(build.WriteFS).WriteFile("/law/a/a.json", []byte(data))
		}); err != nil {
			t.Fatal(err)
		}
	}
	close(release)
	events := pub.all()
	if len(events) != 6 {
		t.Fatalf("%d snapshots published, want 6", len(events))
	}
	got := []workspace.Change{first, next(t, changes), next(t, changes), next(t, changes), next(t, changes)}
	want := []*workspace.Snapshot{events[0].Snapshot, events[1].Snapshot, events[3].Snapshot, events[4].Snapshot, events[5].Snapshot}
	causes := []workspace.Cause{workspace.CauseExternal, workspace.CauseOverlay, workspace.CauseExternal, workspace.CauseEdit, workspace.CauseEdit}
	for i, c := range got {
		if c.Snapshot != want[i] || c.Cause != causes[i] {
			t.Errorf("change %d: %s, not the snapshot published %d-th", i, c.Cause, i)
		}
	}
	if files := shown(got[2]); len(files) != 2 || !slices.Contains(files, "a/a.json") || !slices.Contains(files, "@data/c.json") {
		t.Errorf("joined change files %v", files)
	}
	cancel()
	stopped(t, w)
	if overlap.Load() != 0 {
		t.Error("fn ran concurrently with itself")
	}
}

// API.md W15: a writer's own writes are one event with its cause, never an external one, even
// when the watcher saw them while the writer ran; an overlay is an overlay event.
func TestWatchOwnWrites(t *testing.T) {
	_, p, pub := watched(t)
	clk := newFakeClock()
	changes, _, _ := startWatch(t, p, clk)
	clk.idle(t)
	wrote, release, done := make(chan struct{}), make(chan struct{}), make(chan error)
	go func() {
		_, err := p.Write(context.Background(), workspace.CauseEdit, func(_ context.Context, s *workspace.Snapshot) error {
			err := s.Build().FS().(build.WriteFS).WriteFile("/law/b/b.canon", []byte(srcB+"\n"))
			close(wrote)
			<-release
			return err
		})
		done <- err
	}()
	<-wrote
	clk.step(t, poll)                // the poll sees the write
	clk.advance(quietForTest + poll) // the refresh is due: it waits for the writer
	if clk.idleWithin(quietForTest) || len(pub.all()) != 0 {
		t.Fatal("the watcher refreshed while a writer ran")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	clk.idle(t)
	for range 4 {
		clk.step(t, poll)
	}
	c := next(t, changes)
	if c.Cause != workspace.CauseEdit || !slices.Equal(shown(c), []string{"b/b.canon"}) {
		t.Errorf("the writer's change: %s %v", c.Cause, shown(c))
	}
	if err := p.SetOverlay("a/a.canon", []byte(srcA+"\n")); err != nil {
		t.Fatal(err)
	}
	if c := next(t, changes); c.Cause != workspace.CauseOverlay || !slices.Equal(shown(c), []string{"a/a.canon"}) {
		t.Errorf("the overlay's change: %s %v", c.Cause, shown(c))
	}
	causes := causesOf(pub.all())
	if !slices.Equal(causes, []workspace.Cause{workspace.CauseEdit, workspace.CauseOverlay}) {
		t.Errorf("published %v, want one edit then one overlay", causes)
	}
}

// quietForTest is API.md W14's quiet time.
const quietForTest = 100 * time.Millisecond

// API.md W16: every watch receives every event; one ending leaves the others watching, and once
// the last ends, a read refreshes again (S1).
func TestTwoWatches(t *testing.T) {
	fsys, p, pub := watched(t)
	clk := newFakeClock()
	one, stopOne, w1 := startWatch(t, p, clk)
	two, _, _ := startWatch(t, p, clk)
	clk.idle(t)
	write(t, fsys, "/law/a/a.json", "[8]\n")
	for range 3 {
		clk.step(t, poll)
	}
	if a, b := next(t, one), next(t, two); a.Snapshot != b.Snapshot || len(pub.all()) != 1 {
		t.Error("two watches received different events")
	}
	stopOne()
	stopped(t, w1)
	write(t, fsys, "/law/a/a.json", "[9]\n")
	for range 3 {
		clk.step(t, poll)
	}
	if c := next(t, two); content(t, c.Snapshot, "/law/a/a.json") != "[9]\n" {
		t.Error("the remaining watch missed a change")
	}
	select {
	case c := <-one:
		t.Errorf("a stopped watch received %v", shown(c))
	default:
	}
}

// API.md S1: while a watch runs, a read takes the snapshot the watcher keeps, not the disk; the
// watch ended, a read refreshes again.
func TestWatchKeepsSnapshot(t *testing.T) {
	fsys, p, _ := watched(t)
	clk := newFakeClock()
	changes, cancel, w := startWatch(t, p, clk)
	clk.idle(t)
	before := read(t, p)
	write(t, fsys, "/law/a/a.json", "[10]\n")
	if read(t, p) != before {
		t.Error("a read refreshed while a watch keeps the snapshot")
	}
	for range 3 {
		clk.step(t, poll)
	}
	if c := next(t, changes); read(t, p) != c.Snapshot {
		t.Error("a read does not take the snapshot the watcher published")
	}
	cancel()
	stopped(t, w)
	write(t, fsys, "/law/a/a.json", "[11]\n")
	if s := read(t, p); content(t, s, "/law/a/a.json") != "[11]\n" {
		t.Error("a read after the watch did not refresh")
	}
}

// API.md S2: a file created is a file that changed; its listing is a name changed.
func TestWatchCreatedFile(t *testing.T) {
	fsys, p, _ := watched(t)
	clk := newFakeClock()
	changes, _, _ := startWatch(t, p, clk)
	clk.idle(t)
	write(t, fsys, "/law/a/more.canon", "/// A.\npackage a\n")
	for range 3 {
		clk.step(t, poll)
	}
	c := next(t, changes)
	if !slices.Contains(shown(c), "a/more.canon") || !slices.Contains(c.Changed, "/law/a") {
		t.Errorf("created file: files %v, changed %v", shown(c), c.Changed)
	}
}

// API.md W12, O6: a watch cannot start on a done ctx or a closed project; cancelling it or
// closing the project stops it and the watcher, leaving no goroutine behind.
func TestWatchStops(t *testing.T) {
	_, p, _ := watched(t)
	clk := newFakeClock()
	done, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Watch(done, func(context.Context, workspace.Change) {}, workspace.WatchOptions{Clock: clk}); !errors.Is(err, context.Canceled) {
		t.Errorf("Watch on a done ctx: %v", err)
	}
	base := runtime.NumGoroutine()
	_, _, w1 := startWatch(t, p, clk)
	_, _, w2 := startWatch(t, p, clk)
	clk.idle(t)
	p.Close()
	stopped(t, w1)
	stopped(t, w2)
	if _, err := p.Watch(context.Background(), func(context.Context, workspace.Change) {}, workspace.WatchOptions{Clock: clk}); !errors.Is(err, workspace.ErrClosed) {
		t.Errorf("Watch after Close: %v", err)
	}
	settled(t, base)
}

// settled waits for the goroutines to come back to base.
func settled(t *testing.T, base int) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for runtime.NumGoroutine() > base {
		if time.Now().After(deadline) {
			t.Fatalf("%d goroutines left, %d before", runtime.NumGoroutine(), base)
		}
		time.Sleep(time.Millisecond)
	}
}

// A panic in fn ends its watch cleanly: its goroutines stop and the project refreshes again.
func TestWatchPanicEnds(t *testing.T) {
	fsys, p, _ := watched(t)
	clk := newFakeClock()
	base := runtime.NumGoroutine()
	w, err := p.Watch(context.Background(), func(context.Context, workspace.Change) { panic("fn") }, workspace.WatchOptions{Clock: clk, Poll: poll})
	if err != nil {
		t.Fatal(err)
	}
	clk.idle(t)
	write(t, fsys, "/law/a/a.json", "[12]\n")
	clk.step(t, poll)
	clk.step(t, poll)
	clk.advance(poll)
	stopped(t, w)
	settled(t, base)
	write(t, fsys, "/law/a/a.json", "[13]\n")
	if s := read(t, p); content(t, s, "/law/a/a.json") != "[13]\n" {
		t.Error("a read after the watch ended did not refresh")
	}
}

// API.md X2, W12 (log-2026-09-29 M4 U5-r2): a panic of the shared watcher is a change with its
// error to every watch, and a fresh watcher replaces it at once: changes keep coming, no new
// Watch needed, and a Watch that joins shares it.
func TestWatchHubPanics(t *testing.T) {
	fsys, p, pub := watched(t)
	clk := newFakeClock()
	one, _, _ := startWatch(t, p, clk)
	clk.idle(t)
	clk.boom.Store(true)
	clk.advance(poll)
	if c := next(t, one); !errors.Is(c.Err, safego.ErrPanic) || c.Snapshot == nil {
		t.Fatalf("the watcher's panic: %v", c.Err)
	}
	clk.idle(t)
	before := read(t, p)
	write(t, fsys, "/law/a/a.json", "[14]\n")
	if read(t, p) != before {
		t.Error("a read refreshed: nothing keeps the snapshot fresh after the panic")
	}
	for range 3 {
		clk.step(t, poll)
	}
	if c := next(t, one); c.Err != nil || content(t, c.Snapshot, "/law/a/a.json") != "[14]\n" {
		t.Errorf("a change after the panic: %v", c.Err)
	}
	two, _, _ := startWatch(t, p, clk)
	clk.idle(t)
	write(t, fsys, "/law/a/a.json", "[15]\n")
	for range 3 {
		clk.step(t, poll)
	}
	if a, b := next(t, one), next(t, two); a.Snapshot != b.Snapshot || len(pub.all()) != 2 {
		t.Error("the watches do not share the fresh watcher")
	}
}
