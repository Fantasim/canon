package golden_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// IMPLEMENTATION-PLAN.md §7.1, M0 acceptance: the harness runs one trivial golden.
func TestTrivialGolden(t *testing.T) {
	golden.Run(t, "testdata/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		return c.Archive.Files[0].Data
	})
}
