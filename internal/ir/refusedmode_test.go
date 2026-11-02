package ir_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// TestRefusedModeNames is DECISIONS 213: an emit whose mode check refused (E8009) skips only the
// mode-dependent name problems; each case's golden holds the E8009 and what holds in every mode.
func TestRefusedModeNames(t *testing.T) {
	golden.Run(t, "testdata/refusedmode/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		w := newWorld(t)
		for _, f := range c.Archive.Files {
			if f.Name != findingsFile {
				w.add(t, f.Name, f.Data)
			}
		}
		w.calls = w.fixtureCalls
		w.build(t)
		out := w.findings(t)
		if !strings.Contains(out, "["+string(diag.E8009.Def().Code)+"]") {
			t.Errorf("%s: no refused mode:\n%s", c.Path, out)
		}
		return []byte(out)
	}, golden.Expected(findingsFile))
}
