package cxx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx/config"
)

// Compilers finds every compiler on PATH (meta/state.md: MSVC unverifiable), or gates the test.
func Compilers(t *testing.T) []string {
	t.Helper()
	found := lookupCompilers()
	if len(found) == 0 {
		gate(t, noCompilerMsg)
	}
	return found
}

// Toolchain finds every compiler on PATH and the nlohmann/json include directory reachable
// from the working directory, or gates the test on whichever piece is missing.
func Toolchain(t *testing.T) (found []string, include string) {
	t.Helper()
	found = lookupCompilers()
	if len(found) == 0 {
		gate(t, noCompilerMsg)
	}
	if dir, ok := findNlohmann(); ok {
		return found, dir
	}
	gate(t, "nlohmann/json.hpp not found")
	return nil, ""
}

// lookupCompilers is every compilerNames entry found on PATH.
func lookupCompilers() (found []string) {
	for _, c := range compilerNames {
		if p, err := exec.LookPath(c); err == nil {
			found = append(found, p)
		}
	}
	return found
}

// gate skips the test on msg, or fails it when config.RequireCxx is set.
func gate(t *testing.T, msg string) {
	t.Helper()
	if config.RequireCxx() {
		t.Fatal(msg)
	}
	t.Skip(msg)
}

// findNlohmann is the include directory holding nlohmann/json.hpp: the vendored copy at every
// depth up to maxParentDirs, then a system install.
func findNlohmann() (string, bool) {
	up := ""
	for i := 0; i <= maxParentDirs; i++ {
		if abs, ok := hasNlohmann(filepath.Join(up, "Source", "External")); ok {
			return abs, true
		}
		up = filepath.Join(up, "..")
	}
	return hasNlohmann("/usr/include")
}

// hasNlohmann reports whether dir/nlohmann/json.hpp exists, returning dir's absolute form.
func hasNlohmann(dir string) (string, bool) {
	if _, err := os.Stat(filepath.Join(dir, "nlohmann", "json.hpp")); err != nil {
		return "", false
	}
	abs, err := filepath.Abs(dir)
	return abs, err == nil
}
