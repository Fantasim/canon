package syntax_test

import (
	"bytes"
	"path"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const findingsFile = "findings.txt"

// IMPLEMENTATION-PLAN.md §7.2: each case prints the findings of its .canon files.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		fs := &source.FileSet{}
		bag := diag.NewBag(fs, "p")
		for _, f := range c.Archive.Files {
			if path.Ext(f.Name) != ".canon" {
				continue
			}
			src, err := fs.Add(f.Name, "/"+f.Name, f.Data)
			if err != nil {
				t.Fatal(err)
			}
			kind := syntax.FileSource
			if path.Base(f.Name) == projectFile {
				kind = syntax.FileProject
			}
			tree := syntax.Parse(src, kind, bag)
			checkTree(t, f.Name, tree, false)
			if got := printTokens(tree); !bytes.Equal(got, src.Content) {
				t.Errorf("%s: tokens and trivia do not reproduce the file", f.Name)
			}
		}
		return []byte(renderFindings(t, bag, fs))
	}, golden.Expected(findingsFile))
}
