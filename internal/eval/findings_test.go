package eval_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	findingsFile = "findings.txt"
	budgetFile   = "budget"
)

// IMPLEMENTATION-PLAN §7.2: each case produces the code its name starts with.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		var opt eval.Options
		for _, f := range c.Archive.Files {
			if f.Name == budgetFile {
				n, err := strconv.ParseInt(strings.TrimSpace(string(f.Data)), 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				opt.Budget = n
			}
		}
		out := runBuild(t, fromArchive(t, c.Archive), opt).findings(t)
		code := strings.SplitN(filepath.Base(c.Path), "_", 2)[0]
		if !strings.Contains(out, "["+code+"]") {
			t.Errorf("%s does not produce %s", c.Path, code)
		}
		return []byte(out)
	}, golden.Expected(findingsFile))
}
