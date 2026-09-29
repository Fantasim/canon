package cxx_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// TestFindNlohmannMatchesToolchain proves FindNlohmann agrees with Toolchain's own lookup: the
// nlohmann/json discovery testkit/golden's MSVC check reuses without needing a g++ or clang++.
func TestFindNlohmannMatchesToolchain(t *testing.T) {
	_, wantInclude := cxx.Toolchain(t)
	gotInclude, ok := cxx.FindNlohmann()
	if !ok {
		t.Fatal("FindNlohmann: not found, but Toolchain succeeded")
	}
	if gotInclude != wantInclude {
		t.Errorf("FindNlohmann() = %q, want %q", gotInclude, wantInclude)
	}
}
