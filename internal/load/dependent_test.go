package load_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// fakeHost is wire.Host as the evaluator backs it in a build (DECISIONS 173): each default
// served by field name, each dereference by key, every call counted.
type fakeHost struct {
	defaults map[string]value.Value
	entries  map[string]*value.Record
	calls    int
}

func (h *fakeHost) Default(_ context.Context, f *types.Field, _ wire.Instance, _ *value.Prov) (value.Value, bool) {
	h.calls++
	v, ok := h.defaults[f.Name]
	return v, ok
}

func (h *fakeHost) Deref(_ context.Context, r *value.Ref) (*value.Record, bool) {
	h.calls++
	rec, ok := h.entries[r.Key.S]
	return rec, ok
}

func (h *fakeHost) Bind(*value.Record, map[*types.Param]value.Value) {}

func (h *fakeHost) Cycle(context.Context, *value.Ref) {}

func (h *fakeHost) Reads(*types.Field, []*types.Field) []int { return nil }

func (h *fakeHost) Savepoint() func(bool) { return func(bool) {} }

func bareExpr(path string) *syntax.LoadExpr {
	return &syntax.LoadExpr{Args: []*syntax.Arg{{Value: &syntax.StringLit{Parts: []syntax.StringPart{{Text: path}}}}}}
}

func csvExpr(path string) *syntax.LoadExpr {
	return &syntax.LoadExpr{Method: &syntax.Ident{Name: "csv"}, Args: []*syntax.Arg{{Value: &syntax.StringLit{Parts: []syntax.StringPart{{Text: path}}}}}}
}

// dependentTask is heistia's Task.filterParam shape: `record Task { ev: ref evs, p: P(ev) }`
// with `type P(e: Ev) = match e.k { num => Int, text => String }`, and the entry `a` of evs,
// whose k is num.
func dependentTask() (*types.RecordType, map[string]*value.Record) {
	k := &types.EnumType{Pkg: "p", Name: "K", Members: []*types.Member{{Name: "num"}, {Name: "text"}}}
	kField := &types.Field{Name: "k", Type: k, Wire: "k", WirePath: []string{"k"}}
	ev := &types.RecordType{Pkg: "p", Name: "Ev", Fields: []*types.Field{kField}}
	evs := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "evs", Elem: ev}
	param := &types.Param{Name: "e", Type: ev}
	fn := &types.TypeFunc{Pkg: "p", Name: "P", Params: []*types.Param{param},
		Scrutinee: &types.Scrutinee{Param: param, Path: []*types.Field{kField}, Type: k},
		Arms:      []*types.TypeArm{{Members: []int{0}, Result: types.IntType}, {Members: []int{1}, Result: types.StringType}},
	}
	evField := &types.Field{Name: "ev", Type: &types.RefType{Target: evs}, Wire: "ev", WirePath: []string{"ev"}}
	dep := &types.TypeAppType{Fn: fn, Args: []*types.Arg{{Source: types.ArgField, Path: []*types.Field{evField}}}}
	pField := &types.Field{Name: "p", Index: 1, Type: dep, Wire: "p", WirePath: []string{"p"}, DependsOn: []int{0}}
	task := &types.RecordType{Pkg: "p", Name: "Task", Fields: []*types.Field{evField, pField}}
	entry := &value.Record{T: ev, Fields: []value.Value{&value.Member{Enum: k, Index: 0}}}
	return task, map[string]*value.Record{"a": entry}
}

// WIRE.md §5.9, DECISIONS 175: a dependent field decodes as its branch, selected through Host.Deref.
func TestLoadDependentDecodes(t *testing.T) {
	task, entries := dependentTask()
	cases := []struct {
		name  string
		files map[string]string
		expr  *syntax.LoadExpr
	}{
		{"bare load", map[string]string{"data/a.json": `[{"ev": "a", "p": 3}]`}, bareExpr("data/a.json")},
		{"load.dir", map[string]string{"data/a.json": `{"ev": "a", "p": 3}`}, dirExpr("data/*.json")},
	}
	for _, c := range cases {
		l, req := loaderFor(t, c.files)
		host := &fakeHost{entries: entries}
		req.Host = host
		v, ok, err := l.Load(context.Background(), req, c.expr, &types.ListType{Elem: task})
		if err != nil || !ok {
			t.Fatalf("%s: ok=%v err=%v findings=%+v", c.name, ok, err, req.Bag.Findings())
		}
		if got := v.CanonText(); got != "[Task{ev: a, p: 3}]" || host.calls != 1 {
			t.Errorf("%s: value = %s after %d host calls, want [Task{ev: a, p: 3}] after one Deref", c.name, got, host.calls)
		}
	}
}

// DECISIONS 173: a failed dereference is the host's to report; the decode is not ok, no Go error.
func TestLoadDependentDerefFails(t *testing.T) {
	task, _ := dependentTask()
	l, req := loaderFor(t, map[string]string{"data/a.json": `{"ev": "zz", "p": 3}`})
	req.Host = &fakeHost{}
	if _, ok, err := l.Load(context.Background(), req, bareExpr("data/a.json"), task); err != nil || ok {
		t.Errorf("ok=%v err=%v, want a poisoned value", ok, err)
	}
}

// WIRE.md §5.4, DECISIONS 173: an absent field with a default takes the host's value, in every form.
func TestLoadDefaultThroughHost(t *testing.T) {
	rec := enumDefaultFixture()
	kind := rec.Fields[0].Type.(*types.EnumType)
	kind.Members = []*types.Member{{Name: "other"}, {Name: "local_budget"}}
	member := &value.Member{Enum: kind, Index: 1}
	cases := map[string]struct {
		files map[string]string
		expr  *syntax.LoadExpr
	}{
		"bare load": {map[string]string{"data/a.json": `[{}]`}, bareExpr("data/a.json")},
		"load.dir":  {map[string]string{"data/a.json": `{}`}, dirExpr("data/*.json")},
		"load.csv":  {map[string]string{"data/a.csv": "mode\n\n"}, withHeader(csvExpr("data/a.csv"))},
	}
	for name, c := range cases {
		l, req := loaderFor(t, c.files)
		req.Host = &fakeHost{defaults: map[string]value.Value{"mode": member}}
		v, ok, err := l.Load(context.Background(), req, c.expr, &types.ListType{Elem: rec})
		if err != nil || !ok {
			t.Fatalf("%s: ok=%v err=%v findings=%+v", name, ok, err, req.Bag.Findings())
		}
		if got := v.CanonText(); got != "[R{mode: local_budget}]" {
			t.Errorf("%s: value = %s", name, got)
		}
	}
}

// DECISIONS 173: a default met without a host is the caller's misuse (wire.ErrNoHost), never a
// finding; build always gives one.
func TestLoadDefaultWithoutHost(t *testing.T) {
	l, req := loaderFor(t, map[string]string{"a.json": `{}`})
	if _, _, err := l.Load(context.Background(), req, bareExpr("a.json"), enumDefaultFixture()); !errors.Is(err, wire.ErrNoHost) {
		t.Errorf("err = %v, want wire.ErrNoHost", err)
	}
}

// withHeader is e with `header: true`.
func withHeader(e *syntax.LoadExpr) *syntax.LoadExpr {
	e.Args = append(e.Args, headerOpt(true))
	return e
}
