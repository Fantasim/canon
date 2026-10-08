package std_test

import (
	"bytes"
	"context"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"golang.org/x/tools/txtar"
)

// evaluate checks the .canon files of an archive and forces every const and let, in package
// and declaration order; it returns the rendered findings.
func evaluate(t *testing.T, a *txtar.Archive) string {
	t.Helper()
	out, _ := evaluateAll(t, a)
	return out
}

// evaluateAll is evaluate, with the canonical text of each value forced, by pkg.name; an internal error fails.
func evaluateAll(t *testing.T, a *txtar.Archive) (string, map[string]string) {
	t.Helper()
	ctx := context.Background()
	fs := &source.FileSet{}
	parse := diag.NewBag(fs, "")
	var files []*syntax.File
	for _, f := range a.Files {
		if path.Ext(f.Name) != ".canon" {
			continue
		}
		src, err := fs.Add(f.Name, "/"+f.Name, f.Data)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, syntax.Parse(src, syntax.FileSource, parse))
	}
	bags := check.Bags{}
	proj := project.New("std", project.Version{Minor: 1})
	prog := check.Check(ctx, proj, files, bags, eval.NewFolder(bags, eval.Options{}))
	ev := eval.New(prog, nil, bags, eval.Options{})
	var all []diag.Finding
	values := map[string]string{}
	sum := parse.Summary()
	sum.Packages = 0
	for _, pkg := range prog.Packages {
		for _, obj := range pkg.Decls {
			if obj.Kind() != check.ObjLet && obj.Kind() != check.ObjConst {
				continue
			}
			if v, ok := ev.Force(ctx, eval.Root{Pkg: pkg.Path, Name: obj.Name()}); ok && v != nil {
				values[pkg.Path+"."+obj.Name()] = v.CanonText()
			}
		}
		all = append(all, bags[pkg.Path].Findings()...)
		sum = sum.Merge(bags[pkg.Path].Summary())
	}
	if err := ev.Err(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := diag.Render(&buf, fs, append(parse.Findings(), all...), diag.RenderOptions{Summary: sum, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.String(), values
}

// IMPLEMENTATION-PLAN §7.2: each case produces the code its name starts with.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		out := evaluate(t, c.Archive)
		if code := strings.SplitN(filepath.Base(c.Path), "_", 2)[0]; !strings.Contains(out, "["+code+"]") {
			t.Errorf("%s does not produce %s", c.Path, code)
		}
		return []byte(out)
	}, golden.Expected("findings.txt"))
}
