package lock_test

import (
	"bytes"
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/lock"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	findingsFile = "findings.txt"
	lockName     = "canon.lock"
)

// IMPLEMENTATION-PLAN.md §7.2: each findings case prints its lock's findings into findings.txt.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		var buf bytes.Buffer
		for _, f := range c.Archive.Files {
			if path.Base(f.Name) != lockName {
				continue
			}
			pkg := strings.ReplaceAll(path.Dir(f.Name), "/", ".")
			files := oneFile{path: f.Name, content: f.Data}
			bag := diag.NewBag(files, pkg)
			lock.Parse(1, f.Data, pkg, bag)
			opt := diag.RenderOptions{Summary: bag.Summary(), Golden: true}
			if err := diag.Render(&buf, files, bag.Findings(), opt); err != nil {
				t.Fatal(err)
			}
		}
		return buf.Bytes()
	}, golden.Expected(findingsFile))
}
