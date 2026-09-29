package cxx

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// libcxxProbeMain reports which standard library it was built against, so the test proves the
// libc++ row actually links against libc++ and not merely that it compiles.
const libcxxProbeMain = `#include <cstdio>
int main() {
#ifdef _LIBCPP_VERSION
	printf("libc++\n");
#else
	printf("other\n");
#endif
	return 0;
}
`

// TestShellQuote proves shellQuote's output round-trips through a POSIX shell back to the
// original string, including one holding a single quote: the wrapper script libcxxWrapper
// writes depends on it to carry clang++'s own path unchanged (GOAL 1, libcxx.go).
func TestShellQuote(t *testing.T) {
	cases := []string{"/usr/bin/clang++", "/opt/llvm 21/bin/clang++", "it's/clang++"}
	for _, in := range cases {
		script := "printf '%s' " + shellQuote(in)
		out, err := exec.Command("/bin/sh", "-c", script).Output()
		if err != nil {
			t.Fatalf("shellQuote(%q): %v", in, err)
		}
		if got := string(out); got != in {
			t.Errorf("shellQuote(%q) round trip = %q, want %q", in, got, in)
		}
	}
}

// TestLibcxxRowBuildsAgainstLibcxx proves Compilers' clang++ -stdlib=libc++ row actually links
// against libc++, or skips cleanly where none is installed.
func TestLibcxxRowBuildsAgainstLibcxx(t *testing.T) {
	wrapper := findLibcxxRow(t, Compilers(t))
	if wrapper == "" {
		t.Skip("no clang++ -stdlib=libc++ row (libc++ not installed here)")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "probe.cpp")
	if err := os.WriteFile(src, []byte(libcxxProbeMain), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "probe")
	args := append(append([]string(nil), Flags...), "-o", bin, src)
	if out, err := exec.Command(wrapper, args...).CombinedOutput(); err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	var stdout bytes.Buffer
	run := exec.Command(bin)
	run.Stdout = &stdout
	if err := run.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got, want := stdout.String(), "libc++\n"; got != want {
		t.Errorf("probe printed %q, want %q", got, want)
	}
}

// findLibcxxRow is the wrapperName entry of compilers, or "" when there is none.
func findLibcxxRow(t *testing.T, compilers []string) string {
	t.Helper()
	for _, cc := range compilers {
		if filepath.Base(cc) == wrapperName {
			return cc
		}
	}
	return ""
}
