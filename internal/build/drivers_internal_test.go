package build

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/project"
)

const (
	internalA = "package a\n\nenum Goal { kill, visit }\n\nrecord Kind {\n  goal: Goal\n}\n\n" +
		"let kinds: table Kind = {\n  k1 { goal: kill }\n}\n\n" +
		"type Target(k: Kind) = match k.goal {\n  kill => String\n  visit => Int\n}\n"
	internalB = "package b\n\nimport a\n\nrecord Step {\n  kind: ref a.kinds\n  target: a.Target(kind)?\n}\n"
)

// DECISIONS 195, VIEWMODEL.md 12.3, log-2026-09-29 "Drivers across the project": an internal
// error met in the drivers-only program fails the view model with ErrInternal.
func TestDriversInternalErrorSurfaces(t *testing.T) {
	ctx := context.Background()
	fsys := roFS{
		"law/project.canon": srcFile("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     srcFile(internalA),
		"law/b/b.canon":     srcFile(internalB),
	}
	p, err := Open(fsys, "/law", Options{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.prepare(ctx, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.analyze(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := r.viewModel(ctx, "a"); err != nil || r.wideReads == nil {
		t.Fatalf("before the bug: %v, drivers' program built %t", err, r.wideReads != nil)
	}
	r.wideReads.host.errs = append(r.wideReads.host.errs, internal(errNoProgram)) // a host bug
	if _, err := r.viewModel(ctx, "a"); !errors.Is(err, ErrInternal) || !errors.Is(err, errNoProgram) {
		t.Errorf("viewModel = %v, want ErrInternal wrapping the host's bug", err)
	}
}

// VIEWMODEL.md 12.3, log-2026-09-29 "Drivers review": the drivers-only program is built only
// when a type function needs drivers; studio and sovcommon.time declare none, resource.vocab does.
func TestDriversBuiltLazily(t *testing.T) {
	ctx := context.Background()
	dir, err := filepath.Abs(filepath.Join("..", "..", "examples"))
	if err != nil {
		t.Fatal(err)
	}
	roots := map[string]string{"resource": "_fixtures/resource", "client": "_fixtures/client"}
	p, err := Open(project.OS(), filepath.ToSlash(dir), Options{Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		pkg   string
		built bool
	}{{"studio", false}, {"sovcommon.time", false}, {"resource.vocab", true}} {
		r, err := p.prepare(ctx, []string{c.pkg})
		if err != nil {
			t.Fatal(err)
		}
		if err := r.analyze(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := r.viewModel(ctx, c.pkg); err != nil || (r.wideReads != nil) != c.built || r.ownReads != nil {
			t.Errorf("%s: %v, drivers-only program built %t, want %t", c.pkg, err, r.wideReads != nil, c.built)
		}
	}
}
