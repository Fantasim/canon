package workspace

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
)

// holdFor is how long a test lets a call that must wait for a first read in flight run before
// it releases the read; the call returning sooner shows it did not wait.
const holdFor = 100 * time.Millisecond

// settleWait bounds every wait a test makes for a call that must end.
const settleWait = 10 * time.Second

// inflightGrown is memoPackage with other bytes of another size.
const inflightGrown = memoPackage + "\n/// M.\nlet m: Int = 2\n"

var errReadPanic = errors.New("a read that panics")

// inflightFS is keptFS whose first ReadFile of keptFile once armed takes the bytes there, closes
// reading and holds them until release is closed, then returns them, or panics when panics is
// set; reads counts keptFile's reads.
type inflightFS struct {
	*keptFS
	mu      sync.Mutex
	armed   bool
	panics  bool
	reads   int
	reading chan struct{}
	release chan struct{}
}

func (f *inflightFS) ReadFile(name string) ([]byte, error) {
	data, err := f.keptFS.ReadFile(name)
	if name != keptFile || !f.take() {
		return data, err
	}
	close(f.reading)
	<-f.release
	if f.panics {
		panic(errReadPanic)
	}
	return data, err
}

// take counts a read of keptFile and reports whether it is the one held, disarming.
func (f *inflightFS) take() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	held := f.armed
	f.armed = false
	return held
}

func (f *inflightFS) readsOf() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads
}

// inflightProject is a project over an inflightFS armed after its first Read, which read nothing.
func inflightProject(t *testing.T) (*Project, *inflightFS, *Snapshot) {
	t.Helper()
	fsys := &inflightFS{keptFS: newKeptFS(), reading: make(chan struct{}), release: make(chan struct{})}
	b, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(b)
	t.Cleanup(p.Close)
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	fsys.mu.Lock()
	fsys.armed, fsys.reads = true, 0
	fsys.mu.Unlock()
	return p, fsys, s
}

// readThrough reads keptFile through s on its own goroutine: its content, "" when it panicked.
func readThrough(s *Snapshot) <-chan string {
	out := make(chan string, 1)
	go func() {
		defer func() {
			if recover() != nil {
				out <- ""
			}
		}()
		data, _ := s.fs.ReadFile(keptFile)
		out <- string(data)
	}()
	return out
}

// firstRead is readThrough, returned once the base FS holds keptFile's bytes.
func firstRead(t *testing.T, s *Snapshot, fsys *inflightFS) <-chan string {
	t.Helper()
	out := readThrough(s)
	select {
	case <-fsys.reading:
	case <-time.After(settleWait):
		t.Fatal("the first read never reached the file system")
	}
	return out
}

// await is ch's value, failing t after settleWait.
func await[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(settleWait):
		t.Fatalf("%s never ended", what)
	}
	var zero T
	return zero
}

type readResult struct {
	s   *Snapshot
	err error
}

// readAsync runs p.Read(ctx) on its own goroutine.
func readAsync(ctx context.Context, p *Project) <-chan readResult {
	out := make(chan readResult, 1)
	go func() {
		s, err := p.Read(ctx)
		out <- readResult{s: s, err: err}
	}()
	return out
}

// held is ch's value if it comes within holdFor, then runs release whichever came first.
func held[T any](t *testing.T, ch <-chan T, release func()) T {
	t.Helper()
	select {
	case v := <-ch:
		release()
		return v
	case <-time.After(holdFor):
	}
	release()
	return await(t, ch, "the call")
}

// API.md S1, DECISIONS 421: a refresh overlapping a file's first read waits for it, so the
// entry it stores, read before the file changed, is compared with the disk: a suspect of that
// name, or a stamp changed, gives a new snapshot with the new bytes.
func TestRefreshWaitsForFirstRead(t *testing.T) {
	for _, c := range []struct {
		name    string
		content string
		suspect bool
	}{
		{"suspect kept stamp", keptChanged, true},
		{"suspect new size", inflightGrown, true},
		{"stamp changed", inflightGrown, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, fsys, s := inflightProject(t)
			a := firstRead(t, s, fsys)
			if err := fsys.WriteFile(keptFile, []byte(c.content)); err != nil {
				t.Fatal(err)
			}
			if c.suspect {
				p.Suspect(keptFile)
			}
			r := held(t, readAsync(context.Background(), p), func() { close(fsys.release) })
			if got := await(t, a, "the first read"); got != memoPackage {
				t.Fatalf("the first read gave %q", got)
			}
			if r.err != nil {
				t.Fatal(r.err)
			}
			data, err := r.s.fs.ReadFile(keptFile)
			if err != nil || string(data) != c.content || r.s == s {
				t.Errorf("API.md S1: the refresh kept the first read's old bytes %q (same snapshot %t)", data, r.s == s)
			}
		})
	}
}

// API.md S1, S10, E18: an edit's end, with no watch, compares the names it wrote in its planned
// snapshot once no first read is in flight there, so a first read of a written file, holding the
// bytes before the write, is read again and the published snapshot holds what was written.
func TestEditEndWaitsForFirstRead(t *testing.T) {
	p, fsys, s := inflightProject(t)
	planned := p.snapshot(s.fs.fork(s.fs.over, nil))
	a := firstRead(t, planned, fsys)
	after := func() wrote { return wrote{cause: CauseEdit, files: []string{keptFile}, planned: planned} }
	done := make(chan readResult, 1)
	go func() {
		next, err := p.writeAs(context.Background(), after, func(_ context.Context, w *Snapshot) error {
			return w.fs.WriteFile(keptFile, []byte(inflightGrown))
		})
		done <- readResult{s: next, err: err}
	}()
	r := held(t, done, func() { close(fsys.release) })
	if got := await(t, a, "the first read"); got != memoPackage {
		t.Fatalf("the first read gave %q", got)
	}
	if r.err != nil {
		t.Fatal(r.err)
	}
	if data, _ := r.s.fs.ReadFile(keptFile); string(data) != inflightGrown {
		t.Errorf("API.md S10: the edit published %q, the bytes before its write (planned snapshot %t)", data, r.s == planned)
	}
}

// API.md S1: two first uses of one name read it once, the second taking the first's entry.
func TestFirstReadOnce(t *testing.T) {
	_, fsys, s := inflightProject(t)
	a := firstRead(t, s, fsys)
	b := held(t, readThrough(s), func() { close(fsys.release) })
	if got := await(t, a, "the first read"); got != b || got != memoPackage {
		t.Errorf("API.md S1: the two first uses read %q and %q", got, b)
	}
	if n := fsys.readsOf(); n != 1 {
		t.Errorf("API.md S1: one name read %d times", n)
	}
}

// API.md S1, DECISIONS 421: a refresh cancelled while it waits for a first read returns the
// context's error and keeps the suspects; once the read ends nothing waits, and the next refresh
// reads the new bytes.
func TestRefreshWaitCancelled(t *testing.T) {
	p, fsys, s := inflightProject(t)
	a := firstRead(t, s, fsys)
	if err := fsys.WriteFile(keptFile, []byte(keptChanged)); err != nil {
		t.Fatal(err)
	}
	p.Suspect(keptFile)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := held(t, readAsync(ctx, p), cancel)
	if !errors.Is(r.err, context.Canceled) {
		t.Fatalf("API.md S1: a refresh cancelled while a first read is in flight returned %v", r.err)
	}
	close(fsys.release)
	await(t, a, "the first read")
	assertNextRead(t, p, keptChanged)
}

// API.md S1: a first read that panics still ends its flight, so the name's next first use
// reads it and a refresh does not wait for it (a pin: it fails only if a flight outlives its
// read).
func TestFirstReadPanicEnds(t *testing.T) {
	p, fsys, s := inflightProject(t)
	fsys.panics = true
	a := firstRead(t, s, fsys)
	b := held(t, readThrough(s), func() { close(fsys.release) })
	if got := await(t, a, "the first read"); got != "" {
		t.Fatalf("the first read did not panic: %q", got)
	}
	if b != memoPackage {
		t.Errorf("API.md S1: the second first use read %q", b)
	}
	assertNextRead(t, p, memoPackage)
}

// assertNextRead checks a Read, bounded by settleWait, returns a snapshot holding want at keptFile.
func assertNextRead(t *testing.T, p *Project, want string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), settleWait)
	defer cancel()
	next, err := p.Read(ctx)
	if err != nil {
		t.Fatalf("API.md S1: the next refresh failed: %v", err)
	}
	if data, _ := next.fs.ReadFile(keptFile); string(data) != want {
		t.Errorf("API.md S1: the next refresh read %q", data)
	}
}

// linkHoldFS is keptFS resolving every name to itself, its first resolution held as
// inflightFS holds a read.
type linkHoldFS struct {
	*keptFS
	held    atomic.Bool
	reading chan struct{}
	release chan struct{}
}

func (f *linkHoldFS) EvalSymlinks(name string) (string, error) {
	if f.held.CompareAndSwap(false, true) {
		close(f.reading)
		<-f.release
	}
	return name, nil
}

// resolveAsync resolves keptDir through s on its own goroutine.
func resolveAsync(s *snapFS) <-chan string {
	out := make(chan string, 1)
	go func() {
		real, _ := s.EvalSymlinks(keptDir)
		out <- real
	}()
	return out
}

// API.md S1: a link's second first use resolves it again instead of waiting, since a link's
// read reads other links, yet a refresh still waits for the first (a pin: it fails if a link
// waits, which two resolutions reading each other's links could deadlock).
func TestLinkFirstUseReadsAgain(t *testing.T) {
	fsys := &linkHoldFS{keptFS: newKeptFS(), reading: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	defer release.Do(func() { close(fsys.release) })
	s := newSnapFS(fsys, nil, time.Now)
	a := resolveAsync(s)
	await(t, fsys.reading, "the first resolution")
	if got := await(t, resolveAsync(s), "the second first use"); got != keptDir {
		t.Fatalf("the second first use resolved %q", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), holdFor)
	defer cancel()
	if err := s.settled(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("API.md S1: a refresh did not wait for a link's first resolution: %v", err)
	}
	release.Do(func() { close(fsys.release) })
	await(t, a, "the first resolution")
	if err := s.settled(context.Background()); err != nil {
		t.Error(err)
	}
}
