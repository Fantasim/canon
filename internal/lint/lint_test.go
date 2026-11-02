package lint_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// API.md §5.7 (DECISIONS 440): each case lints a project and prints its findings and fixes, named by the code it proves.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		return []byte(lintCase(t, c.Archive))
	}, golden.Expected(findingsFile))
}
