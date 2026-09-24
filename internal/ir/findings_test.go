package ir_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const findingsFile = "findings.txt"

// IMPLEMENTATION-PLAN.md §7.2: each emit rule's txtar case checks cleanly and fails only its stage-E rule; values come from its `<pkg>.<name>.json` files.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
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
		code := strings.SplitN(filepath.Base(c.Path), "_", 2)[0]
		if !strings.Contains(out, "["+code+"]") {
			t.Errorf("%s does not produce %s", c.Path, code)
		}
		return []byte(out)
	}, golden.Expected(findingsFile))
}
