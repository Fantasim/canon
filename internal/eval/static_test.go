package eval_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// TYPES.md §1, EVALUATION.md §1: a broken declaration is never evaluated: only the static error, and W4001 (DECISIONS 331).
func TestStaticErrorsDoNotCascade(t *testing.T) {
	golden.Run(t, "testdata/static/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		b := runBuild(t, fromArchive(t, c.Archive), eval.Options{})
		code := strings.SplitN(filepath.Base(c.Path), "_", 2)[0]
		for _, bag := range b.bags {
			for _, f := range bag.Findings() {
				if f.Severity == diag.Error && string(f.Code) != code {
					t.Errorf("%s: %s besides %s: %s", c.Path, f.Code, code, f.Message)
				}
			}
		}
		return []byte(b.findings(t))
	}, golden.Expected(findingsFile))
}
