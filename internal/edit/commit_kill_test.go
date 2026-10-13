package edit_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// A helper commit is this test binary run with killRun, then killSep and a project directory: it
// commits killChanges there through the OS's files and stops for good after its first rename.
const (
	killRun     = "-test.run=^TestCommitHelperProcess$"
	killSep     = "commit-helper"
	flagsEnd    = "--" // ends the test binary's flags
	renamedMark = "renamed"
	killWait    = 30 * time.Second
	exitHelper  = 2
)

// killFiles is the project the helper commits in.
var killFiles = map[string]string{"a.canon": "a old\n", "b.canon": "b old\n"}

// killChanges modifies a and b and creates items/new.canon, in a directory of its own.
func killChanges() []edit.Change {
	return []edit.Change{
		{Kind: edit.ChangeModified, Path: "a.canon", Before: []byte("a old\n"), After: []byte("a new\n")},
		{Kind: edit.ChangeModified, Path: "b.canon", Before: []byte("b old\n"), After: []byte("b new\n")},
		{Kind: edit.ChangeCreated, Path: "items/new.canon", After: []byte("new\n")},
	}
}

// stallFS is the OS's files, which, once its first rename is made, says so and never returns.
type stallFS struct{ build.WriteFS }

func (s stallFS) Rename(oldname, newname string) error {
	if err := s.WriteFS.Rename(oldname, newname); err != nil {
		return err
	}
	fmt.Println(renamedMark)
	_, _ = io.Copy(io.Discard, os.Stdin) // until the test kills this process
	return nil
}

// TestCommitHelperProcess is the helper commit; run any other way it does nothing.
func TestCommitHelperProcess(t *testing.T) {
	i := slices.Index(os.Args, killSep)
	if i < 0 || i+1 >= len(os.Args) {
		t.Skip("a helper process only")
	}
	dir := os.Args[i+1]
	site := edit.Site{FS: stallFS{build.OS()}, Layout: layout{dir: dir}, Self: edit.Process{Host: testHost, PID: os.Getpid()}}
	fmt.Fprintln(os.Stderr, edit.Commit(context.Background(), site, testRev, killChanges()))
	os.Exit(exitHelper)
}

// DECISIONS 351, API.md N11, O5: after a commit killed past its first rename, Recover (<j> from the
// journal's name) restores every file and clears its stages, 0.1.5's fixed one, and the WriteFile
// leftovers of its stages and journal, nothing else.
func TestCommitKilledMidway(t *testing.T) {
	dir := diskProject(t, killFiles)
	before := diskRead(t, dir)
	killedCommit(t, dir)
	left := diskRead(t, dir)
	stage := "." + "b.canon" + testStage
	if _, ok := left[stage]; !ok {
		t.Fatalf("the killed commit left %v, no %s", slices.Sorted(maps.Keys(left)), stage)
	}
	plantOS(t, dir, map[string]string{
		".a.canon.canon-edit":                                "a 0.1.5 stage\n",
		"." + stage + leftover:                               "half a stage", // a WriteFile of a stage killed
		".canon/journal/." + journalHex + ".json" + leftover: "half a journal",
	})
	kept := map[string]string{ // like the edit's names, never its own: Recover leaves them
		"..b.canon.cdcdcdcdcdcdcdcd.canon-edit" + leftover: "another edit's",
		".b.canon.notes": "the user's",
		".canon/journal/." + strings.Repeat("cd", 32) + ".json" + leftover: "another journal's",
	}
	plantOS(t, dir, kept)
	site := edit.Site{FS: build.OS(), Layout: layout{dir: dir}, Self: edit.Process{Host: testHost, PID: 1, Alive: func(int) bool { return false }}}
	if err := edit.Recover(site, nil); err != nil {
		t.Fatal(err)
	}
	want := maps.Clone(before)
	maps.Copy(want, kept)
	if after := diskRead(t, dir); !maps.Equal(after, want) {
		t.Errorf("after Recover %v, want %v", slices.Sorted(maps.Keys(after)), slices.Sorted(maps.Keys(want)))
	}
}

// leftover is what the OS WriteFile leaves of a write killed midway after the target's name.
const leftover = ".ABCDEFGHIJKLM.canon-tmp"

// journalHex is the journal name's hex of a commit at testRev.
var journalHex = strings.Repeat("ab", 32)

// plantOS writes files, by slash path relative to dir.
func plantOS(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files { //canon:unordered each file is written alone
		p := filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), dirPerm); err != nil || os.WriteFile(p, []byte(text), filePerm) != nil {
			t.Fatal(name, err)
		}
	}
}

// killedCommit runs the helper commit in dir until its first rename, then kills it.
func killedCommit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], killRun, flagsEnd, killSep, dir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line := make(chan string, 1)
	go func() {
		s := bufio.NewScanner(out)
		for s.Scan() {
			if s.Text() == renamedMark {
				line <- s.Text()
			}
		}
		close(line)
	}()
	select {
	case got, ok := <-line:
		if !ok || got != renamedMark {
			_ = cmd.Process.Kill()
			t.Fatalf("the helper commit ended before its first rename: %s", stderr.String())
		}
	case <-time.After(killWait):
		_ = cmd.Process.Kill()
		t.Fatal("the helper commit made no rename")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait() // killed: its exit status is the signal's
}
