package workspace

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/fantasim/canonlang/internal/build"
)

var (
	errNoNotify = errors.New("no notifications here")
	errNoWatch  = errors.New("no more watches")
)

// fakeNotifier stands for the OS's notifications: it watches anything until it fails.
type fakeNotifier struct {
	failing atomic.Bool
	ev      chan fsnotify.Event
	er      chan error
}

func (f *fakeNotifier) add(string) error {
	if f.failing.Load() {
		return errNoWatch
	}
	return nil
}

func (f *fakeNotifier) remove(string) error           { return nil }
func (f *fakeNotifier) close() error                  { return nil }
func (f *fakeNotifier) events() <-chan fsnotify.Event { return f.ev }
func (f *fakeNotifier) errs() <-chan error            { return f.er }

// logLines is a log's text, safe for the hub's goroutine and the test's.
type logLines struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logLines) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logLines) warnings() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Count(l.buf.String(), "level=WARN")
}

// API.md S1, W12 (log-2026-09-29 M4 U5-r G1): when the OS's notifications cannot start, or a
// directory cannot be watched at the start or at a later resync, the watcher says so once and
// polls instead, and changes keep coming.
func TestWatchFallsBackToPolling(t *testing.T) {
	late := &fakeNotifier{}
	cases := []struct {
		name   string
		notify func() (notifier, error)
		fail   func()
	}{
		{"constructor", func() (notifier, error) { return nil, errNoNotify }, func() {}},
		{"add at start", func() (notifier, error) { f := &fakeNotifier{}; f.failing.Store(true); return f, nil }, func() {}},
		{"add at a resync", func() (notifier, error) { return late, nil }, func() { late.failing.Store(true) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, p := osLaw(t)
			var log logLines
			changes := make(chan Change, 16)
			ctx, cancel := context.WithCancel(context.Background())
			w, err := p.Watch(ctx, func(ctx context.Context, c Change) {
				_, _ = c.Snapshot.Revision(ctx) // a re-check reads a new directory: a resync follows it
				changes <- c
			}, WatchOptions{OS: true, Poll: time.Millisecond, Logger: slog.New(slog.NewTextHandler(&log, nil)), notify: tc.notify})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { cancel(); <-w.Done() }()
			tc.fail()
			writeFile(t, filepath.Join(dir, "e", "e.canon"), "/// E.\npackage e\n")
			waitChange(t, changes, "e/e.canon", false)
			writeFile(t, filepath.Join(dir, "e", "e.canon"), "/// E.\npackage e\n\n/// N.\nconst N = 1\n")
			waitChange(t, changes, "e/e.canon", true)
			if n := log.warnings(); n != 1 {
				t.Errorf("%d warnings, want 1:\n%s", n, log.buf.String())
			}
		})
	}
}

// osLaw is a project of one package on the OS's files, read and revised.
func osLaw(t *testing.T) (string, *Project) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "project.canon"), "project acme {\n  canon: \"0.1\"\n}\n")
	writeFile(t, filepath.Join(dir, "a", "a.canon"), "/// A.\npackage a\n")
	b, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(b)
	t.Cleanup(p.Close)
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Revision(context.Background()); err != nil {
		t.Fatal(err)
	}
	return dir, p
}

func writeFile(t *testing.T, name, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// waitChange waits for a change naming file among its files, or with named false its names.
func waitChange(t *testing.T, changes <-chan Change, file string, named bool) {
	t.Helper()
	deadline := time.After(20 * time.Second)
	for {
		select {
		case c := <-changes:
			if slices.ContainsFunc(c.Files, func(f Path) bool { return f.Display == file }) ||
				!named && slices.ContainsFunc(c.Changed, func(n string) bool { return strings.HasSuffix(n, "/e") }) {
				return
			}
		case <-deadline:
			t.Fatalf("no change naming %s", file)
		}
	}
}
