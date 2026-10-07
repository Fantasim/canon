//go:build unix

package cli_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
)

const (
	umaskMask   = 0o022
	umaskGroup  = 0o664
	umaskOthers = 0o644
	umaskDir    = 0o755
)

// setUmask sets the process umask for the test, restored after it; no test using it is parallel.
func setUmask(t *testing.T, mask int) {
	t.Helper()
	old := syscall.Umask(mask)
	t.Cleanup(func() { syscall.Umask(old) })
}

// modeOf is the permission bits of the file or directory at path.
func modeOf(t *testing.T, path string) fs.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

// canonOK is canon's main in dir with args, which must exit 0.
func canonOK(t *testing.T, dir string, args ...string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := cli.Main(context.Background(), args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: dir}); code != 0 {
		t.Fatalf("%v: exit %d, stderr %q", args, code, stderr.String())
	}
}

// handoff 2026-10-07 A2, CLI.md §3.1, §3.2: under umask 022 init and new create 0644 files, 0755 directories.
func TestCreatedFileModes(t *testing.T) {
	setUmask(t, umaskMask)
	dir := t.TempDir()
	canonOK(t, dir, "init", "--name", "acme")
	canonOK(t, dir, "new", "game.items")
	for mode, rels := range map[fs.FileMode][]string{ //canon:unordered each path is judged alone
		umaskOthers: {"project.canon", ".gitignore", "game/items/items.canon"},
		umaskDir:    {"game", "game/items"},
	} {
		for _, rel := range rels {
			if got := modeOf(t, filepath.Join(dir, filepath.FromSlash(rel))); got != mode {
				t.Errorf("%s: mode %o, want %o", rel, got, mode)
			}
		}
	}
}

// handoff 2026-10-07 A2, CLI.md §2.7: init's rewrite of an existing .gitignore keeps its mode.
func TestGitignoreKeepsMode(t *testing.T) {
	setUmask(t, umaskMask)
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("bin/\n"), umaskGroup); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, umaskGroup); err != nil {
		t.Fatal(err)
	}
	canonOK(t, dir, "init", "--name", "acme")
	if got := modeOf(t, path); got != umaskGroup {
		t.Errorf(".gitignore: mode %o after init, want %o", got, umaskGroup)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "bin/\n.canon/\n" {
		t.Errorf(".gitignore = %q, %v", data, err)
	}
}
