package canon

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Rule X2: a panic inside the compiler is an *InternalError, and the project stays usable.
func TestPanicIsInternalError(t *testing.T) {
	m := map[string][]byte{
		"/law/project.canon": []byte("project a {\n  canon: \"0.1\"\n}\n"),
		"/law/x/x.canon":     []byte("package x\n"),
	}
	fsys := newMapFS(m)
	panics := func(context.Context, *project.Project, []*syntax.File, map[string]*diag.Bag) *check.Program {
		panic("checker bug")
	}
	b, err := build.Open(fsys, "/law", build.Options{Checker: panics})
	if err != nil {
		t.Fatal(err)
	}
	p := &Project{root: "/law", b: b}
	_, err = p.Check(context.Background())
	var ierr *InternalError
	if !errors.Is(err, ErrInternal) || !errors.As(err, &ierr) || ierr.Msg != "checker bug" || ierr.Stack == "" {
		t.Errorf("Check: %v", err)
	}
	if pkgs, err := p.Packages(context.Background()); err != nil || len(pkgs) != 1 {
		t.Errorf("Packages after the panic: %v, %v", pkgs, err)
	}
}

// mapFS is project.FS over absolute names held in memory.
type mapFS struct{ m fstest.MapFS }

func newMapFS(files map[string][]byte) mapFS {
	m := fstest.MapFS{}
	//canon:unordered each file is stored under its own name
	for name, data := range files {
		m[strings.TrimPrefix(name, "/")] = &fstest.MapFile{Data: data}
	}
	return mapFS{m}
}

func (f mapFS) ReadFile(name string) ([]byte, error) {
	return f.m.ReadFile(strings.TrimPrefix(name, "/"))
}
func (f mapFS) Stat(name string) (fs.FileInfo, error) { return f.m.Stat(strings.TrimPrefix(name, "/")) }
func (f mapFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return f.m.ReadDir(strings.TrimPrefix(name, "/"))
}
