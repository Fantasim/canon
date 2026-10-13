package workspace_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
	"github.com/fantasim/canonlang/internal/writelock"
)

// A helper edit is this test binary run with editRun, flagsEnd, editRole, then a project
// directory, a path and a count: it sets the path to the count, says when it waits for a lock,
// then how its edit ended.
const (
	editRun     = "-test.run=^TestEditHelperProcess$"
	flagsEnd    = "--"
	editRole    = "edit-helper"
	waitingLine = "waiting"
	appliedLine = "applied"
	staleLine   = "stale"
	helperArgN  = 3
)

// TestEditHelperProcess is the helper edit; run any other way it does nothing.
func TestEditHelperProcess(t *testing.T) {
	i := slices.Index(os.Args, editRole)
	if i < 0 || len(os.Args)-i-1 < helperArgN {
		t.Skip("a helper process only")
	}
	os.Exit(helperEdit(os.Args[i+1], os.Args[i+2], os.Args[i+3]))
}

// helperEdit sets path to n in the project at dir and prints how it went; 0 unless it failed.
func helperEdit(dir, path, n string) int {
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
	ctx := writelock.WithWaiting(context.Background(), func(writelock.Wait) { fmt.Println(waitingLine) })
	_, err = p.Edit(ctx, setOps(count, path))
	switch {
	case err == nil:
		fmt.Println(appliedLine)
	case errors.Is(err, edit.ErrStale):
		fmt.Println(staleLine)
	default:
		fmt.Println(err)
		return 1
	}
	return 0
}

// DECISIONS 352, API.md N9: an edit in another process and one in this process of one package,
// planned on the same sources, both wait for the lock the test holds; then one commits and N9
// makes the other stale, and the file never loses the one that committed.
func TestEditsAcrossProcesses(t *testing.T) {
	dir, p := onDisk(t, rowFiles)
	held := holdWriters(t, dir, "b")
	cmd := exec.Command(os.Args[0], editRun, flagsEnd, editRole, dir, "b:r.y", "2")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 2)
	go func() {
		for s := bufio.NewScanner(out); s.Scan(); {
			lines <- s.Text()
		}
		close(lines)
	}()
	if got := <-lines; got != waitingLine {
		t.Fatalf("%sthe helper edit said %q before waiting", writersRule, got)
	}
	waits, errs := make(chan writelock.Wait, 1), make(chan error, 1)
	go func() {
		_, err := p.Edit(waitsOn(context.Background(), waits), setOps(1, "b:r.x"))
		errs <- err
	}()
	waited(t, waits)
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	theirs, mine := <-lines, <-errs
	if err := cmd.Wait(); err != nil {
		t.Fatalf("%sthe helper edit: %v", writersRule, err)
	}
	text := fileText(t, dir, "b/b.canon")
	switch {
	case theirs == appliedLine && errors.Is(mine, edit.ErrStale):
		if !strings.Contains(text, "y: 2") || strings.Contains(text, "x: 1") {
			t.Errorf("%sthe other process applied, b.canon:\n%s", writersRule, text)
		}
	case theirs == staleLine && mine == nil:
		if !strings.Contains(text, "x: 1") || strings.Contains(text, "y: 2") {
			t.Errorf("%sthis process applied, b.canon:\n%s", writersRule, text)
		}
	default:
		t.Errorf("%sthe other process %q, this one %v: want one applied, one stale", writersRule, theirs, mine)
	}
}
