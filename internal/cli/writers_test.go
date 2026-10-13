package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
	"github.com/fantasim/canonlang/internal/writelock"
)

// The 2026-10-12 repro through real processes: rounds of a package with an emit text into a new
// directory, its source changed, and two `canon build` processes started at once.
const (
	writersRule   = "DECISIONS 351, 352: "
	canonRun      = "-test.run=^TestCanonProcess$"
	argSep        = "--"
	processRounds = 5
	processes     = 2
	racedProject  = "project acme {\n  canon: \"0.1\"\n  roots {\n    out: \"out\"\n  }\n}\n"
)

// racedSource is package a writing t.sql holding content into @out/<dir>, and one JSON value.
func racedSource(dir, content string) string {
	return "/// P.\npackage a\n\n/// V.\nlet v: Int = 1\n\n/// F.\n@text(\"t.sql\")\nexport fn f() -> String { return \"" + content + "\" }\n\n" +
		"emit text { out: \"@out/" + dir + "\" }\n\nemit json { out: \"@out/" + dir + "/json/\" }\n"
}

// TestCanonProcess is canon itself in a process of its own, its directory and arguments after
// argSep; run any other way it does nothing.
func TestCanonProcess(t *testing.T) {
	i := slices.Index(os.Args, argSep)
	if i < 0 || len(os.Args) < i+3 {
		t.Skip("a canon process only")
	}
	dir := os.Args[i+1]
	os.Exit(cli.Main(context.Background(), os.Args[i+2:], cli.Env{Stdout: os.Stdout, Stderr: os.Stderr, Dir: dir}))
}

// DECISIONS 351, 352: two canon build processes of one package at once, each round its source
// changed and its outputs moved to a new directory, both succeed and leave every output complete
// and correct, nothing staged; before them 80 of 80 such builds lost both outputs.
func TestConcurrentBuildProcesses(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "project.canon"), racedProject)
	for round := range processRounds {
		out, content := fmt.Sprintf("sql%d", round), fmt.Sprintf("v%d", round)
		write(t, filepath.Join(dir, "a", "a.canon"), racedSource(out, content))
		cmds := make([]*exec.Cmd, processes)
		outs := make([]bytes.Buffer, processes)
		for i := range cmds {
			cmds[i] = exec.Command(os.Args[0], canonRun, argSep, dir, "build", "--project", dir)
			cmds[i].Stdout, cmds[i].Stderr = &outs[i], &outs[i]
			if err := cmds[i].Start(); err != nil {
				t.Fatal(err)
			}
		}
		for i, cmd := range cmds {
			if err := cmd.Wait(); err != nil {
				t.Fatalf("%sround %d, build %d: %v\n%s", writersRule, round, i, err, outs[i].String())
			}
		}
		got, err := os.ReadFile(filepath.Join(dir, "out", out, "t.sql"))
		_, jerr := os.Stat(filepath.Join(dir, "out", out, "json", "v.json"))
		if err != nil || string(got) != content || jerr != nil {
			t.Fatalf("%sround %d: t.sql %q (%v), v.json %v", writersRule, round, got, err, jerr)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "out", "*", ".*.canon-tmp")); len(left) > 0 {
		t.Errorf("%sstaged files left: %v", writersRule, left)
	}
}

// waitHook is a standard error that, at each note of a wait for package p's lock, rewrites a/a.canon
// to import z, a new value each time, and frees p's lock: another writer done meanwhile.
type waitHook struct {
	buf   bytes.Buffer // a field, not embedded: io.WriteString would pass Write by
	dir   string
	held  map[string]*writelock.Held
	waits int
	err   error
}

func (w *waitHook) Write(p []byte) (int, error) {
	for pkg, h := range w.held { //canon:unordered a note names one package
		if strings.Contains(string(p), "package "+pkg+",") {
			w.waits++
			src := fmt.Sprintf("/// A.\npackage a\n\nimport z { Z }\n\n/// V.\nlet v: Z = { n: %d }\n\nemit json { out: \"@out/\" }\n", w.waits)
			w.err = errors.Join(w.err, os.WriteFile(filepath.Join(w.dir, "a", "a.canon"), []byte(src), 0o600), h.Release())
		}
	}
	return w.buf.Write(p)
}

func (w *waitHook) String() string { return w.buf.String() }

// DECISIONS 352, API.md §15: sources changed under the locks on the run and its redo exit 1.
func TestBuildStaleExitsOne(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "project.canon"), racedProject)
	write(t, filepath.Join(dir, "a", "a.canon"), racedSource("sql", "v0"))
	write(t, filepath.Join(dir, "z", "z.canon"), "/// Z.\npackage z\n\n/// A z.\nrecord Z {\n  /// N.\n  n: Int\n}\n")
	stderr := &waitHook{dir: dir, held: map[string]*writelock.Held{"a": holdLock(t, dir, "a"), "z": holdLock(t, dir, "z")}}
	code := cli.Main(context.Background(), []string{"build", "a", "--project", dir}, cli.Env{Stdout: &bytes.Buffer{}, Stderr: stderr, Dir: dir})
	if code != 1 || stderr.waits != 2 || stderr.err != nil || !strings.Contains(stderr.String(), "canon: sources changed since the base revision: a/a.canon") {
		t.Errorf("%sexit %d, %d waits (%v), stderr %q", writersRule, code, stderr.waits, stderr.err, stderr.String())
	}
}

// noteBuffer is a standard error a test reads while the command writes it, which says on a
// channel when its first line is written.
type noteBuffer struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	first chan struct{}
	once  sync.Once
}

func newNoteBuffer() *noteBuffer { return &noteBuffer{first: make(chan struct{})} }

func (b *noteBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	defer b.once.Do(func() { close(b.first) })
	return b.buf.Write(p)
}

func (b *noteBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// holdLock holds package pkg's writer lock of the project at dir, as another writer would.
func holdLock(t *testing.T, dir, pkg string) *writelock.Held {
	t.Helper()
	held, err := writelock.Acquire(context.Background(), dir, []writelock.Request{{Package: pkg, Mode: writelock.Exclusive}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Release() })
	return held
}

// runAsync runs a command line in the background; its exit code comes on the channel.
func runAsync(dir string, stderr *noteBuffer, args ...string) (chan int, *bytes.Buffer) {
	var stdout bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- cli.Main(context.Background(), args, cli.Env{Stdout: &stdout, Stderr: stderr, Dir: dir})
	}()
	return done, &stdout
}

// DECISIONS 352, CLI.md §3.4: a build waiting for another writer says once whom it waits for, then builds.
func TestBuildWaitsAndSaysWhom(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "project.canon"), racedProject)
	write(t, filepath.Join(dir, "a", "a.canon"), racedSource("sql", "v"))
	held := holdLock(t, dir, "a")
	stderr := newNoteBuffer()
	done, _ := runAsync(dir, stderr, "build", "--project", dir)
	<-stderr.first
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("canon: waiting for the writer lock of package a, held by %s %d\n", host, os.Getpid())
	if code := <-done; code != 0 || stderr.String() != want {
		t.Errorf("%sexit %d, stderr %q, want %q", writersRule, code, stderr.String(), want)
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "sql", "t.sql")); err != nil {
		t.Errorf("%safter the wait: %v", writersRule, err)
	}
}

// DECISIONS 352, CLI.md §3.6: canon fmt waits for its package's lock and refuses a file changed meanwhile.
func TestFmtWaitsAndRefusesAChangedFile(t *testing.T) {
	dir := t.TempDir()
	const messy, edited = "package   a\n", "package a\n\n/// V.\nlet v: Int = 2\n"
	write(t, filepath.Join(dir, "project.canon"), racedProject)
	write(t, filepath.Join(dir, "a", "a.canon"), messy)
	held := holdLock(t, dir, "a")
	stderr := newNoteBuffer()
	done, _ := runAsync(dir, stderr, "fmt", "--project", dir)
	<-stderr.first
	write(t, filepath.Join(dir, "a", "a.canon"), edited)
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	code := <-done
	got, err := os.ReadFile(filepath.Join(dir, "a", "a.canon"))
	if code != 2 || !strings.Contains(stderr.String(), "a/a.canon: changed while canon fmt ran") || err != nil || string(got) != edited {
		t.Errorf("%sexit %d, stderr %q, file %q (%v)", writersRule, code, stderr.String(), got, err)
	}
	stderr = newNoteBuffer()
	if code := cli.Main(context.Background(), []string{"fmt", "--project", dir}, cli.Env{Stdout: &bytes.Buffer{}, Stderr: stderr, Dir: dir}); code != 0 || stderr.String() != "" {
		t.Errorf("%sfmt again: exit %d, stderr %q", writersRule, code, stderr.String())
	}
}
