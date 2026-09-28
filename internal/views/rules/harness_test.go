package rules_test

import (
	"bytes"
	"context"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/views/rules"
	"golang.org/x/tools/txtar"
)

const (
	findingsFile = "findings.txt"
	projectFile  = "project.canon"
	studioPkg    = "studio"
	lawDir       = "/law"
	unitsLet     = "units"
)

// mapFS is an in-memory project tree whose paths are absolute.
type mapFS fstest.MapFS

func rel(name string) string {
	if name == "/" {
		return "."
	}
	return strings.TrimPrefix(name, "/")
}

func (m mapFS) ReadFile(name string) ([]byte, error)       { return fstest.MapFS(m).ReadFile(rel(name)) }
func (m mapFS) Stat(name string) (fs.FileInfo, error)      { return fstest.MapFS(m).Stat(rel(name)) }
func (m mapFS) ReadDir(name string) ([]fs.DirEntry, error) { return fstest.MapFS(m).ReadDir(rel(name)) }

// archiveFS is a txtar case's project under /law, but its golden file; a project.canon is
// added when the case has none, naming the studio package when the case holds one.
func archiveFS(a *txtar.Archive) mapFS {
	fsys := mapFS{}
	studio := ""
	for _, f := range a.Files {
		if f.Name == findingsFile {
			continue
		}
		fsys[rel(lawDir)+"/"+f.Name] = &fstest.MapFile{Data: f.Data}
		if strings.HasPrefix(f.Name, studioPkg+"/") {
			studio = studioPkg
		}
	}
	if _, ok := fsys[rel(lawDir)+"/"+projectFile]; !ok {
		text := "project demo {\n  canon: \"0.1\"\n"
		if studio != "" {
			text += "  studio: " + studio + "\n"
		}
		fsys[rel(lawDir)+"/"+projectFile] = &fstest.MapFile{Data: []byte(text + "}\n")}
	}
	return fsys
}

// analyzed is a project after phases 1 to 7, then its view checks (DECISIONS 221).
type analyzed struct {
	a    *build.Analysis
	bags check.Bags
}

// analyze runs phases 1 to 7, then the view checks with project.studio's units (EVALUATION.md §1).
func analyze(t *testing.T, fsys project.FS, dir string, opt build.Options) *analyzed {
	t.Helper()
	studio := studioOf(t, fsys, dir)
	ctx := context.Background()
	p, err := build.Open(fsys, dir, opt)
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	bags := check.Bags{}
	for _, cp := range a.Program().Packages {
		if b := a.Bag(cp.Path); b != nil {
			bags[cp.Path] = b
		}
	}
	rules.Check(ctx, a.Program(), bags, studio, emitsView(a.Program()))
	units, _ := a.Force(eval.Root{Pkg: studio, Name: unitsLet})
	rules.CheckUnits(ctx, a.Program(), bags, studio, units)
	return &analyzed{a: a, bags: bags}
}

// findings are every package's findings, packages in path order, and their summary.
func (x *analyzed) findings() ([]diag.Finding, diag.Summary) {
	names := make([]string, 0, len(x.bags))
	//canon:unordered the names are sorted before any output
	for name := range x.bags {
		names = append(names, name)
	}
	slices.Sort(names)
	var all []diag.Finding
	var sum diag.Summary
	for _, name := range names {
		all = append(all, x.bags[name].Findings()...)
		sum = sum.Merge(x.bags[name].Summary())
	}
	return all, sum
}

// render is the findings in the golden text form.
func (x *analyzed) render(t *testing.T) string {
	t.Helper()
	all, sum := x.findings()
	var buf bytes.Buffer
	if err := diag.Render(&buf, x.a.Files(), all, diag.RenderOptions{Summary: sum, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// emitsView are the packages declaring `emit view`, as build finds them (VIEWMODEL.md N4).
func emitsView(prog *check.Program) map[string]bool {
	out := map[string]bool{}
	for _, p := range prog.Packages {
		for _, f := range p.Files {
			out[p.Path] = out[p.Path] || slices.ContainsFunc(f.Decls, emitsViewDecl)
		}
	}
	return out
}

func emitsViewDecl(d syntax.Decl) bool {
	e, ok := d.(*syntax.EmitDecl)
	return ok && e.Target != nil && e.Target.Name == check.TargetView
}

// studioOf is project.studio of the project in dir, "" for none.
func studioOf(t *testing.T, fsys project.FS, dir string) string {
	t.Helper()
	name := dir + "/" + projectFile
	data, err := fsys.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	set := &source.FileSet{}
	src, err := set.Add(projectFile, name, data)
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(src, diag.NewBag(set, ""))
	if err != nil {
		t.Fatal(err)
	}
	return p.Studio.Path
}
