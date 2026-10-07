package workspace_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/workspace"
)

// osProject is files written under a temporary directory, the project in its proj/, opened on
// the OS's files, read, analyzed and revised; skipped where the OS cannot notify changes.
func osProject(t *testing.T, files map[string]string) (string, *workspace.Project) {
	t.Helper()
	probe, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("the OS cannot notify changes here: %v", err)
	}
	_ = probe.Close()
	root := t.TempDir()
	for name, text := range files {
		writeOS(t, filepath.Join(root, name), text)
	}
	b, err := build.Open(build.OS(), filepath.ToSlash(filepath.Join(root, "proj")), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := workspace.New(b)
	t.Cleanup(p.Close)
	s := read(t, p)
	analyze(t, s)
	revision(t, s)
	return root, p
}

// osWatch watches p on the OS's notifications by clk; each change's snapshot is revised, as a
// re-check reads it, before it is sent on.
func osWatch(t *testing.T, p *workspace.Project, clk *fakeClock) <-chan workspace.Change {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	changes := make(chan workspace.Change, 64)
	lim := &limitLog{}
	clk.exhausted = lim.err
	w, err := p.Watch(ctx, func(ctx context.Context, c workspace.Change) {
		_, _ = c.Snapshot.Revision(ctx)
		changes <- c
	}, workspace.WatchOptions{Clock: clk, OS: true, Logger: slog.New(lim)})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		stopped(t, w)
	})
	clk.idle(t)
	clk.skipIfExhausted(t)
	clk.step(t, quietForTest) // the directories first watched were a change: nothing changed
	return changes
}

// limitLog is a log handler that keeps the error the hub gave when it fell back to polling for
// want of what the machine limits per user: inotify watches (ENOSPC) or instances (EMFILE).
type limitLog struct {
	mu  sync.Mutex
	got error
}

func (l *limitLog) Enabled(context.Context, slog.Level) bool { return true }
func (l *limitLog) WithAttrs([]slog.Attr) slog.Handler       { return l }
func (l *limitLog) WithGroup(string) slog.Handler            { return l }

func (l *limitLog) Handle(_ context.Context, r slog.Record) error {
	r.Attrs(func(a slog.Attr) bool {
		err, ok := a.Value.Any().(error)
		if ok && (errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EMFILE)) {
			l.mu.Lock()
			l.got = err
			l.mu.Unlock()
		}
		return true
	})
	return nil
}

func (l *limitLog) err() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.got
}

const osProjectCanon = "project acme {\n  canon: \"0.1\"\n}\n"

// notifiedSteps bounds the quiet times a notified write waits: half the second before the
// comparison with the disk would see it instead.
const notifiedSteps = 5

// API.md W12, W14: on the OS's files a change is notified, seen at once rather than at the next
// comparison with the disk; a directory that appears is a change, and is followed once read.
func TestWatchOS(t *testing.T) {
	root, p := osProject(t, map[string]string{"proj/project.canon": osProjectCanon, "proj/a/a.canon": srcA, "proj/a/a.json": "[1, 2]\n"})
	pub := record(t, p)
	clk := newFakeClock()
	changes := osWatch(t, p, clk)
	notified := func(name, text string) { // a write can be notified twice: the second moves the quiet time
		before := len(pub.all())
		writeOS(t, filepath.Join(root, "proj", name), text)
		clk.armedIn(t, quietForTest)
		for i := 0; i < notifiedSteps && len(pub.all()) == before; i++ {
			clk.step(t, quietForTest)
		}
	}
	notified("a/a.json", "[3]\n")
	waitFor(t, changes, func(c workspace.Change) bool { return slices.Contains(shown(c), "a/a.json") })
	notified("e/e.canon", "/// E.\npackage e\n")
	waitFor(t, changes, func(c workspace.Change) bool {
		return slices.Contains(c.Changed, path.Join(filepath.ToSlash(root), "proj/e"))
	})
	clk.armedIn(t, quietForTest) // the re-check read e/: watched from now on
	clk.step(t, quietForTest)
	notified("e/e.canon", "/// E.\npackage e\n\n/// N.\nconst N = 1\n")
	waitFor(t, changes, func(c workspace.Change) bool { return slices.Contains(shown(c), "e/e.canon") })
}

// API.md S1, W12: a change the OS does not notify, to the file a symbolic link leads to, is seen
// by the comparison with the disk the watcher makes every second.
func TestWatchSafetyNet(t *testing.T) {
	root, p := osProject(t, map[string]string{"proj/project.canon": osProjectCanon, "proj/a/a.canon": srcA, "ext/t.json": "[1]\n"})
	if err := os.Symlink(filepath.Join(root, "ext", "t.json"), filepath.Join(root, "proj", "a", "a.json")); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	analyze(t, read(t, p))
	pub := record(t, p)
	clk := newFakeClock()
	changes := osWatch(t, p, clk)
	writeOS(t, filepath.Join(root, "ext", "t.json"), "[1, 2, 3]\n")
	clk.step(t, time.Second-quietForTest)
	clk.step(t, quietForTest)
	if len(pub.all()) == 0 {
		t.Fatal("a change to a link's target unseen after the watcher's interval")
	}
	waitFor(t, changes, func(c workspace.Change) bool { return slices.Contains(shown(c), "a/a.json") })
}

// API.md W12: a directory a read needs that does not exist yet is watched through the one above
// it and followed once it appears, under a root outside the project too.
func TestWatchMissingDirectory(t *testing.T) {
	project := "project acme {\n  canon: \"0.1\"\n  roots {\n    ext: \"../ext\"\n  }\n}\n"
	src := "/// C.\npackage c\n\n/// Ys.\nlet ys: [Int] = load(\"@ext/sub/c.json\")\n"
	root, p := osProject(t, map[string]string{"proj/project.canon": project, "proj/c/c.canon": src, "ext/keep": ""})
	pub := record(t, p)
	clk := newFakeClock()
	changes := osWatch(t, p, clk)
	writeOS(t, filepath.Join(root, "ext", "sub", "c.json"), "[3]\n")
	for elapsed := time.Duration(0); len(pub.all()) == 0 && elapsed <= time.Second+quietForTest; elapsed += quietForTest {
		clk.step(t, quietForTest)
	}
	if len(pub.all()) == 0 {
		t.Fatal("a file in a new directory unseen after the watcher's interval")
	}
	want := path.Join(filepath.ToSlash(root), "ext/sub/c.json")
	waitFor(t, changes, func(c workspace.Change) bool { return slices.Contains(c.Changed, want) })
}

func writeOS(t *testing.T, name, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// waitFor waits for a change that is, skipping the others.
func waitFor(t *testing.T, changes <-chan workspace.Change, is func(workspace.Change) bool) {
	t.Helper()
	deadline := time.After(waitLimit)
	var seen [][]string
	for {
		select {
		case c := <-changes:
			if is(c) {
				return
			}
			seen = append(seen, append(shown(c), c.Changed...))
		case <-deadline:
			t.Fatalf("no such change; saw %v", seen)
		}
	}
}
