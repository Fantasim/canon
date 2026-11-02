package writelock

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The paths and ops of the relative tests, and the rule they prove.
const (
	ruleDisplay = "DECISIONS 201, 416: "
	firstName   = "one"
	secondName  = "two/three"
	opFirst     = "first"
	opSecond    = "second"
)

// errCause is the cause a rewritten path error must keep.
var errCause = errors.New("cause")

// DECISIONS 201, 416: Release after a closed handle names the lock file relative to the project.
func TestReleaseNamesNoAbsolutePath(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	h, err := Acquire(t.Context(), dir, []Request{ex(pkgA)})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.files[0].Close(); err != nil {
		t.Fatal(err)
	}
	err = h.Release()
	if err == nil {
		t.Fatalf("%sRelease of a closed handle: no error", ruleDisplay)
	}
	if strings.Contains(err.Error(), dir) {
		t.Errorf("%sabsolute path in %q", ruleDisplay, err)
	}
	if !errors.Is(err, os.ErrClosed) {
		t.Errorf("%scause lost: %v", ruleDisplay, err)
	}
	if want := writersPath + pathSep + lockPrefix + pkgA; !strings.Contains(err.Error(), want) {
		t.Errorf("%sdisplay path %q missing from %q", ruleDisplay, want, err)
	}
}

// DECISIONS 201, 416: stamp on a closed file joins the write error and the close error, both relative.
func TestStampNamesNoAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, firstName))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	err = stamp(f, dir)
	if err == nil {
		t.Fatalf("%sstamp of a closed file: no error", ruleDisplay)
	}
	if strings.Contains(err.Error(), dir) {
		t.Errorf("%sabsolute path in %q", ruleDisplay, err)
	}
}

// DECISIONS 201, 416: relative rewrites every path error of a joined tree, in order, keeping the
// cause; nil stays nil.
func TestRelativeJoined(t *testing.T) {
	dir := t.TempDir()
	first := &fs.PathError{Op: opFirst, Path: filepath.Join(dir, firstName), Err: errCause}
	second := &fs.PathError{Op: opSecond, Path: filepath.Join(dir, filepath.FromSlash(secondName)), Err: fs.ErrClosed}
	if relative(dir, nil) != nil {
		t.Errorf("%snil became an error", ruleDisplay)
	}
	got := relative(dir, errors.Join(first, second))
	text := got.Error()
	if strings.Contains(text, dir) {
		t.Errorf("%sabsolute path in %q", ruleDisplay, text)
	}
	want := opFirst + " " + firstName + ": " + errCause.Error() + "\n" + opSecond + " " + secondName + ": " + fs.ErrClosed.Error()
	if text != want {
		t.Errorf("%sgot %q, want %q", ruleDisplay, text, want)
	}
	if !errors.Is(got, errCause) || !errors.Is(got, fs.ErrClosed) {
		t.Errorf("%scause lost: %v", ruleDisplay, got)
	}
}
