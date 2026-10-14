package workspace_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
	"github.com/fantasim/canonlang/internal/writelock"
)

const journalRule = "DECISIONS 375, API.md N11: "

// A stalled commit is this test binary run with stallRun, flagsEnd, stallRole, then a project
// directory, a path and a count: it plans the Set, takes its packages' locks as an edit does,
// commits, says stalledLine after the first file it renames, and waits until its stdin closes.
const (
	stallRun    = "-test.run=^TestStallHelperProcess$"
	stallRole   = "stall-helper"
	stalledLine = "stalled"
)

// cutRev is the revision a commit made by hand names its journal by.
var cutRev = "sha256:" + strings.Repeat("c", 64)

// errCut is the panic a commit cut short dies of.
var errCut = errors.New("cut short")

// TestStallHelperProcess is the stalled commit; run any other way it does nothing.
func TestStallHelperProcess(t *testing.T) {
	i := slices.Index(os.Args, stallRole)
	if i < 0 || len(os.Args)-i-1 < helperArgN {
		t.Skip("a helper process only")
	}
	os.Exit(helperStall(os.Args[i+1], os.Args[i+2], os.Args[i+3]))
}

// helperStall commits the Set of path to n at dir, stalling after its first rename; 1 if it failed.
func helperStall(dir, path, n string) int {
	count, err := strconv.Atoi(n)
	if err != nil {
		return 1
	}
	b, err := build.Open(build.OS(), dir, build.Options{})
	if err != nil {
		fmt.Println(err)
		return 1
	}
	p := workspace.New(b)
	defer p.Close()
	ctx := context.Background()
	req := setOps(count, path)
	req.DryRun = true
	out, err := p.Edit(ctx, req)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	stall := func() {
		fmt.Println(stalledLine)
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(1)
	}
	if err := cutCommit(b, out.Plan.Touched, out.Changes, stall); err != nil {
		fmt.Println(err)
	}
	return 1
}

// cutFS is the OS's files, calling at after the after-th rename outside the journal directory.
type cutFS struct {
	build.WriteFS
	journal string
	after   int
	renames int
	at      func()
}

func (f *cutFS) Rename(oldname, newname string) error {
	if err := f.WriteFS.Rename(oldname, newname); err != nil {
		return err
	}
	if !strings.HasPrefix(newname, f.journal) {
		if f.renames++; f.renames == f.after {
			f.at()
		}
	}
	return nil
}

// cutOS is the OS's files of the project at dir calling at after their after-th rename.
func cutOS(dir string, after int, at func()) *cutFS {
	return &cutFS{WriteFS: build.OS(), journal: dir + "/" + edit.JournalDir, after: after, at: at}
}

// anyLocker is a Locker that never takes a lock: a commit's journal records what it holds.
type anyLocker struct{}

func (anyLocker) TryLock([]string) (func() error, bool, error) { return nil, false, nil }

// hostName is this host as a journal records it.
func hostName() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return host
}

// cutCommit commits changes on b's project holding pkgs exclusive, as an edit does, through the
// OS's files calling at after the first rename.
func cutCommit(b *build.Project, pkgs []string, changes []edit.Change, at func()) error {
	held, err := writelock.Acquire(context.Background(), filepath.FromSlash(b.Dir()), writelock.ExclusiveOf(pkgs...))
	if err != nil {
		return err
	}
	defer func() { _ = held.Release() }()
	site := edit.Site{FS: cutOS(b.Dir(), 1, at), Layout: b, Self: edit.Process{Host: hostName(), PID: os.Getpid()}, Locks: anyLocker{}, Held: pkgs}
	return edit.Commit(context.Background(), site, cutRev, changes)
}

// tornCommit is an edit of dir holding pkgs that dies after its after-th rename: its journal
// records pkgs, whose locks are free again when it returns.
func tornCommit(t *testing.T, dir string, pkgs []string, after int, changes []edit.Change) {
	t.Helper()
	b := openBuild(t, dir)
	held := holdWriters(t, dir, pkgs...)
	site := edit.Site{FS: cutOS(dir, after, func() { panic(errCut) }), Layout: b, Self: edit.Process{Host: hostName(), PID: os.Getpid()}, Locks: anyLocker{}, Held: pkgs}
	ended := func() (r any) {
		defer func() { r = recover() }()
		return edit.Commit(context.Background(), site, cutRev, changes)
	}()
	if ended != errCut {
		t.Fatalf("the commit was not cut: %v", ended)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
}

// openBuild is the build of the project at dir on the OS's files.
func openBuild(t *testing.T, dir string) *build.Project {
	t.Helper()
	b, err := build.Open(build.OS(), dir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// rowChange modifies rel of dir, its default 0 for x becoming n.
func rowChange(t *testing.T, dir, rel string, n int) edit.Change {
	t.Helper()
	before := fileText(t, dir, rel)
	after := strings.Replace(before, "x: Int = 0", "x: Int = "+strconv.Itoa(n), 1)
	return edit.Change{Kind: edit.ChangeModified, Path: rel, Before: []byte(before), After: []byte(after)}
}

// journalsOf are the journal files of the project at dir.
func journalsOf(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(edit.JournalDir)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			out = append(out, e.Name())
		}
	}
	return out
}

// stalled is a helper process stalled mid-commit.
type stalled struct {
	cmd  *exec.Cmd
	once sync.Once
}

// startStall runs the stalled commit of path to n at dir and returns once it stalls.
func startStall(t *testing.T, dir, path, n string) *stalled {
	t.Helper()
	cmd := exec.Command(os.Args[0], stallRun, flagsEnd, stallRole, dir, path, n)
	if _, err := cmd.StdinPipe(); err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	h := &stalled{cmd: cmd}
	t.Cleanup(h.kill)
	lines := make(chan string, 1)
	go func() {
		s := bufio.NewScanner(out)
		s.Scan()
		lines <- s.Text()
		_, _ = io.Copy(io.Discard, out)
	}()
	select {
	case got := <-lines:
		if got != stalledLine {
			t.Fatalf("the stalled commit said %q", got)
		}
	case <-time.After(patience):
		t.Fatal("the helper never stalled")
	}
	return h
}

// kill ends the helper and waits for it, its locks freed with it.
func (h *stalled) kill() {
	h.once.Do(func() {
		_ = h.cmd.Process.Kill()
		_ = h.cmd.Wait()
	})
}
