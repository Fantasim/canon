package cppgen_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// boundLines are bound constants of Probe with their T19 comments (CODEGEN.md §2.5, §5.15), in the order the class writes them.
var boundLines = []string{
	"    /// Lower bound of n, from its refinement 1..=120.\n    static constexpr int64_t kNMin = 1;\n" +
		"    /// Upper bound of n, from its refinement 1..=120.\n    static constexpr int64_t kNMax = 120;\n",
	"    /// Upper bound of u8, from its refinement 0..=255.\n    static constexpr int64_t kU8Max = 255;\n",
	"    /// Upper bound of i8, from its refinement 0..=300.\n    static constexpr int64_t kI8Max = 300;\n",
	"    /// Exclusive upper bound of ex, from its refinement ..5.\n    static constexpr int64_t kExEnd = 5;\n",
	"    /// Upper bound of f32, from its refinement ..=0.1.\n    static constexpr double kF32Max = 0.1;\n",
	"    /// Lower bound of d, from its refinement 1s..=1h.\n    static constexpr std::chrono::milliseconds kDMin = std::chrono::milliseconds{1000};\n",
	"    /// Upper bound of the byte length of s, from its refinement 1..=4.\n    static constexpr int64_t kSLenMax = 4;\n",
	"    /// Upper bound of the element count of tags, from its refinement ..=3.\n    static constexpr int64_t kTagsLenMax = 3;\n",
	"    /// Upper bound of each element of tags, from its refinement 0..=5.\n    static constexpr int64_t kTagsElemMax = 5;\n",
	"    /// Lower bound of the byte length of each key of m, from its refinement 1..=3.\n    static constexpr int64_t kMKeyLenMin = 1;\n",
	"    /// Upper bound of each value of m, from its refinement 0..=9.\n    static constexpr int64_t kMValueMax = 9;\n",
}

// CODEGEN.md §5.15, §3.3, T19, DECISIONS 400: each finite side of a field's refinement is a static constexpr member of its class, written after Decode and Parse, a driver can static_assert.
func TestCppBoundConstants(t *testing.T) {
	t.Parallel()
	p := rangePackage()
	var header string
	for _, f := range generate(t, p) {
		if strings.HasSuffix(f.Path, ".gen.h") {
			header = string(f.Content)
		}
	}
	for _, want := range boundLines {
		if !strings.Contains(header, want) {
			t.Errorf("header lacks:\n%s", want)
		}
	}
	probe := header[strings.Index(header, "class Probe {"):]
	if first := strings.Index(probe, "kNMin"); first < strings.Index(probe, ir.CppParse) {
		t.Errorf("a bound constant precedes Parse")
	}
	dir := t.TempDir()
	writeTree(t, dir, generate(t, p), "range_consts_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"range.gen.cpp", "main.cpp"}) {
		if out != "ok\n" {
			t.Errorf("driver printed %q", out)
		}
	}
}

// failLine is the line of the generated decoder that fails key, "" when it writes none.
func failLine(source, key string) string {
	for _, line := range strings.Split(source, "\n") {
		if strings.Contains(line, `dec.Fail("`+key+`"`) {
			return line
		}
	}
	return ""
}

// CODEGEN.md §5.15, DECISIONS 394: a side the stored type guarantees writes no comparison (clang accepts them, GCC's -Wtype-limits refuses them).
func TestCppRangeImpliedSidesWriteNoCompare(t *testing.T) {
	t.Parallel()
	var source string
	for _, f := range generate(t, rangePackage()) {
		if strings.HasSuffix(f.Path, ".gen.cpp") {
			source = string(f.Content)
		}
	}
	if line := failLine(source, "u8"); line != "" {
		t.Errorf("UInt8(0..=255) writes a check: %s", line)
	}
	line := failLine(source, "i8")
	if !strings.Contains(line, ">= 0") || strings.Contains(line, "<=") {
		t.Errorf("Int8(0..=300) should compare its lower side only: %q", line)
	}
}
