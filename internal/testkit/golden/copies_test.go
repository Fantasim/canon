package golden

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// copiesCppSources are the C++ sources of the copies of features.copies and features.copies.base
// under examples/features/copies/expected/, two per owning root.
var copiesCppSources = []string{
	"copies/out/cpp/base/base.gen.cpp", "copies/out/cpp/copies/copies.gen.cpp",
	"sovcommon/copies/cpp/base/base.gen.cpp", "sovcommon/copies/cpp/copies/copies.gen.cpp",
}

// CODEGEN.md §2.8, DECISIONS 229: each C++ copy compiles where it lies, including its sibling copy.
func TestCopiesCppCompiles(t *testing.T) {
	expected := filepath.Join(examplesDir, "features", "copies", "expected")
	compilers, include := cxx.Toolchain(t)
	obj := filepath.Join(t.TempDir(), "out.o")
	for _, src := range copiesCppSources {
		for _, cc := range compilers {
			for _, mode := range cxx.Modes {
				flags := append(append(append([]string(nil), cxx.Flags...), mode...), "-I", include, "-c", "-o", obj)
				compileOne(t, cc, flags, filepath.Join(expected, filepath.FromSlash(src)))
			}
		}
	}
}

// compileOne compiles src with cc and flags, -Werror included.
func compileOne(t *testing.T, cc string, flags []string, src string) {
	t.Helper()
	args := append(flags, src)
	ctx, cancel := context.WithTimeout(context.Background(), cxx.Timeout)
	defer cancel()
	if out, err := exec.CommandContext(ctx, cc, args...).CombinedOutput(); err != nil {
		t.Errorf("%s %s: %v\n%s", filepath.Base(cc), src, err, out)
	}
}
