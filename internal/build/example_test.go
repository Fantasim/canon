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
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
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

func file(text string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(text)} }

// Open reads project.canon only; Check parses the selected packages and their imports and
// reports the findings of the selected ones (API.md O2, R2).
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
	// [a] 0 0 1
	// [b] 1 1 1
}

// A Checker sees the files and bags of the selected packages and their imports (IMPLEMENTATION-PLAN.md §4.7).
func ExampleChecker() {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     file("package a\n\nimport b\n"),
		"law/b/b.canon":     file("package b\n"),
		"law/c/c.canon":     file("package c\n"),
	}
	var seen []string
	checker := func(_ context.Context, _ *project.Project, files []*syntax.File, bags map[string]*diag.Bag) *check.Program {
		for _, f := range files {
			seen = append(seen, f.Src.Path)
		}
		fmt.Println(len(bags))
		return nil
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
