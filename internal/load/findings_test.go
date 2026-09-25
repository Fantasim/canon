package load_test

import (
	"bytes"
	"context"
	"io/fs"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
	"golang.org/x/tools/txtar"
)

// The findings cases place their tree under this project directory, a file named with a
// leading "/" outside it; symlinks lists "name target" lines, name becoming a symlink entry
// that only the archive's own tree resolves (meta/decisions/log-2026-09-24.md "load.dir round 3").
const (
	projectDir   = "/p"
	patternFile  = "pattern"
	symlinksFile = "symlinks"
	findingsFile = "findings.txt"
	maxLinkHops  = 40
)

// memFS is an archive's tree as a project.FS under projectDir, its control files aside; it
// resolves links itself, so no case depends on the machine's disk.
type memFS struct{ m fstest.MapFS }

func newMemFS(a *txtar.Archive) memFS {
	m := fstest.MapFS{}
	for _, f := range a.Files {
		if f.Name == patternFile || f.Name == symlinksFile {
			continue
		}
		m[inProject(f.Name)[1:]] = &fstest.MapFile{Data: f.Data}
	}
	if data, ok := archiveFile(a, symlinksFile); ok {
		addSymlinks(m, data)
	}
	return memFS{m}
}

// inProject is name under projectDir, or name itself when it starts with "/".
func inProject(name string) string {
	if path.IsAbs(name) {
		return name
	}
	return path.Join(projectDir, name)
}

// addSymlinks adds one fs.ModeSymlink entry per non-empty line of data, "name target".
func addSymlinks(m fstest.MapFS, data []byte) {
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		name, target, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		m[inProject(name)[1:]] = &fstest.MapFile{Data: []byte(target), Mode: fs.ModeSymlink}
	}
}

// EvalSymlinks is name with every link of the tree followed, as the OS FS resolves them: a
// relative target is relative to its link's directory; a missing result is fs.ErrNotExist.
func (m memFS) EvalSymlinks(name string) (string, error) {
	done, todo := "/", strings.Split(rel(name), "/")
	for hops := 0; len(todo) > 0; {
		next := path.Join(done, todo[0])
		todo = todo[1:]
		f := m.m[rel(next)]
		if f == nil || f.Mode&fs.ModeSymlink == 0 {
			done = next
			continue
		}
		if hops++; hops > maxLinkHops {
			return "", project.ErrSymlinkLoop
		}
		target := string(f.Data)
		if !path.IsAbs(target) {
			target = path.Join(done, target)
		}
		done, todo = "/", append(strings.Split(rel(target), "/"), todo...)
	}
	if _, err := m.m.Stat(rel(done)); err != nil {
		return "", err
	}
	return done, nil
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

// dirExpr is `load.dir(pattern)`, built directly since dirPattern reads it off the syntax tree (WIRE.md §6.5).
func dirExpr(pattern string) *syntax.LoadExpr {
	return &syntax.LoadExpr{
		Method: &syntax.Ident{Name: "dir"},
		Args:   []*syntax.Arg{{Value: &syntax.StringLit{Parts: []syntax.StringPart{{Text: pattern}}}}},
	}
}

// rowType is `table Row`: one String field, supported with no host (DECISIONS 173).
func rowType() *types.TableType {
	f := &types.Field{Name: "label", Type: types.StringType, Wire: "label", WirePath: []string{"label"}}
	return &types.TableType{Elem: &types.RecordType{Pkg: "p", Name: "Row", Fields: []*types.Field{f}}}
}

// TestFindings runs `load.dir(pattern)` of every case's tree, rendering what load reports (WIRE.md §6).
func TestFindings(t *testing.T) {
	golden.Run(t, "testdata/findings/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		set := &source.FileSet{}
		bag := diag.NewBag(set, "p")
		data, ok := archiveFile(c.Archive, patternFile)
		if !ok {
			t.Skip("no pattern file: a call-form case, TestFindingsCall's")
		}
		pattern := strings.TrimSuffix(string(data), "\n")
		src, err := set.Add(patternFile, path.Join(projectDir, patternFile), data)
		if err != nil {
			t.Fatal(err)
		}
		span := source.Span{File: src.ID, Start: 0, End: source.Pos(len(pattern))}
		layout, ok := project.NewLayout(&project.Project{}, projectDir, nil, bag)
		if !ok {
			t.Fatalf("%s: layout", c.Path)
		}
		l := &load.Loader{FS: newMemFS(c.Archive), Layout: layout, Set: set}
		req := load.Request{Pkg: "p", Span: span, Bag: bag}
		_, _, err = l.Load(context.Background(), req, dirExpr(pattern), rowType())
		if err != nil {
			t.Fatalf("%s: %v (a findings case reports through the bag, never a Go error)", c.Path, err)
		}
		var buf bytes.Buffer
		opt := diag.RenderOptions{Summary: bag.Summary(), Golden: true}
		if err := diag.Render(&buf, set, bag.Findings(), opt); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}, golden.Expected(findingsFile))
}
