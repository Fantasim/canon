package i18n_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const findingsFile = "findings.txt"

// TestFindings covers every reachable code of I18N.md §12, from a txtar fixture.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		out := runCase(t, c).render(t)
		code := strings.SplitN(filepath.Base(c.Path), "_", 2)[0]
		if !strings.Contains(out, "["+code+"]") {
			t.Errorf("%s does not produce %s", c.Path, code)
		}
		return []byte(out)
	}, golden.Expected(findingsFile))
}
