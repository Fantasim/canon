package rules_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// IMPLEMENTATION-PLAN.md §7.2: each case prints its project's findings, views' included.
func TestFindings(t *testing.T) {
	golden.Run(t, "../testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		fsys := archiveFS(c.Archive)
		out := analyze(t, fsys, lawDir, build.Options{}).render(t)
		code := strings.SplitN(filepath.Base(c.Path), "_", 2)[0]
		if !strings.Contains(out, "["+code+"]") {
			t.Errorf("%s does not produce %s", c.Path, code)
		}
		return []byte(out)
	}, golden.Expected(findingsFile))
}
