package writelock

import (
	"errors"
	"testing"
)

const tryRule = "DECISIONS 375, API.md N11: "

// tryTake is TryAcquire of reqs in dir, failing the test on an error; the lock is released with t.
func tryTake(t *testing.T, dir string, reqs ...Request) (*Held, bool) {
	t.Helper()
	h, ok, err := TryAcquire(dir, reqs)
	if err != nil {
		t.Fatal(err)
	}
	if h != nil {
		t.Cleanup(func() { _ = h.Release() })
	}
	return h, ok
}

// API.md N11: a free lock is taken without waiting, and held until released.
func TestTryAcquireTakesFree(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	h, ok := tryTake(t, dir, ex(pkgB), ex(pkgA))
	if !ok || !h.Holds(pkgA, Exclusive) || !h.Holds(pkgB, Exclusive) {
		t.Fatalf("%sfree locks not taken: %v", tryRule, ok)
	}
	if _, ok := tryTake(t, dir, ex(pkgA)); ok {
		t.Errorf("%sa held lock taken", tryRule)
	}
	if err := h.Release(); err != nil {
		t.Fatal(err)
	}
	if _, ok := tryTake(t, dir, ex(pkgA)); !ok {
		t.Errorf("%sa released lock busy", tryRule)
	}
}

// API.md N11 (J3): all or none, and a second open file of one process is busy.
func TestTryAcquireAllOrNone(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	acquire(t, dir, ex(pkgB))
	h, ok := tryTake(t, dir, ex(pkgA), sh(pkgB))
	if ok || h != nil {
		t.Fatalf("%sa set with a held lock was taken", tryRule)
	}
	if err := within(dir, held, ex(pkgA)); err != nil {
		t.Errorf("%sthe first lock stayed held: %v", tryRule, err)
	}
}

// API.md N11 (DECISIONS 352): shared holders admit each other, an exclusive one is busy among them.
func TestTryAcquireModes(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	tryTake(t, dir, sh(pkgA))
	if _, ok := tryTake(t, dir, sh(pkgA)); !ok {
		t.Errorf("%sshared beside shared busy", tryRule)
	}
	if _, ok := tryTake(t, dir, ex(pkgA)); ok {
		t.Errorf("%sexclusive among shared holders taken", tryRule)
	}
}

// J2: Valid is the package-name rule of Acquire.
func TestValid(t *testing.T) {
	for name, bad := range map[string]bool{"a": false, "a.b": false, "": true, ".a": true, "a/b": true, `a\b`: true} {
		if err := Valid(name); bad != errors.Is(err, ErrBadPackage) || (!bad && err != nil) {
			t.Errorf("%s%q: %v", tryRule, name, err)
		}
	}
}

// API.md N11 (J3): another process's lock makes TryAcquire busy; its death frees it.
func TestTryAcquireAcrossProcesses(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	cmd := startHelper(t, holdRole, dir, pkgA)
	if _, ok := tryTake(t, dir, ex(pkgB), ex(pkgA)); ok {
		t.Fatalf("%sheld by another process, taken", tryRule)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait() // killed: its error says so
	if _, ok := tryTake(t, dir, ex(pkgB), ex(pkgA)); !ok {
		t.Errorf("%safter the holder was killed, busy", tryRule)
	}
}
