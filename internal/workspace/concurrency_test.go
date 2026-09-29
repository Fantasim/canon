package workspace_test

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/workspace"
	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// prompt bounds how long a call may take to return once its ctx is done (API.md S11).
const prompt = 5 * time.Second

// API.md S8: identical concurrent reads of one snapshot share one computation; another key, or a
// call after it finished, runs its own.
func TestShareOneComputation(t *testing.T) {
	p := open(t, newMemFS(lawFiles()))
	s := read(t, p)
	var runs atomic.Int32
	release := make(chan struct{})
	fn := func(context.Context) (*int, error) {
		runs.Add(1)
		<-release
		v := 1
		return &v, nil
	}
	key := workspace.Key(workspace.OpAnalyze, []string{"b", "a"})
	results := make(chan *int, 3)
	var wg sync.WaitGroup
	for _, k := range []string{key, workspace.Key(workspace.OpAnalyze, []string{"b", "a"}), key} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := workspace.Share(context.Background(), s, k, fn)
			if err != nil {
				t.Error(err)
			}
			results <- v
		}()
	}
	for runs.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)
	close(release)
	wg.Wait()
	close(results)
	first := <-results
	for v := range results {
		if v != first {
			t.Error("two identical calls got different results")
		}
	}
	if runs.Load() != 1 {
		t.Errorf("%d computations for identical calls, want 1", runs.Load())
	}
	if _, err := workspace.Share(context.Background(), s, key, fn); err != nil || runs.Load() != 2 {
		t.Errorf("a finished computation was shared again: %d runs, %v", runs.Load(), err)
	}
}

// API.md S8: a key keeps its parts apart and the selection's order (DOCTRINE §5).
func TestKeyIsUnambiguous(t *testing.T) {
	a := workspace.OpAnalyze
	keys := []string{
		workspace.Key(a, nil),
		workspace.Key(a, []string{""}),
		workspace.Key(a, []string{"", ""}),
		workspace.Key(a, []string{"a", "b"}),
		workspace.Key(a, []string{"b", "a"}),
		workspace.Key(a, []string{"a\x00b"}),
		workspace.Key(a, []string{"a"}, "b"),
		workspace.Key(a, []string{"a", "b"}, ""),
		workspace.Key(a, nil, "a\x01", "b"),
		workspace.Key(a, nil, "a", "\x01b"),
		workspace.Key(workspace.OpTest, nil),
	}
	seen := map[string]int{}
	for i, k := range keys {
		if j, dup := seen[k]; dup {
			t.Errorf("keys %d and %d are equal: %q", j, i, k)
		}
		seen[k] = i
	}
}

// API.md S8, S11: a call whose ctx is done returns ctx.Err() at once while another keeps
// waiting; when the last one leaves, the computation's own context is cancelled.
func TestShareCancel(t *testing.T) {
	p := open(t, newMemFS(lawFiles()))
	s := read(t, p)
	started, stopped := make(chan struct{}), make(chan struct{})
	release := make(chan struct{})
	fn := func(ctx context.Context) (int, error) {
		close(started)
		select {
		case <-release:
			return 1, nil
		case <-ctx.Done():
			close(stopped)
			return 0, ctx.Err()
		}
	}
	key := workspace.Key(workspace.OpPackages, nil)
	first, cancelFirst := context.WithCancel(context.Background())
	firstDone := make(chan error)
	go func() { _, err := workspace.Share(first, s, key, fn); firstDone <- err }()
	<-started
	second, cancelSecond := context.WithCancel(context.Background())
	secondDone := make(chan error)
	go func() { _, err := workspace.Share(second, s, key, fn); secondDone <- err }()
	time.Sleep(10 * time.Millisecond)
	cancelFirst()
	if err := waitErr(t, firstDone); !errors.Is(err, context.Canceled) {
		t.Errorf("the cancelled call: %v", err)
	}
	select {
	case <-stopped:
		t.Fatal("the computation stopped while a call still waited")
	case <-time.After(10 * time.Millisecond):
	}
	cancelSecond()
	if err := waitErr(t, secondDone); !errors.Is(err, context.Canceled) {
		t.Errorf("the second call: %v", err)
	}
	select {
	case <-stopped:
	case <-time.After(prompt):
		t.Error("the computation ran on after its last call left")
	}
	if _, err := workspace.Share(first, s, key, fn); !errors.Is(err, context.Canceled) {
		t.Errorf("a call with a done ctx: %v", err)
	}
}

func waitErr(t *testing.T, done chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(prompt):
		t.Fatal("the call did not return promptly (S11)")
		return nil
	}
}

// API.md X2: a panic in a shared computation is a *safego.PanicError; the snapshot stays usable.
func TestSharePanic(t *testing.T) {
	p := open(t, newMemFS(lawFiles()))
	s := read(t, p)
	_, err := workspace.Share(context.Background(), s, "k", func(context.Context) (int, error) { panic("bug") })
	var pe *safego.PanicError
	if !errors.As(err, &pe) || pe.Value != "bug" {
		t.Errorf("panic: %v", err)
	}
	if v, err := workspace.Share(context.Background(), s, "k", func(context.Context) (int, error) { return 2, nil }); v != 2 || err != nil {
		t.Errorf("after the panic: %v, %v", v, err)
	}
}

// API.md S9, S10: one writer at a time; readers meanwhile get the snapshot they had, without
// the writer's bytes and without waiting; the writer's snapshot is published when it returns.
func TestWriterPublishes(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	var causes []workspace.Cause
	defer p.Subscribe(func(e workspace.Event) { causes = append(causes, e.Cause) })()
	before := read(t, p)
	inside, release := make(chan struct{}), make(chan struct{})
	done := make(chan *workspace.Snapshot)
	go func() {
		next, err := p.Write(context.Background(), workspace.CauseEdit, func(_ context.Context, s *workspace.Snapshot) error {
			if err := s.Build().FS().(interface {
				WriteFile(string, []byte) error
			}).WriteFile("/law/b/b.canon", []byte(srcB+"\n")); err != nil {
				return err
			}
			close(inside)
			<-release
			return nil
		})
		if err != nil {
			t.Error(err)
		}
		done <- next
	}()
	<-inside
	during := read(t, p)
	data, _ := during.Build().FS().ReadFile("/law/b/b.canon")
	if during != before || string(data) != srcB {
		t.Errorf("a reader during a write saw %q (same snapshot %v)", data, during == before)
	}
	short, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := p.Write(short, workspace.CauseEdit, func(context.Context, *workspace.Snapshot) error { return nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a second writer: %v", err)
	}
	close(release)
	next := <-done
	if next == before || read(t, p) != next || len(causes) != 1 || causes[0] != workspace.CauseEdit {
		t.Errorf("the writer's snapshot was not published: %v", causes)
	}
	if data, _ := next.Build().FS().ReadFile("/law/b/b.canon"); string(data) != srcB+"\n" {
		t.Errorf("the published snapshot reads %q", data)
	}
}

// writeFS is a snapshot's file system as a writer uses it.
type writeFS interface {
	ReadFile(string) ([]byte, error)
	ReadDir(string) ([]fs.DirEntry, error)
	Stat(string) (fs.FileInfo, error)
	WriteFile(string, []byte) error
	Rename(string, string) error
	MkdirAll(string) error
}

// writeLikeBuild writes a new source through a temporary file, and an output in a new directory.
func writeLikeBuild(fsys writeFS) error {
	return errors.Join(
		fsys.WriteFile("/law/b/.new.canon.canon-tmp", []byte("/// New.\npackage b\n")),
		fsys.Rename("/law/b/.new.canon.canon-tmp", "/law/b/new.canon"),
		fsys.WriteFile("/law/b/b.canon", []byte(srcB+"\n")),
		fsys.MkdirAll("/law/out/x"),
		fsys.WriteFile("/law/out/x/o.json", []byte("{}\n")),
	)
}

// listed is the names of dir in fsys, or the error.
func listed(fsys writeFS, dir string) ([]string, error) {
	entries, err := fsys.ReadDir(dir)
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out, err
}

// API.md S9: a reader's first listing or read after a write, on a snapshot taken before it,
// shows no name and no byte the write made, not even its temporary file.
func TestWriteInvisibleToOlderSnapshot(t *testing.T) {
	p := open(t, newMemFS(lawFiles()))
	before := read(t, p)
	if _, err := p.Write(context.Background(), workspace.CauseEdit, func(_ context.Context, s *workspace.Snapshot) error {
		return writeLikeBuild(s.Build().FS().(writeFS))
	}); err != nil {
		t.Fatal(err)
	}
	old := before.Build().FS().(writeFS)
	names, err := listed(old, "/law/b")
	if err != nil || !slices.Equal(names, []string{"b.canon"}) {
		t.Errorf("the old snapshot lists %v, %v", names, err)
	}
	for _, name := range names {
		if data, err := old.ReadFile("/law/b/" + name); err != nil || string(data) != srcB {
			t.Errorf("%s reads %q, %v", name, data, err)
		}
	}
	if _, err := old.Stat("/law/out/x"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the old snapshot sees a directory made after it: %v", err)
	}
	if names, err := listed(old, "/law"); err != nil || slices.Contains(names, "out") {
		t.Errorf("the old snapshot lists %v, %v", names, err)
	}
	if names, _ := listed(read(t, p).Build().FS().(writeFS), "/law/b"); !slices.Contains(names, "new.canon") {
		t.Errorf("the new snapshot lists %v", names)
	}
}

// API.md S9: a writer that first refreshes to a new snapshot, after an external change, still
// hides its writes from a reader holding the snapshot before that change.
func TestWriteInvisibleAcrossRefresh(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	a := read(t, p)
	old := a.Build().FS().(writeFS)
	if _, err := old.ReadFile("/law/data/c.json"); err != nil { // in a's read set, b/b.canon is not
		t.Fatal(err)
	}
	_ = fsys.WriteFile("/law/data/c.json", []byte("[8]\n"))
	var during *workspace.Snapshot
	if _, err := p.Write(context.Background(), workspace.CauseEdit, func(_ context.Context, s *workspace.Snapshot) error {
		during = s
		return writeLikeBuild(s.Build().FS().(writeFS))
	}); err != nil {
		t.Fatal(err)
	}
	if during == a {
		t.Fatal("the writer did not refresh past the external change")
	}
	if data, err := old.ReadFile("/law/b/b.canon"); err != nil || string(data) != srcB {
		t.Errorf("the reader of the older snapshot reads %q, %v", data, err)
	}
	if names, err := listed(old, "/law/b"); err != nil || !slices.Equal(names, []string{"b.canon"}) {
		t.Errorf("the reader of the older snapshot lists %v, %v", names, err)
	}
}

// API.md S10: the event of a write names the files it changed, never a directory it made.
func TestWriteEventNamesFiles(t *testing.T) {
	p := open(t, newMemFS(lawFiles()))
	var files []string
	defer p.Subscribe(func(e workspace.Event) { files = append(files, e.Files...) })()
	if _, err := p.Write(context.Background(), workspace.CauseEdit, func(_ context.Context, s *workspace.Snapshot) error {
		fsys := s.Build().FS().(writeFS)
		return errors.Join(fsys.MkdirAll("/law/out/x"), fsys.WriteFile("/law/out/x/o.json", []byte("{}\n")))
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(files, []string{"out/x/o.json"}) {
		t.Errorf("the write's event names %v, want out/x/o.json", files)
	}
}

// API.md S9, X2: a writer that panics still ends its write: the lock is free and readers
// refresh again.
func TestWriterPanic(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	func() {
		defer func() { _ = recover() }()
		_, _ = p.Write(context.Background(), workspace.CauseEdit, func(context.Context, *workspace.Snapshot) error { panic("bug") })
	}()
	s := read(t, p)
	revision(t, s) // reads b/b.canon, so a change to it is a change of the read set (S1)
	_ = fsys.WriteFile("/law/b/b.canon", []byte(srcB+"\n"))
	if read(t, p) == s {
		t.Error("no refresh after a writer panicked")
	}
	if _, err := p.Write(context.Background(), workspace.CauseEdit, func(context.Context, *workspace.Snapshot) error { return nil }); err != nil {
		t.Errorf("the lock after a panic: %v", err)
	}
}

// API.md O6, S11: after Close every call is ErrClosed, a computation running is stopped, and
// Close is idempotent; a done ctx is returned before anything.
func TestClose(t *testing.T) {
	p := open(t, newMemFS(lawFiles()))
	s := read(t, p)
	started := make(chan struct{})
	done := make(chan error)
	go func() {
		_, err := workspace.Share(context.Background(), s, "k", func(ctx context.Context) (int, error) {
			close(started)
			<-ctx.Done()
			return 0, ctx.Err()
		})
		done <- err
	}()
	<-started
	p.Close()
	p.Close()
	if err := waitErr(t, done); !errors.Is(err, workspace.ErrClosed) {
		t.Errorf("a computation running at Close: %v", err)
	}
	ctx := context.Background()
	_, rerr := p.Read(ctx)
	_, werr := p.Write(ctx, workspace.CauseEdit, func(context.Context, *workspace.Snapshot) error { return nil })
	for _, err := range []error{rerr, werr, p.SetOverlay("a/a.canon", nil), p.ClearOverlay("a/a.canon")} {
		if !errors.Is(err, workspace.ErrClosed) {
			t.Errorf("after Close: %v", err)
		}
	}
	p.Subscribe(func(workspace.Event) { t.Error("an event after Close") })()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := p.Read(cancelled); !errors.Is(err, context.Canceled) {
		t.Errorf("a done ctx: %v", err)
	}
}

// API.md S1, S7: many readers refreshing at once see a change once, as one new snapshot.
func TestConcurrentRefresh(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	var events atomic.Int32
	defer p.Subscribe(func(e workspace.Event) {
		if e.Cause == workspace.CauseExternal && len(e.Files) == 1 && e.Files[0] == "a/a.json" {
			events.Add(1)
		}
	})()
	s := read(t, p)
	analyze(t, s)
	_ = fsys.WriteFile("/law/a/a.json", []byte("[5]\n"))
	var wg sync.WaitGroup
	snaps := make(chan *workspace.Snapshot, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snaps <- read(t, p)
		}()
	}
	wg.Wait()
	close(snaps)
	for got := range snaps {
		if got == s {
			t.Error("a reader after the change got the old snapshot")
		}
	}
	if events.Load() != 1 {
		t.Errorf("%d external events naming a/a.json for one change, want 1", events.Load())
	}
}
