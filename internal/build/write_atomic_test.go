package build_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

var errInjected = errors.New("injected")

// longBase is a file name long enough that a long temporary name beside it would not fit.
const longBase = 220

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

// API.md §2.2, §10.3: the OS WriteFile replaces atomically, keeps modes, leaves nothing behind.
func TestOSWriteFileAtomic(t *testing.T) {
	dir, abs := oldFile(t)
	fsys := build.OS()
	if err := fsys.WriteFile(abs, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	added := filepath.ToSlash(filepath.Join(dir, "b.canon"))
	if err := fsys.WriteFile(added, []byte("b\n")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(abs)
	if err != nil || string(got) != "new\n" || entries(t, dir) != 2 {
		t.Fatalf("file %q (%v), %d entries", got, err, entries(t, dir))
	}
	info, err := os.Stat(abs)
	if err != nil {
		t.Fatal(err)
	}
	if unixModes() && info.Mode().Perm() != 0o640 {
		t.Errorf("mode %v, want 0640", info.Mode().Perm())
	}
	missing := filepath.ToSlash(filepath.Join(dir, "none", "c.canon"))
	if err := fsys.WriteFile(missing, []byte("c\n")); !errors.Is(err, os.ErrNotExist) || entries(t, dir) != 2 {
		t.Errorf("a write into no directory: %v, %d entries", err, entries(t, dir))
	}
}

// API.md §10.3 (log-2026-09-29 "U4c review FAIL" (f)): the OS file system's SyncDir.
func TestOSSyncDir(t *testing.T) {
	syncer, ok := build.OS().(interface{ SyncDir(dir string) error })
	if !ok {
		t.Fatal("build.OS has no SyncDir")
	}
	dir := filepath.ToSlash(t.TempDir())
	if err := syncer.SyncDir(dir); err != nil {
		t.Errorf("SyncDir(%s): %v", dir, err)
	}
	err := syncer.SyncDir(dir + "/none")
	if unixModes() && !errors.Is(err, os.ErrNotExist) || !unixModes() && err != nil {
		t.Errorf("SyncDir of a missing directory: %v", err)
	}
}

// ADR-0010 (log-2026-09-29 M4 U8-r): a link planted at an output's temporary name is removed, never written through.
func TestPlantedTempLinkRemoved(t *testing.T) {
	dir, abs := oldFile(t)
	outside := filepath.Join(t.TempDir(), "target.txt")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if os.Symlink(outside, filepath.Join(dir, ".a.canon.canon-tmp")) != nil {
		t.Skip("no symbolic links here")
	}
	if err := build.WriteAtomic(build.OS(), abs, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "outside\n" {
		t.Errorf("the link's target holds %q, %v", got, err)
	}
	info, err := os.Lstat(abs)
	got, _ := os.ReadFile(abs)
	if err != nil || !info.Mode().IsRegular() || string(got) != "new\n" || entries(t, dir) != 1 {
		t.Errorf("output %q (%v, %v), %d entries", got, info, err, entries(t, dir))
	}
}

// API.md §2.2: a reader of a file rewritten again and again sees one whole content or the other.
func TestOSWriteFileNeverPartial(t *testing.T) {
	if !unixModes() {
		t.Skip("Windows refuses a rename over a file a reader holds open (osfs.go writeSynced)")
	}
	_, abs := oldFile(t)
	short, long := []byte("old\n"), bytes.Repeat([]byte("long line\n"), 1<<12)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 64 {
			if err := build.OS().WriteFile(abs, [][]byte{long, short}[i%2]); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	for {
		select {
		case <-done:
			return
		default:
		}
		got, err := os.ReadFile(abs)
		if err != nil || !bytes.Equal(got, short) && !bytes.Equal(got, long) {
			<-done
			t.Fatalf("read %d bytes (%v): neither content", len(got), err)
		}
	}
}

// API.md §2.2 (log-2026-09-29 M4 U8-r): writes through links reach their targets; long names fit.
func TestOSWriteFileLinksAndLongNames(t *testing.T) {
	dir, abs := oldFile(t)
	link, dangling := filepath.Join(dir, "link.canon"), filepath.Join(dir, "dangling.canon")
	if os.Symlink("a.canon", link) != nil || os.Symlink("made.canon", dangling) != nil {
		t.Skip("no symbolic links here")
	}
	long := filepath.ToSlash(filepath.Join(dir, strings.Repeat("n", longBase)+".canon"))
	for _, name := range []string{link, dangling, long} {
		if err := build.OS().WriteFile(filepath.ToSlash(name), []byte("new\n")); err != nil {
			t.Fatalf("%s: %v", filepath.Base(name), err)
		}
	}
	for _, name := range []string{abs, filepath.Join(dir, "made.canon"), long} {
		if got, err := os.ReadFile(name); err != nil || string(got) != "new\n" {
			t.Errorf("%s holds %q, %v", filepath.Base(name), got, err)
		}
	}
	for _, name := range []string{link, dangling} {
		if info, err := os.Lstat(name); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is no longer a link: %v", filepath.Base(name), err)
		}
	}
}
