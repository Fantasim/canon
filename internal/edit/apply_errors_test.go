package edit_test

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// API.md N3 (log-2026-09-29 M4 U4b-r): a path collision is an *edit.CollisionError naming the
// taken paths, which wraps ErrPathCollision and is no internal failure.
func TestCollisionErrorPaths(t *testing.T) {
	ar, err := txtar.ParseFile(filepath.Join("testdata", "edits", "refuse_collision.txtar"))
	if err != nil {
		t.Fatal(err)
	}
	c := readCase(t, ar)
	s := open(t, c.fsys, c.layers, c.editLayer, c.pkgs...)
	op := edit.Operation{Kind: edit.OpAddEntry, Path: "offers", Key: edit.Key("summer"), Value: edit.Source("{ price: 10 }")}
	_, err = edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	var ce *edit.CollisionError
	switch {
	case !errors.As(err, &ce):
		t.Fatalf("Apply = %v, want a *CollisionError", err)
	case !slices.Equal(ce.Paths, []string{"shop/offers/summer.canon"}):
		t.Errorf("Paths = %v, want [shop/offers/summer.canon]", ce.Paths)
	case !errors.Is(err, edit.ErrPathCollision) || errors.Is(err, edit.ErrInternal):
		t.Errorf("%v: want ErrPathCollision and no ErrInternal", err)
	}
}

// failFS is a file system whose reads of file fail with err once armed.
type failFS struct {
	mapFS
	file string
	err  error
}

func (f *failFS) ReadFile(name string) ([]byte, error) {
	if f.err != nil && name == f.file {
		return nil, f.err
	}
	return f.mapFS.ReadFile(name)
}

// API.md X2 (log-2026-09-29 M4 U4b-r3): a file system failure, reading a file Apply writes or
// analyzing the edit again, is returned as it is; any other failure, one inside the analysis
// included, is an internal compiler error.
func TestReadFailures(t *testing.T) {
	pathErr := &fs.PathError{Op: "read", Path: "p", Err: fs.ErrPermission}
	for _, c := range []struct {
		name, file string
		err        error
		internal   bool
	}{
		{"written file, fs error", "/law/p/p.canon", pathErr, false},
		{"analysis, fs error", "/law/p/q.canon", pathErr, false},
		{"analysis, fs sentinel", "/law/p/q.canon", fs.ErrClosed, false},
		{"written file, other error", "/law/p/p.canon", errors.New("disk says no"), true},
		{"analysis, other error", "/law/p/q.canon", errors.New("disk says no"), true},
	} {
		fsys := &failFS{mapFS: mapFS{
			"law/project.canon": file(projectCanon), "law/p/p.canon": file(regionSrc),
			"law/p/rows.json": file(regionJSON), "law/p/q.canon": file("package p\n\n/// Q.\nlet q: Int = 1\n"),
		}, file: c.file}
		p, err := build.Open(fsys, "/law", build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		a, err := p.Analyze(context.Background(), []string{"p"})
		if err != nil {
			t.Fatal(err)
		}
		fsys.err = c.err
		ops := []edit.Operation{{Kind: edit.OpSet, Path: "other", Value: edit.Int(9)}, {Kind: edit.OpSet, Path: "more", Value: edit.Int(7)}}
		_, err = edit.Apply(context.Background(), edit.Env{Project: p, Host: hostOf}, edit.NewSnapshot(a), edit.Request{Ops: ops})
		if !errors.Is(err, c.err) || errors.Is(err, edit.ErrInternal) != c.internal {
			t.Errorf("%s: Apply = %v, want %v, internal %v", c.name, err, c.err, c.internal)
		}
	}
}
