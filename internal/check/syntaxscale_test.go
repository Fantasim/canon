package check_test

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// missingSeparators is a table of n rows whose fields lack their separators (E1117): every row
// holds three syntax errors inside one declaration.
func missingSeparators(n int) string {
	var sb strings.Builder
	sb.WriteString("package a\n\n/// Row.\nrecord Row {\n  /// f\n  f0: Int\n  /// f\n  f1: Int\n  /// f\n  f2: Int\n  /// f\n  f3: Int\n}\n\n/// Rows.\nlet rows: table Row = {\n")
	for k := range n {
		fmt.Fprintf(&sb, "  R%d { f0: 1 f1: 1 f2: 1 f3: 1 }\n", k+1)
	}
	sb.WriteString("}\n")
	return sb.String()
}

// checkAllocs is the heap allocations of checking, once, a file of n rows holding 3n syntax
// errors, its parse findings in the package's bag as a build puts them.
func checkAllocs(t *testing.T, n int) (mallocs uint64, errors int) {
	t.Helper()
	set := &source.FileSet{}
	src, err := set.Add(builtFile, "/"+builtFile, []byte(missingSeparators(n)))
	if err != nil {
		t.Fatal(err)
	}
	bags := check.Bags{builtPackage: diag.NewBag(set, builtPackage)}
	f := syntax.Parse(src, syntax.FileSource, bags[builtPackage])
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	check.Check(context.Background(), exampleProject(), []*syntax.File{f}, bags, literalFolder{})
	runtime.ReadMemStats(&after)
	return after.Mallocs - before.Mallocs, bags[builtPackage].Summary().Errors
}

// DECISIONS 209, NFR-01: four times the syntax errors cost about four times the allocations.
func TestSyntaxErrorsCostLinearly(t *testing.T) {
	small, smallErrors := checkAllocs(t, 400)
	large, largeErrors := checkAllocs(t, 1600)
	if smallErrors <= diag.DefaultMaxFindings || largeErrors != 4*smallErrors {
		t.Fatalf("%d and %d errors: the rows lost their syntax errors", smallErrors, largeErrors)
	}
	if large > small*6 {
		t.Errorf("%d errors cost %d allocations, %d cost %d: more than 6 times", smallErrors, small, largeErrors, large)
	}
}
