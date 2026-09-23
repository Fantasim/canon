package project_test

import (
	"bytes"
	"context"
	"io/fs"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"golang.org/x/tools/txtar"
)

const (
	projectDir   = "/p"
	findingsFile = "findings.txt"
	findFile     = "find"  // a directory to search from (E1003)
	rootsFile    = "roots" // `name=dir` lines: the --root overrides (E7003)
	pathsFile    = "paths" // `from written` lines: paths to resolve (E7001, E7003)
)

// memFS is an archive as a project.FS under /p: fstest.MapFS with absolute names.
type memFS struct{ m fstest.MapFS }

func newMemFS(a *txtar.Archive) memFS {
	m := fstest.MapFS{}
	for _, f := range a.Files {
		m[path.Join(projectDir[1:], f.Name)] = &fstest.MapFile{Data: f.Data}
	}
	return memFS{m}
}

func rel(name string) string {
	if name == "/" {
		return "."
	}
	return strings.TrimPrefix(name, "/")
}

func (m memFS) ReadFile(name string) ([]byte, error)       { return m.m.ReadFile(rel(name)) }
func (m memFS) Stat(name string) (fs.FileInfo, error)      { return m.m.Stat(rel(name)) }
func (m memFS) ReadDir(name string) ([]fs.DirEntry, error) { return m.m.ReadDir(rel(name)) }

func archiveFile(a *txtar.Archive, name string) ([]byte, bool) {
	for _, f := range a.Files {
		if f.Name == name {
			return f.Data, true
		}
	}
	return nil, false
}

// IMPLEMENTATION-PLAN.md §7.2: each case prints the findings of project.canon, paths or a search.
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		set := &source.FileSet{}
		bag := diag.NewBag(set, "")
		fsys := newMemFS(c.Archive)
		if dir, ok := archiveFile(c.Archive, findFile); ok {
			_, _ = project.Find(fsys, path.Join(projectDir, strings.TrimSpace(string(dir))), bag)
		} else {
			loadCase(t, c.Archive, fsys, set, bag)
		}
		var buf bytes.Buffer
		opt := diag.RenderOptions{Summary: bag.Summary(), Golden: true}
		if err := diag.Render(&buf, set, bag.Findings(), opt); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}, golden.Expected(findingsFile))
}

func loadCase(t *testing.T, a *txtar.Archive, fsys memFS, set *source.FileSet, bag *diag.Bag) {
	t.Helper()
	data, _ := archiveFile(a, project.FileName)
	src, err := set.Add(project.FileName, path.Join(projectDir, project.FileName), data)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := project.Load(src, bag)
	if p == nil {
		return
	}
	layout, _ := project.NewLayout(p, projectDir, overrides(a), bag)
	resolvePaths(t, a, layout, set, bag)
	names, err := project.Scan(fsys, projectDir)
	if err != nil {
		t.Fatal(err)
	}
	r := &project.Reader{FS: fsys, Dir: projectDir, Set: set, BagOf: func(string) *diag.Bag { return bag }}
	units, err := r.Parse(context.Background(), names)
	if err != nil {
		t.Fatal(err)
	}
	project.CheckStudio(p, units, bag)
}

func overrides(a *txtar.Archive) map[string]string {
	data, ok := archiveFile(a, rootsFile)
	if !ok {
		return nil
	}
	out := map[string]string{}
	for _, line := range strings.Fields(string(data)) {
		name, dir, _ := strings.Cut(line, "=")
		out[name] = dir
	}
	return out
}

// resolvePaths resolves each `from written` line of the paths file, at the span of written.
func resolvePaths(t *testing.T, a *txtar.Archive, l *project.Layout, set *source.FileSet, bag *diag.Bag) {
	t.Helper()
	data, ok := archiveFile(a, pathsFile)
	if !ok {
		return
	}
	src, err := set.Add(pathsFile, path.Join(projectDir, pathsFile), data)
	if err != nil {
		t.Fatal(err)
	}
	start := 0
	for _, line := range strings.SplitAfter(string(data), "\n") {
		from, written, _ := strings.Cut(strings.TrimSuffix(line, "\n"), " ")
		at := start + len(from) + 1
		span := source.Span{File: src.ID, Start: source.Pos(at), End: source.Pos(at + len(written))}
		if from == "." {
			from = ""
		}
		if line != "" {
			l.Resolve(written, from, span, bag)
		}
		start += len(line)
	}
}
