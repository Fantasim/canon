package jsonsrc_test

import (
	"bytes"
	"errors"
	"path"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// IMPLEMENTATION-PLAN.md §7.2: each case prints the findings of reading its .json files.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		fs := &source.FileSet{}
		bag := diag.NewBag(fs, "p")
		for _, f := range c.Archive.Files {
			if path.Ext(f.Name) != ".json" {
				continue
			}
			src, err := fs.Add(f.Name, "/"+f.Name, f.Data)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := jsonsrc.Parse(src, bag); errors.Is(err, jsonsrc.ErrEncoding) {
				t.Fatalf("%s: %v", f.Name, err)
			}
		}
		var buf bytes.Buffer
		opt := diag.RenderOptions{Summary: bag.Summary(), Golden: true}
		if err := diag.Render(&buf, fs, bag.Findings(), opt); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}, golden.Expected("findings.txt"))
}
