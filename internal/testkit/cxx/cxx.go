package cxx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Toolchain finds every compiler on PATH and the nlohmann/json include directory reachable
// from the working directory: the vendored copy, walked up toward the repository root, or a
// system install. It skips the test when either is missing (meta/state.md: MSVC unverifiable).
func Toolchain(t *testing.T) (found []string, include string) {
	t.Helper()
	for _, c := range compilerNames {
		if p, err := exec.LookPath(c); err == nil {
			found = append(found, p)
		}
	}
	if len(found) == 0 {
		t.Skip("no C++ compiler (g++, clang++) on PATH")
	}
	if dir, ok := findNlohmann(); ok {
		return found, dir
	}
	t.Skip("nlohmann/json.hpp not found")
	return nil, ""
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
