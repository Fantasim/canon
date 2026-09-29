package edit_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// noLoads serves an evaluator that computes defaults: a default never loads a file.
type noLoads struct{}

func (noLoads) Load(context.Context, *syntax.LoadExpr, types.Type) (value.Value, bool) {
	return nil, false
}

func (noLoads) Verify(context.Context, eval.Root, value.Value) bool { return true }

// evalHost is a wire.Host over an evaluator of the analysis's program, as workspace supplies
// one to Apply: defaults are computed, dereferences answered (API.md E6, V1).
type evalHost struct{ ev *eval.Evaluator }

// hostOf is the host of an analysis, as Env.Host gives it.
func hostOf(a *build.Analysis) wire.Host { return newEvalHost(a) }

func newEvalHost(a *build.Analysis) evalHost {
	prog := a.Program()
	bags := check.Bags{}
	for _, pkg := range prog.Packages {
		bags[pkg.Path] = diag.NewBag(a.Files(), pkg.Path)
	}
	return evalHost{ev: eval.New(prog, noLoads{}, bags, eval.Options{})}
}

func (h evalHost) Default(ctx context.Context, f *types.Field, in wire.Instance, via *value.Prov) (value.Value, bool) {
	return h.ev.Default(ctx, f, in.Record, in.Params, via)
}

func (h evalHost) Deref(ctx context.Context, r *value.Ref) (*value.Record, bool) {
	return h.ev.Deref(ctx, r)
}

func (h evalHost) Bind(rec *value.Record, p map[*types.Param]value.Value) { h.ev.Bind(rec, p) }

func (h evalHost) Cycle(ctx context.Context, r *value.Ref) { h.ev.Cycle(ctx, r) }

func (h evalHost) Reads(f *types.Field, fs []*types.Field) []int { return h.ev.Reads(f, fs) }

func (h evalHost) Savepoint() func(bool) { return h.ev.Savepoint() }

// session is a project analyzed once, and what Apply needs of it.
type session struct {
	p    *build.Project
	a    *build.Analysis
	snap *edit.Snapshot
	env  edit.Env
}

// open analyzes the project at /law of fsys, pkgs selected, layers active.
func open(t testing.TB, fsys mapFS, layers []string, editLayer string, pkgs ...string) session {
	t.Helper()
	p, err := build.Open(fsys, "/law", build.Options{Layers: layers})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), pkgs)
	if err != nil {
		t.Fatal(err)
	}
	return session{p: p, a: a, snap: edit.NewSnapshot(a), env: edit.Env{Project: p, EditLayer: editLayer, Host: hostOf}}
}
