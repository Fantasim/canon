package cxx

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx/config"
)

// libcxx caches detectLibcxx's result process-wide: at most once per test binary, however many
// tests call Toolchain or Compilers.
var libcxx struct {
	once    sync.Once
	wrapper string
	ok      bool
}

// libcxxRow is clang++'s -stdlib=libc++ wrapper, appended right after plain clangxx wherever
// libc++ is installed: a second row of the compile-and-run matrix "for free" (GOAL 1).
func libcxxRow(t *testing.T, clangxx string) []string {
	t.Helper()
	libcxx.once.Do(func() { libcxx.wrapper, libcxx.ok = detectLibcxx(clangxx) })
	if !libcxx.ok {
		if config.RequireCxx() {
			t.Fatalf("%s: %s", clangxx, noLibcxxMsg)
		}
		return nil
	}
	return []string{libcxx.wrapper}
}

// detectLibcxx writes clangxx's libc++ wrapper when libcxxUsable finds a libc++ install.
func detectLibcxx(clangxx string) (string, bool) {
	if !libcxxUsable() {
		return "", false
	}
	wrapper, err := libcxxWrapper(clangxx)
	return wrapper, err == nil
}

// libcxxUsable reports whether a libc++ install exists: always true on macOS, where Apple
// clang's own SDK bundles it as the platform default, else a libcxxHeaderGlobs match on Linux.
func libcxxUsable() bool {
	switch runtime.GOOS {
	case osDarwin:
		return true
	case osLinux:
		return libcxxHeadersFound()
	default:
		return false
	}
}

// libcxxHeadersFound reports whether any libcxxHeaderGlobs pattern holds a libc++ <vector>.
func libcxxHeadersFound() bool {
	for _, pattern := range libcxxHeaderGlobs {
		matches, err := filepath.Glob(filepath.Join(pattern, "vector"))
		if err == nil && len(matches) > 0 {
			return true
		}
	}
	return false
}

// libcxxWrapper is the absolute path of a POSIX shell shim that runs clangxx with libcxxFlag
// prepended to its own arguments, written once under os.TempDir() (an atomic rename guards
// against two test binaries writing it at once).
func libcxxWrapper(clangxx string) (string, error) {
	dir := filepath.Join(os.TempDir(), wrapperDirName)
	if err := os.MkdirAll(dir, wrapperPerm); err != nil {
		return "", fmt.Errorf("libc++ wrapper dir: %w", err)
	}
	path := filepath.Join(dir, wrapperName)
	script := wrapperShebang + "exec " + shellQuote(clangxx) + " " + libcxxFlag + " \"$@\"\n"
	tmp := path + wrapperTmpSuffix
	if err := os.WriteFile(tmp, []byte(script), wrapperPerm); err != nil {
		return "", fmt.Errorf("libc++ wrapper write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", fmt.Errorf("libc++ wrapper rename: %w", err)
	}
	return path, nil
}

// shellQuote wraps s in single quotes for a POSIX shell, escaping any single quote it contains.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
