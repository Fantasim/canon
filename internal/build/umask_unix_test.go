//go:build unix

package build_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

const (
	umaskSource = textSource + "/// A.\nrecord Tier {\n  /// W.\n  weight: Int = 1\n}\n\n" +
		"/// Tiers.\nlet tiers: stable table Tier = { low {} }\n\n" +
		"emit text { out: \"@out/sql\" }\nemit json { out: \"@out/json/\" }\n"
	umaskGroup  = 0o664
	umaskOthers = 0o644
	umaskMask   = 0o022
	umaskDir    = 0o755
)

// umaskFiles are the files a build of umaskSource writes: outputs, the text manifest and a lock.
var restoreFiles = []string{"out/sql/Schema.sql", "out/sql/_version.sql"}

var umaskFiles = []string{"out/sql/Schema.sql", "out/sql/.canon-text", "out/json/tiers.json", "a/canon.lock"}

// setUmask sets the process umask for the test, restored after it; no test using it is parallel.
func setUmask(t *testing.T, mask int) {
	t.Helper()
	old := syscall.Umask(mask)
	t.Cleanup(func() { syscall.Umask(old) })
}

// modeOf is the permission bits of the file at path.
func modeOf(t *testing.T, path string) fs.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

// umaskProject writes a project holding src to a directory, and returns it with a build of it on the OS.
func umaskProject(t *testing.T, src string) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a"), umaskDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project.canon"), []byte(textProject), umaskOthers); err != nil {
		t.Fatal(err)
	}
	rewrite(t, dir, src)
	return dir, func() {
		p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Build(context.Background(), build.BuildOptions{})
		if err != nil || res.Summary.Errors != 0 {
			t.Fatalf("build: %v, %v", err, codes(res.List))
		}
	}
}

// rewrite replaces the source of package a in dir.
func rewrite(t *testing.T, dir, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "a", "a.canon"), []byte(src), umaskOthers); err != nil {
		t.Fatal(err)
	}
}

// handoff 2026-10-07 A2, CODEGEN.md §2.4, §2.9, LOCK.md: under umask 022 new outputs, manifest and lock get 0644.
func TestOSWriteNewFileModes(t *testing.T) {
	setUmask(t, umaskMask)
	dir, run := umaskProject(t, umaskSource)
	run()
	for _, name := range umaskFiles {
		if got := modeOf(t, filepath.Join(dir, filepath.FromSlash(name))); got != umaskOthers {
			t.Errorf("%s: mode %o, want %o", name, got, umaskOthers)
		}
	}
	if got := modeOf(t, filepath.Join(dir, "out", "sql")); got != umaskDir {
		t.Errorf("out/sql: mode %o, want %o", got, umaskDir)
	}
}

// failLastRename is the OS file system whose n-th rename fails.
type failLastRename struct {
	build.WriteFS
	left *int
}

// Chmod forwards to the OS, so that the writes keep their modes.
func (f failLastRename) Chmod(name string, mode fs.FileMode) error {
	return f.WriteFS.(build.ModeFS).Chmod(name, mode)
}

func (f failLastRename) Rename(oldname, newname string) error {
	*f.left--
	if *f.left == 0 {
		return errInjected
	}
	return f.WriteFS.Rename(oldname, newname)
}

// handoff 2026-10-07 A2, API.md N11: a file restored after a failed write keeps its mode too.
func TestOSWriteRestoreKeepsMode(t *testing.T) {
	setUmask(t, umaskMask)
	dir, run := umaskProject(t, textEmit)
	run()
	for _, name := range restoreFiles {
		if err := os.Chmod(filepath.Join(dir, filepath.FromSlash(name)), umaskGroup); err != nil {
			t.Fatal(err)
		}
	}
	rewrite(t, dir, strings.Replace(strings.Replace(textEmit, "id INT", "id BIGINT", 1), "VERSION = 3", "VERSION = 4", 1))
	left := 2
	p, err := build.Open(failLastRename{WriteFS: build.OS(), left: &left}, filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Build(context.Background(), build.BuildOptions{}); !errors.Is(err, errInjected) {
		t.Fatalf("build: %v", err)
	}
	for _, name := range restoreFiles {
		if got := modeOf(t, filepath.Join(dir, filepath.FromSlash(name))); got != umaskGroup {
			t.Errorf("%s: mode %o after a restore, want %o", name, got, umaskGroup)
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "out", "sql", "Schema.sql")); err != nil || strings.Contains(string(data), "BIGINT") {
		t.Errorf("Schema.sql was not restored: %q, %v", data, err)
	}
}

// handoff 2026-10-07 A2: an overwrite keeps the mode the file has, a rewrite that changes it included.
func TestOSWriteKeepsExistingMode(t *testing.T) {
	setUmask(t, umaskMask)
	dir, run := umaskProject(t, umaskSource)
	run()
	for _, name := range umaskFiles {
		if err := os.Chmod(filepath.Join(dir, filepath.FromSlash(name)), umaskGroup); err != nil {
			t.Fatal(err)
		}
	}
	changed := strings.Replace(umaskSource, "id INT", "id BIGINT", 1)
	rewrite(t, dir, strings.Replace(changed, "low {}", "low { weight: 2 }, high {}", 1))
	run()
	for _, name := range umaskFiles {
		if got := modeOf(t, filepath.Join(dir, filepath.FromSlash(name))); got != umaskGroup {
			t.Errorf("%s: mode %o after a rewrite, want %o", name, got, umaskGroup)
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "out", "sql", "Schema.sql")); err != nil || !strings.Contains(string(data), "BIGINT") {
		t.Errorf("Schema.sql was not rewritten: %q, %v", data, err)
	}
}
