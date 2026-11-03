package cppgen_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

const (
	bulkAccessName = "BulkAccess::"
	bulkSource     = "bulk.gen.cpp"
)

// DECISIONS 392, 435 (D-SIZE), CODEGEN.md §2.11, §7.3: a 7 400-row table fills its columns in one loop, its variants in parts of Build.
func TestBulkBuildIsSplit(t *testing.T) {
	if testing.Short() {
		t.Skip("generates a 7 MB source")
	}
	var src string
	for _, f := range generate(t, bulkPackage()) {
		if strings.HasSuffix(f.Path, bulkSource) {
			src = string(f.Content)
		}
	}
	parts, longest, in, n := 0, 0, false, 0
	for l := range strings.SplitSeq(src, "\n") {
		switch {
		case strings.Contains(l, bulkAccessName) && strings.HasSuffix(l, "{"):
			in, n = true, 0
			parts += strings.Count(l, "::Build") - strings.Count(l, "::Build()")
		case in && l == "}":
			in, longest = false, max(longest, n)
		case in:
			n++
		}
	}
	if parts < 2 || longest > ir.PartLines {
		t.Errorf("Build has %d parts, the longest function %d lines: want parts, each at most %d", parts, longest, ir.PartLines)
	}
	if !strings.Contains(src, "for (size_t i = 0; i < 7400; ++i) {") {
		t.Error("the items' columns are not one loop over the rows")
	}
}
