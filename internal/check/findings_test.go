package check_test

import (
	"bytes"
	"context"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	findingsFile = "findings.txt"
	noFindings   = "0 errors, 0 warnings"
)

// IMPLEMENTATION-PLAN.md §7.2.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		fs := &source.FileSet{}
		parse := diag.NewBag(fs, "")
		var files []*syntax.File
		for _, f := range c.Archive.Files {
			if path.Ext(f.Name) != canonExt {
				continue
			}
			src, err := fs.Add(f.Name, "/"+f.Name, f.Data)
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, syntax.Parse(src, syntax.FileSource, parse))
		}
		bags := check.Bags{}
		check.Check(context.Background(), exampleProject(), files, bags, literalFolder{})
		out := render(t, fs, parse, bags)
		code := strings.SplitN(filepath.Base(c.Path), "_", 2)[0]
		if !strings.Contains(out, "["+code+"]") {
			t.Errorf("%s does not produce %s", c.Path, code)
		}
		return []byte(out)
	}, golden.Expected(findingsFile))
}

// TYPES.md: each accepted program checks with no finding, and its Info is complete.
func TestAccepted(t *testing.T) {
	cases, err := golden.Load("testdata/accept/*.txtar", golden.Expected(findingsFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(filepath.Base(c.Path), func(t *testing.T) {
			fs := &source.FileSet{}
			parse := diag.NewBag(fs, "")
			var files []*syntax.File
			for _, f := range c.Archive.Files {
				src, err := fs.Add(f.Name, "/"+f.Name, f.Data)
				if err != nil {
					t.Fatal(err)
				}
				files = append(files, syntax.Parse(src, syntax.FileSource, parse))
			}
			bags := check.Bags{}
			prog := check.Check(context.Background(), exampleProject(), files, bags, literalFolder{})
			if out := render(t, fs, parse, bags); !strings.HasPrefix(out, noFindings) {
				t.Errorf("findings:\n%s", out)
			}
			for _, f := range files {
				for _, g := range gaps(f, prog.Info, func(syntax.Decl) bool { return false }) {
					t.Error(g)
				}
			}
		})
	}
}

// render prints the findings of the parse bag and of every package bag, with their summary.
func render(t *testing.T, fs *source.FileSet, parse *diag.Bag, bags check.Bags) string {
	t.Helper()
	all := parse.Findings()
	sum := parse.Summary()
	sum.Packages = 0
	names := make([]string, 0, len(bags))
	for name := range bags {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		all = append(all, bags[name].Findings()...)
		sum = sum.Merge(bags[name].Summary())
	}
	var buf bytes.Buffer
	if err := diag.Render(&buf, fs, all, diag.RenderOptions{Summary: sum, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
