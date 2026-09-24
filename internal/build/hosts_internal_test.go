package build

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
)

// roFS is a read-only project.FS over fstest.MapFS, absolute names ("/law/...").
type roFS fstest.MapFS

func roRel(name string) string {
	if name == "/" {
		return "."
	}
	return strings.TrimPrefix(name, "/")
}

func (m roFS) ReadFile(name string) ([]byte, error)  { return fstest.MapFS(m).ReadFile(roRel(name)) }
func (m roFS) Stat(name string) (fs.FileInfo, error) { return fstest.MapFS(m).Stat(roRel(name)) }
func (m roFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fstest.MapFS(m).ReadDir(roRel(name))
}

func srcFile(text string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(text)} }

// DECISIONS 195, 210: an internal error met by a fold is never silent; failure hands it to the
// build as ErrInternal, beside the evaluator's own (eval/folder.go FoldErr).
func TestFailureSurfacesFoldBugs(t *testing.T) {
	ctx := context.Background()
	fsys := roFS{
		"law/project.canon": srcFile("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     srcFile("package a\n\nconst ONE = 1\n"),
	}
	p, err := Open(fsys, "/law", Options{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.prepare(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.check(ctx); err != nil {
		t.Fatal(err)
	}
	r.stageA(ctx)
	owner := r.prog.Packages[0].Decls[0]
	buggy := eval.NewFolder(r.bags, r.opt)
	untyped := &syntax.IdentExpr{Name: "nowhere"} // a name the checker never resolved
	if _, ok := buggy.Fold(ctx, owner, untyped, r.prog.Info); ok {
		t.Fatal("an unresolved name folded")
	}
	r.host.fold = buggy
	if err := r.host.failure(r.s.set, r.prog); !errors.Is(err, ErrInternal) {
		t.Fatalf("failure() = %v, want ErrInternal", err)
	}
}
