package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// API.md §2.2: FS names use '/', so the OS resolver turns a Windows result into C:/... form.
func TestSlashed(t *testing.T) {
	cases := []struct {
		in    string
		osSep rune
		want  string
	}{
		{`C:\proj\data\a.json`, '\\', "C:/proj/data/a.json"},
		{`C:\`, '\\', "C:/"},
		{`\\server\share\x`, '\\', "//server/share/x"},
		{"/proj/data/a.json", '/', "/proj/data/a.json"},
		{`/proj/back\slash`, '/', `/proj/back\slash`},
	}
	for _, c := range cases {
		if got := slashed(c.in, c.osSep); got != c.want {
			t.Errorf("slashed(%q, %q) = %q, want %q", c.in, c.osSep, got, c.want)
		}
	}
}

// noLinks is an FS with no EvalSymlinks method.
type noLinks struct{ FS }

// meta/decisions/log-2026-09-24.md "load.dir round 3": an FS without the capability resolves no link.
func TestEvalSymlinksWithoutCapability(t *testing.T) {
	if _, err := EvalSymlinks(noLinks{OS()}, "/"); !errors.Is(err, errNoLinks) {
		t.Errorf("err = %v, want errNoLinks", err)
	}
}

// WIRE.md §6.5: the OS FS resolves a link to its real '/' path; a broken one is an error.
func TestOSEvalSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevated privilege on windows")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	got, err := EvalSymlinks(OS(), filepath.ToSlash(link))
	if err != nil || got != filepath.ToSlash(target) {
		t.Errorf("EvalSymlinks(link) = %q, %v; want %q", got, err, filepath.ToSlash(target))
	}
	if err := os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "broken")); err != nil {
		t.Fatal(err)
	}
	if _, err := EvalSymlinks(OS(), filepath.ToSlash(filepath.Join(dir, "broken"))); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("broken link: err = %v, want fs.ErrNotExist", err)
	}
}
