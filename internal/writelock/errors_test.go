package writelock_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/writelock"
)

const errRule = "DECISIONS 352: "

// lockA is Acquire of package a's exclusive lock in dir, released at once.
func lockA(dir string) error {
	h, err := writelock.Acquire(context.Background(), dir, []writelock.Request{{Package: "a", Mode: writelock.Exclusive}})
	if err != nil {
		return err
	}
	return h.Release()
}

// DECISIONS 352: a name that is not a plain file name of .canon/writers is refused.
func TestBadPackage(t *testing.T) {
	for _, name := range []string{"", ".a", "a/b", `a\b`} {
		_, err := writelock.Acquire(context.Background(), t.TempDir(), []writelock.Request{{Package: name, Mode: writelock.Exclusive}})
		if !errors.Is(err, writelock.ErrBadPackage) {
			t.Errorf("%s%q: %v", errRule, name, err)
		}
	}
}

// DECISIONS 352: an unwritable .canon refuses the lock; a link where a directory or the lock
// file should be is ErrUnsafe, and nothing is written through it.
func TestUnwritable(t *testing.T) {
	if !writelock.Lockable() {
		t.Skip("no lock primitive on this platform: the lock is skipped")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".canon"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := lockA(dir); err == nil {
		t.Errorf("%s.canon a file: no error", errRule)
	}
	outside := t.TempDir()
	victim := filepath.Join(outside, "a")
	if err := os.WriteFile(victim, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	linked := t.TempDir()
	if err := os.MkdirAll(filepath.Join(linked, ".canon", "writers"), 0o700); err != nil {
		t.Fatal(err)
	}
	if os.Symlink(victim, filepath.Join(linked, ".canon", "writers", "pkg.a")) != nil {
		t.Skip("no symbolic links here")
	}
	if err := lockA(linked); !errors.Is(err, writelock.ErrUnsafe) {
		t.Errorf("%sa linked lock file: %v", errRule, err)
	}
	dirLinked := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dirLinked, ".canon"), 0o700); err != nil || os.Symlink(outside, filepath.Join(dirLinked, ".canon", "writers")) != nil {
		t.Fatal(err)
	}
	if err := lockA(dirLinked); !errors.Is(err, writelock.ErrUnsafe) {
		t.Errorf("%sa linked writers directory: %v", errRule, err)
	}
	if data, err := os.ReadFile(victim); err != nil || string(data) != "keep" {
		t.Errorf("%sthe link's target holds %q (%v)", errRule, data, err)
	}
	canonLinked, target := t.TempDir(), t.TempDir()
	if os.Symlink(target, filepath.Join(canonLinked, ".canon")) != nil {
		t.Fatal("a link to .canon")
	}
	if err := lockA(canonLinked); !errors.Is(err, writelock.ErrUnsafe) {
		t.Errorf("%sa linked .canon: %v", errRule, err)
	}
	if list, err := os.ReadDir(target); err != nil || len(list) != 0 {
		t.Errorf("%sthe linked .canon's target got %v (%v)", errRule, list, err)
	}
}
