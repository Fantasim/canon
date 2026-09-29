package build_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

var errInjected = errors.New("injected")

// faultyOS is build.OS with the operations named by its flags failing.
type faultyOS struct {
	build.WriteFS
	failWrite, failRename bool
}

func (f faultyOS) WriteFile(name string, data []byte) error {
	if f.failWrite {
		return errInjected
	}
	return f.WriteFS.WriteFile(name, data)
}

func (f faultyOS) Rename(oldname, newname string) error {
	if f.failRename {
		return errInjected
	}
	return f.WriteFS.Rename(oldname, newname)
}

// chmodFails is a file system that can set permissions, and fails to.
type chmodFails struct{ build.WriteFS }

func (chmodFails) Chmod(string, os.FileMode) error { return errInjected }

// oldFile is a directory holding one file, a.canon, of the given mode.
func oldFile(t *testing.T) (dir, abs string) {
	t.Helper()
	dir = t.TempDir()
	abs = filepath.ToSlash(filepath.Join(dir, "a.canon"))
	if err := os.WriteFile(abs, []byte("old\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	return dir, abs
}

// entries is the number of files in dir: a temporary file left behind would count.
func entries(t *testing.T, dir string) int {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(list)
}

// API.md N11: WriteAtomic replaces a file through a temporary file, keeping its permissions.
func TestWriteAtomicReplaces(t *testing.T) {
	dir, abs := oldFile(t)
	if err := build.WriteAtomic(build.OS(), abs, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(abs)
	if err != nil || string(got) != "new\n" || entries(t, dir) != 1 {
		t.Fatalf("file %q (%v), %d entries", got, err, entries(t, dir))
	}
	info, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if unixModes() && info.Mode().Perm() != 0o640 {
		t.Errorf("mode %v, want 0640", info.Mode().Perm())
	}
}

// API.md N11: after a failed write, rename or chmod the file is as it was and no temporary file is left.
func TestWriteAtomicFailures(t *testing.T) {
	tests := []struct {
		name string
		fsys build.WriteFS
	}{
		{"write fails", faultyOS{WriteFS: build.OS(), failWrite: true}},
		{"rename fails", faultyOS{WriteFS: build.OS(), failRename: true}},
		{"chmod fails", chmodFails{build.OS()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, abs := oldFile(t)
			if err := build.WriteAtomic(tt.fsys, abs, []byte("new\n")); !errors.Is(err, errInjected) {
				t.Fatalf("error %v", err)
			}
			got, err := os.ReadFile(abs)
			if err != nil || string(got) != "old\n" || entries(t, dir) != 1 {
				t.Errorf("file %q (%v), %d entries", got, err, entries(t, dir))
			}
		})
	}
}
