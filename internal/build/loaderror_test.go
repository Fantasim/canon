package build_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// DECISIONS 196: a refusal names its cause in parentheses; with none reported yet, no "()".
func TestLoadErrorCause(t *testing.T) {
	site := source.Location{Path: "a/a.canon", Line: 5, Col: 16}
	cases := []struct {
		cause, want string
	}{
		{"", build.ErrLoad.Error() + ": a/a.canon:5:16"},
		{"a load form other than load.dir", build.ErrLoad.Error() + " (a load form other than load.dir): a/a.canon:5:16"},
	}
	for _, c := range cases {
		if got := (&build.LoadError{Site: site, Cause: c.cause}).Error(); got != c.want {
			t.Errorf("cause %q: %q, want %q", c.cause, got, c.want)
		}
	}
}

// WIRE.md §6.5, log-2026-09-24 "load.dir round 3": a build's OS FS resolves links for load.dir.
func TestOSResolvesLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevated privilege on windows")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target, link := filepath.Join(dir, "target"), filepath.Join(dir, "link")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	got, err := project.EvalSymlinks(build.OS(), filepath.ToSlash(link))
	if err != nil || got != filepath.ToSlash(target) {
		t.Errorf("EvalSymlinks(link) = %q, %v; want %q", got, err, filepath.ToSlash(target))
	}
}
