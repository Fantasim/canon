package tsgen_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// fileHeader separates the files of a case in its expected output.
const fileHeader = "=== %s ===\n"

// CODEGEN.md §2.7, §8: each case builds from its sources and generates the same files twice; the files are the case's golden, written by -update.
func TestGenerated(t *testing.T) {
	golden.Run(t, "testdata/ts/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		w := buildWorld(t, c.Archive.Files)
		if all, errors := w.findings(t); errors > 0 {
			t.Fatalf("%s does not build: %v", c.Path, all)
		}
		var b bytes.Buffer
		for _, f := range w.generate(t) {
			fmt.Fprintf(&b, fileHeader, f.Path)
			b.Write(f.Content)
		}
		return b.Bytes()
	}, golden.Expected(generatedTS))
}
