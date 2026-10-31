//go:build windows

package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// mklinkJunction makes link a junction to target, skipping the test where mklink cannot.
func mklinkJunction(t *testing.T, target, link string) {
	t.Helper()
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("mklink /J failed: %v: %s", err, out)
	}
}

// WIRE.md §6.5, DECISIONS 419: on a real junction, the OS walk follows it, its listing entry is a link candidate, and it is not Unlinked.
func TestEvalSymlinksMklink(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	write(t, dir, "target/inside.txt", "")
	mklinkJunction(t, filepath.FromSlash(dir+"/target"), filepath.FromSlash(dir+"/junction"))
	want, err := evalSymlinksOS(dir + "/target/inside.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, err := evalSymlinksOS(dir + "/junction/inside.txt")
	if err != nil || got != want {
		t.Errorf("evalSymlinksOS(junction/inside.txt) = %q, %v; want %q, nil", got, err, want)
	}
	entries, err := os.ReadDir(filepath.FromSlash(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "junction" {
			continue
		}
		if !LinkCandidate(OS(), e.Type()) {
			t.Errorf("LinkCandidate(junction, %v) = false", e.Type())
		}
		if Unlinked(OS().(osFS).EvalSymlinks, dir, e.Name()) {
			t.Error("Unlinked(junction) = true")
		}
		return
	}
	t.Error("the listing has no junction")
}
