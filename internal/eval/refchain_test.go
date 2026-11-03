package eval_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// TYPES.md §4.1, §9.1, §11.6, WIRE.md §5.9, DECISIONS 456: a nested ref takes its leaf key's literal form.
func TestRefChain(t *testing.T) {
	golden.Run(t, "testdata/refchain/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		b := runBuild(t, fromArchive(t, c.Archive), eval.Options{})
		return []byte(b.dump() + "\n" + b.findings(t))
	}, golden.Expected(dumpFile))
}
