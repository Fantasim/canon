package i18n_test

import (
	"bytes"
	"context"
	"maps"
	"path"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const canonExt = ".canon"

// fixtureProject is a small project with languages en, fr and a "studio" studio package, as the
// txtar fixtures assume.
func fixtureProject() *project.Project {
	p := project.New("fixture", project.Version{Major: 0, Minor: 1})
	p.Languages = []string{"en", "fr"}
	p.Studio = project.Package{Path: "studio"}
	return p
}

// checked is a txtar case, checked and i18n-checked.
type checked struct {
	fs   *source.FileSet
	bags check.Bags
	prog *check.Program
	res  map[string]*i18n.Result
}

// runCase parses c's .canon files, checks them and runs i18n.Check.
func runCase(t *testing.T, c golden.Case) *checked {
	t.Helper()
	files := map[string][]byte{}
	for _, f := range c.Archive.Files {
		if path.Ext(f.Name) == canonExt {
			files[f.Name] = f.Data
		}
	}
	return checkBytes(t, files)
}

// checkFiles checks the given sources (name -> content) and runs i18n.Check, for tests that
// assert on the catalogue or a Result directly rather than on rendered findings.
func checkFiles(t *testing.T, files map[string]string) *checked {
	t.Helper()
	bytes := map[string][]byte{}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		bytes[name] = []byte(files[name])
	}
	return checkBytes(t, bytes)
}

// checkBytes parses each file into its package's own bag (its directory, every fixture's
// convention), as the real pipeline shares parsing and checking bags: a fixture can then show
// I18N.md F4's unparsed branch, not only what check reports afterwards.
func checkBytes(t *testing.T, files map[string][]byte) *checked {
	t.Helper()
	fs := &source.FileSet{}
	bags := check.Bags{}
	names := slices.Sorted(maps.Keys(files))
	var parsed []*syntax.File
	for _, name := range names {
		src, err := fs.Add(name, "/"+name, files[name])
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, syntax.Parse(src, syntax.FileSource, packageBag(bags, fs, name)))
	}
	proj := fixtureProject()
	prog := check.Check(context.Background(), proj, parsed, bags, eval.NewFolder(bags, eval.Options{}))
	res := i18n.Check(prog, proj, bags, emitsView(prog))
	return &checked{fs: fs, bags: bags, prog: prog, res: res}
}

// packageBag is name's directory's bag in bags (every fixture's package), made on first use.
func packageBag(bags check.Bags, fs *source.FileSet, name string) *diag.Bag {
	pkg := path.Dir(name)
	if bags[pkg] == nil {
		bags[pkg] = diag.NewBag(fs, pkg)
	}
	return bags[pkg]
}

// emitsView is every loaded package (all of them "selected", here) that declares `emit view`
// (I18N.md W1; the one emitsView map views/rules.Check also takes, computed once by build, U8).
func emitsView(prog *check.Program) map[string]bool {
	out := map[string]bool{}
	for _, pkg := range prog.Packages {
		if pkgEmitsView(pkg) {
			out[pkg.Path] = true
		}
	}
	return out
}

// pkgEmitsView reports whether pkg declares `emit view`.
func pkgEmitsView(pkg *check.Package) bool {
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			if e, ok := d.(*syntax.EmitDecl); ok && e.Target != nil && e.Target.Name == check.TargetView {
				return true
			}
		}
	}
	return false
}

// render prints every package bag's findings, with their summary (as check's tests do).
func (c *checked) render(t *testing.T) string {
	t.Helper()
	var all []diag.Finding
	var sum diag.Summary
	names := slices.Sorted(maps.Keys(c.bags))
	for _, name := range names {
		all = append(all, c.bags[name].Findings()...)
		sum = sum.Merge(c.bags[name].Summary())
	}
	var buf bytes.Buffer
	if err := diag.Render(&buf, c.fs, all, diag.RenderOptions{Summary: sum, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
