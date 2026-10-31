//go:build windows

package load_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// mklinkJunction makes link a junction to target with `mklink /J`, skipping t where that fails.
func mklinkJunction(t *testing.T, link, target string) {
	t.Helper()
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("mklink /J: %v: %s", err, out)
	}
}

// WIRE.md §6.5, DECISIONS 419: TestLoadDirJunction on real junctions.
func TestLoadDirMklink(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root, map[string]string{
		"proj/data/a.json":  `{"id": "a", "name": "A"}`,
		"proj/real/b.json":  `{"id": "b", "name": "B"}`,
		"outside/o/x.json":  `{"id": "x", "name": "X"}`,
		"proj/ghost/x.keep": "",
	}, nil)
	at := func(name string) string { return filepath.Join(root, filepath.FromSlash(name)) }
	mklinkJunction(t, at("proj/data/good"), at("proj/real"))
	mklinkJunction(t, at("proj/data/bad"), at("outside/o"))
	mklinkJunction(t, at("proj/data/gone"), at("proj/ghost"))
	if err := os.RemoveAll(at("proj/ghost")); err != nil {
		t.Fatal(err)
	}
	run := runDirMessages(t, project.OS(), layoutAt(t, filepath.ToSlash(at("proj"))), allJSON)
	if want := []string{"a", "b"}; !slices.Equal(run.ids, want) {
		t.Errorf("entries = %v, want %v", run.ids, want)
	}
	var span source.Span
	want := []string{
		messageOf(diag.W7115.AtOutsideRoots(span, "data/bad")),
		messageOf(diag.W7115.AtDangling(span, "data/gone")),
	}
	if !slices.Equal(run.msgs, want) {
		t.Errorf("findings = %q, want %q", run.msgs, want)
	}
}
