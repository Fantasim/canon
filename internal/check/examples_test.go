package check_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	examplesDir = "../../examples"
	projectFile = "project.canon"
	canonExt    = ".canon"
)

// exampleRoots are the roots examples/project.canon declares, for the asset paths.
var exampleRoots = []string{"client", "features", "generated", "parity", "pipeline_go", "resource", "services", "source", "sovcommon", "web"}

// loaded is a set of example files, parsed.
type loaded struct {
	fs    *source.FileSet
	files []*syntax.File
	parse *diag.Bag
}

// loadExamples parses every .canon file under examples/ whose directory is under one of dirs
// (all of them when dirs is empty), in path order.
func loadExamples(t *testing.T, dirs ...string) *loaded {
	t.Helper()
	l := &loaded{fs: &source.FileSet{}}
	l.parse = diag.NewBag(l.fs, "")
	err := filepath.WalkDir(examplesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != canonExt || d.Name() == projectFile {
			return err
		}
		rel, err := filepath.Rel(examplesDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if len(dirs) > 0 && !slices.ContainsFunc(dirs, func(d string) bool { return strings.HasPrefix(rel, d+"/") }) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src, err := l.fs.Add(rel, "/"+rel, data)
		if err != nil {
			return err
		}
		l.files = append(l.files, syntax.Parse(src, syntax.FileSource, l.parse))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// exampleProject is the project of examples/project.canon, as far as check reads it.
func exampleProject() *project.Project {
	p := project.New("sovereign", project.Version{Major: 0, Minor: 1})
	for _, r := range exampleRoots {
		p.Roots = append(p.Roots, project.Root{Name: r, Path: r})
	}
	return p
}

// run checks the loaded files and returns the program and the rendered findings of every
// package, in path order.
func (l *loaded) run(t *testing.T) (*check.Program, string) {
	t.Helper()
	bags := check.Bags{}
	prog := check.Check(context.Background(), exampleProject(), l.files, bags, literalFolder{})
	var all []diag.Finding
	var sum diag.Summary
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
	if err := diag.Render(&buf, l.fs, all, diag.RenderOptions{Summary: sum, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return prog, buf.String()
}

// M1 acceptance: teamboard, sovcommon/ui and sovcommon/roles check with no finding.
func TestTeamboardHasNoFinding(t *testing.T) {
	l := loadExamples(t, "teamboard", "sovcommon/ui", "sovcommon/roles")
	if got := l.parse.Findings(); len(got) != 0 {
		t.Fatalf("parse findings: %v", got)
	}
	_, out := l.run(t)
	if !strings.HasPrefix(out, "0 errors, 0 warnings") {
		t.Errorf("findings:\n%s", out)
	}
}

// Every example checks with no finding, and Info covers every declaration but the views, which
// the checker does not type yet (meta/state.md).
func TestExamplesCheck(t *testing.T) {
	l := loadExamples(t)
	prog, out := l.run(t)
	if !strings.HasPrefix(out, noFindings) {
		t.Errorf("findings:\n%s", out)
	}
	for _, f := range l.files {
		for _, g := range gaps(f, prog.Info, isView) {
			t.Error(g)
		}
	}
}

func isView(d syntax.Decl) bool {
	_, ok := d.(*syntax.ViewDecl)
	return ok
}
