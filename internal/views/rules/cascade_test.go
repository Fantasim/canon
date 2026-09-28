package rules_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// TYPES.md §1, VIEWMODEL.md J4: views reports nothing on a view another package's error broke.
func TestBrokenViewsAreNotChecked(t *testing.T) {
	golden.Run(t, "testdata/cascade/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		fsys := archiveFS(c.Archive)
		x := analyze(t, fsys, lawDir, build.Options{})
		all, _ := x.findings()
		for _, f := range all {
			if owner(f.Code) == viewsOwner {
				t.Errorf("%s: views reported %s %s", c.Path, f.Code, f.Message)
			}
		}
		out := x.render(t)
		if !strings.Contains(out, "error[") {
			t.Errorf("%s: no finding of another package", c.Path)
		}
		return []byte(out)
	}, golden.Expected(findingsFile))
}
