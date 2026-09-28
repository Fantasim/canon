package build_test

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// mapFS is a build's file system under "/": fstest.MapFS with absolute names.
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

func (m mapFS) WriteFile(name string, data []byte) error {
	m[rel(name)] = &fstest.MapFile{Data: data}
	return nil
}

func (m mapFS) Rename(oldname, newname string) error {
	f, ok := m[rel(oldname)]
	if !ok {
		return fs.ErrNotExist
	}
	m[rel(newname)] = f
	delete(m, rel(oldname))
	return nil
}

func (m mapFS) Remove(name string) error {
	delete(m, rel(name))
	return nil
}

func (m mapFS) MkdirAll(string) error { return nil }

func file(text string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(text)} }

// Open reads project.canon only; Check runs phases 1 to 7 on the selected packages and their
// imports, reporting an imported package's errors too (API.md O2, R2).
func Example() {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     file("package a\n\nimport b\n"),
		"law/b/b.canon":     file("package b\n\nconst = 1\n"),
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, sel := range [][]string{{"a"}, {"b"}} {
		res, err := p.Check(context.Background(), sel)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(res.Packages, len(res.List), res.Summary.Errors, res.Summary.Packages)
	}
	// Output:
	// [a] 1 1 1
	// [b] 1 1 1
}

// Analyze keeps the checked program and the evaluator that forced it (§4.6-§4.8).
func ExampleProject_Analyze() {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     file("/// X.\npackage a\n\n/// X.\nlet x: Int = 1\n"),
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	a, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(a.Program().Packages), a.Result().Summary.Errors)
	x, ok := a.Force(eval.Root{Pkg: "a", Name: "x"})
	fmt.Println(x.(*value.Int).V, ok)
	// Output:
	// 1 0
	// 1 true
}

// A Checker replaces phase 2 over the files and bags of the selection and its imports (§4.7).
func ExampleChecker() {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     file("package a\n\nimport b\n"),
		"law/b/b.canon":     file("package b\n"),
		"law/c/c.canon":     file("package c\n"),
	}
	var seen []string
	checker := func(ctx context.Context, proj *project.Project, files []*syntax.File, bags map[string]*diag.Bag) *check.Program {
		for _, f := range files {
			seen = append(seen, f.Src.Path)
		}
		fmt.Println(len(bags))
		return check.Check(ctx, proj, files, bags, eval.NewFolder(bags, eval.Options{}))
	}
	p, err := build.Open(fsys, "/law", build.Options{Checker: checker})
	if err != nil {
		fmt.Println(err)
		return
	}
	if _, err := p.Check(context.Background(), []string{"a"}); err != nil {
		fmt.Println(err)
	}
	fmt.Println(seen)
	// Output:
	// 2
	// [a/a.canon b/b.canon]
}

// Build writes the outputs and the locks of the selection, all or nothing (CLI.md §3.4).
func ExampleProject_Build() {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n  roots {\n    out: \"out\"\n  }\n}\n"),
		"law/a/a.canon": file("/// A.\npackage a\n\n/// Tier.\nrecord Tier {\n  /// Weight.\n  weight: Int = 1\n}\n\n" +
			"/// Tiers.\nlet tiers: stable table Tier = { low {}, high { weight: 2 } }\n\nemit json { out: \"@out/\" }\n"),
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	res, err := p.Build(context.Background(), build.BuildOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, o := range res.Outputs {
		fmt.Println(o.Path, o.Abs, o.Status == build.StatusWritten)
	}
	for _, l := range res.Locks {
		fmt.Printf("%s %q %t\n", l.Path, l.Lines, fsys["law/a/canon.lock"] != nil && fsys["law/out/tiers.json"] != nil)
	}
	// Output:
	// @out/tiers.json /law/out/tiers.json true
	// a/canon.lock ["table  a.tiers  high" "table  a.tiers  low"] true
}
