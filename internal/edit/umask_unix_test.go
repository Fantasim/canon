//go:build unix

package edit_test

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

const (
	umaskMask   = 0o022
	umaskGroup  = 0o664
	umaskOthers = 0o644
	gatherFile  = "d/q/collect/gather.canon"
)

// setUmask sets the process umask for the test, restored after it; no test using it is parallel.
func setUmask(t *testing.T, mask int) {
	t.Helper()
	old := syscall.Umask(mask)
	t.Cleanup(func() { syscall.Umask(old) })
}

// modeIn is the permission bits of the file rel in dir.
func modeIn(t *testing.T, dir, rel string) fs.FileMode {
	t.Helper()
	info, err := os.Stat(filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

// handoff 2026-10-07 A2, API.md N10: a commit on disk gives a file it creates, and the directory
// made for it, what a plain create gets under umask 022, and a file it rewrites keeps its mode.
func TestCommitFileModes(t *testing.T) {
	setUmask(t, umaskMask)
	dir := filesProject(t)
	if err := os.Chmod(filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(gatherFile)), umaskGroup); err != nil {
		t.Fatal(err)
	}
	before := slices.Sorted(maps.Keys(diskRead(t, dir)))
	ops := []edit.Operation{
		{Kind: edit.OpAddEntry, Path: "quests", Key: edit.Key("fresh"), Value: edit.Source(`{ goal: kill, target: "n" }`)},
		setAt("quests.gather.goal", edit.Member("kill")),
	}
	if _, ok := commitOnDisk(t, "created and rewritten", dir, ops); !ok {
		return
	}
	if got := modeIn(t, dir, gatherFile); got != umaskGroup {
		t.Errorf("%s: mode %o after a rewrite, want %o", gatherFile, got, umaskGroup)
	}
	created := 0
	for _, name := range slices.Sorted(maps.Keys(diskRead(t, dir))) {
		if slices.Contains(before, name) {
			continue
		}
		created++
		if got := modeIn(t, dir, name); got != umaskOthers {
			t.Errorf("%s: mode %o, want %o", name, got, umaskOthers)
		}
	}
	if created == 0 {
		t.Error("the commit created no file")
	}
}
