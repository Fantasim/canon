package canon_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// The lock case on the OS: x/lk links to a's canon.lock, and the user overlays x/lk.
const (
	lockLinkName   = "x/lk"
	lockLinkTarget = "../a/canon.lock"
	lockLinkBuffer = "# canon.lock v1\n"
	lockLinkDir    = 0o750
	lockLinkFile   = 0o600
)

// API.md S12, E20 (log-2026-09-29 M4 P14-r4): an edit that adds an id to a stable table, whose
// lock lines it must write, is refused when an overlay covers the lock under another name, the
// lock there or not yet (the link dangling), and the disk is left as it was.
func TestEditRefusesOverlaidLockLink(t *testing.T) {
	for _, present := range []bool{true, false} {
		dir := t.TempDir()
		for name, text := range editLaw { //canon:unordered each file is written alone
			if name == "a/canon.lock" && !present {
				continue
			}
			p := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(p), lockLinkDir); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(text), lockLinkFile); err != nil {
				t.Fatal(err)
			}
		}
		link := filepath.Join(dir, filepath.FromSlash(lockLinkName))
		if err := os.MkdirAll(filepath.Dir(link), lockLinkDir); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.FromSlash(lockLinkTarget), link); err != nil {
			t.Skipf("no symbolic links here: %v", err)
		}
		lockRefused(t, dir, present)
	}
}

// lockRefused adds an id with x/lk overlaid and fails unless the edit is refused and the lock
// is as it was: editLaw's, or absent.
func lockRefused(t *testing.T, dir string, present bool) {
	t.Helper()
	p, err := canon.Open(dir, canon.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	if err := p.SetOverlay(lockLinkName, []byte(lockLinkBuffer)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{addSecond}}); !errors.Is(err, canon.ErrOverlay) {
		t.Errorf("API.md S12 (lock present %v): %v, want ErrOverlay", present, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "a", "canon.lock"))
	switch {
	case present && (err != nil || string(data) != editLaw["a/canon.lock"]):
		t.Errorf("the lock changed: %q, %v", data, err)
	case !present && !errors.Is(err, os.ErrNotExist):
		t.Errorf("a lock was created: %q, %v", data, err)
	}
}
