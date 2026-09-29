package cxx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx/config"
)

// Compilers finds every compiler on PATH, plus a clang++ -stdlib=libc++ row wherever libc++ is
// installed (meta/state.md: MSVC unverifiable here; DetectMSVC and MSVCCompileArgs cover it),
// or gates the test.
func Compilers(t *testing.T) []string {
	t.Helper()
	found := lookupCompilers(t)
	if len(found) == 0 {
		gate(t, noCompilerMsg)
	}
	return found
}

// Toolchain finds every compiler on PATH (as Compilers does) and the nlohmann/json include
// directory reachable from the working directory, or gates the test on whichever piece is
// missing.
func Toolchain(t *testing.T) (found []string, include string) {
	t.Helper()
	found = lookupCompilers(t)
	if len(found) == 0 {
		gate(t, noCompilerMsg)
	}
	if dir, ok := findNlohmann(); ok {
		return found, dir
	}
	gate(t, noNlohmannMsg)
	return nil, ""
}

// lookupCompilers is every compilerNames entry found on PATH, with clang++'s libc++ row
// (libcxxRow) appended right after plain clang++ when it applies.
func lookupCompilers(t *testing.T) (found []string) {
	t.Helper()
	for _, c := range compilerNames {
		p, err := exec.LookPath(c)
		if err != nil {
			continue
		}
		found = append(found, p)
		if c == clangxx {
			found = append(found, libcxxRow(t, p)...)
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

// FindNlohmann is Toolchain's nlohmann/json.hpp lookup alone, for a caller that needs the
// include directory without also needing a g++ or clang++ on PATH (testkit/golden's MSVC check).
func FindNlohmann() (string, bool) { return findNlohmann() }

// findNlohmann is the include directory holding nlohmann/json.hpp: config.NlohmannInclude's
// override (Windows CI, where no system path holds it) when set, else the vendored copy at
// every depth up to maxParentDirs, else a system install.
func findNlohmann() (string, bool) {
	if dir, ok := config.NlohmannInclude(); ok {
		return hasNlohmann(dir)
	}
	up := ""
	for i := 0; i <= maxParentDirs; i++ {
		if abs, ok := hasNlohmann(filepath.Join(up, "Source", "External")); ok {
			return abs, true
		}
		up = filepath.Join(up, "..")
	}
	for _, dir := range systemIncludeDirs {
		if abs, ok := hasNlohmann(dir); ok {
			return abs, true
		}
	}
	return "", false
}

// hasNlohmann reports whether dir/nlohmann/json.hpp exists, returning dir's absolute form.
func hasNlohmann(dir string) (string, bool) {
	if _, err := os.Stat(filepath.Join(dir, "nlohmann", "json.hpp")); err != nil {
		return "", false
	}
	abs, err := filepath.Abs(dir)
	return abs, err == nil
}
