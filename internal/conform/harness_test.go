package conform_test

import (
	"bytes"
	"context"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const projectFile = "project.canon"

// tsSafe is the largest integer TypeScript holds exactly (CONFORMANCE.md §4).
const tsSafe = 1<<53 - 1

// world is a checked program, its stage-A evaluator and its stage-E IR: the real producers of
// what Fill reads, before build wires them.
type world struct {
	fs    *source.FileSet
	files []*syntax.File
	parse *diag.Bag
	proj  *project.Project
	bags  check.Bags
	prog  *check.Program
	ev    *eval.Evaluator
	pkgs  []*ir.Package
}

// newWorld parses files (name, content pairs) under the examples' project and runs stage E.
func newWorld(t *testing.T, files ...string) *world {
	t.Helper()
	w := &world{fs: &source.FileSet{}, bags: check.Bags{}}
	w.parse = diag.NewBag(w.fs, "")
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", projectFile))
	if err != nil {
		t.Fatal(err)
	}
	src, err := w.fs.Add(projectFile, "/"+projectFile, data)
	if err != nil {
		t.Fatal(err)
	}
	if w.proj, err = project.Load(src, w.parse); err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(files); i += 2 {
		src, err := w.fs.Add(files[i], "/"+files[i], []byte(files[i+1]))
		if err != nil {
			t.Fatal(err)
		}
		w.files = append(w.files, syntax.Parse(src, syntax.FileSource, w.parse))
	}
	ctx := context.Background()
	w.prog = check.Check(ctx, w.proj, w.files, w.bags, eval.NewFolder(w.bags, eval.Options{}))
	w.ev = eval.New(w.prog, served{}, w.bags, eval.Options{})
	w.pkgs = ir.Build(ctx, ir.Input{Program: w.prog, Project: w.proj, Bags: w.bags, Host: stageE{w.ev}, Fold: eval.NewFolder(w.bags, eval.Options{})})
	return w
}

// value is the forced top-level value name of pkg.
func (w *world) value(t *testing.T, pkg, name string) value.Value {
	t.Helper()
	v, ok := w.ev.Force(context.Background(), eval.Root{Pkg: pkg, Name: name})
	if !ok {
		t.Fatalf("%s.%s is poisoned", pkg, name)
	}
	return v
}

// fn is the translated IR fn and its declared object: owner is the record, "" for a package fn.
func (w *world) fn(t *testing.T, pkg, owner, name string) (*ir.ExportFn, check.Object) {
	t.Helper()
	for _, p := range w.pkgs {
		for _, f := range w.fnsOf(p, owner) {
			if f.Name == name {
				return f, w.object(t, pkg, owner, name)
			}
		}
	}
	t.Fatalf("no fn %s %s.%s in the IR", pkg, owner, name)
	return nil, nil
}

func (w *world) fnsOf(p *ir.Package, owner string) []*ir.ExportFn {
	if owner == "" {
		return p.Fns
	}
	for _, typ := range p.Types {
		if r, ok := typ.(*ir.Record); ok && r.Name == owner {
			return r.Methods
		}
	}
	return nil
}

// object is the declared object of a fn: the definition of its declaration's name, found by
// package, owner ("" at top level) and name in source order.
func (w *world) object(t *testing.T, pkg, owner, name string) check.Object {
	t.Helper()
	for _, d := range w.decls(pkg) {
		if fn := fnDecl(d, owner, name); fn != nil {
			return w.prog.Info.Defs[fn.Name]
		}
	}
	t.Fatalf("no object %s %s.%s", pkg, owner, name)
	return nil
}

// decls is the top-level declarations of package pkg, file by file.
func (w *world) decls(pkg string) []syntax.Decl {
	var out []syntax.Decl
	for _, cp := range w.prog.Packages {
		if cp.Path != pkg {
			continue
		}
		for _, f := range cp.Files {
			out = append(out, f.Decls...)
		}
	}
	return out
}

// fnDecl is the fn named name that d declares under owner, nil when it declares none.
func fnDecl(d syntax.Decl, owner, name string) *syntax.FnDecl {
	switch x := d.(type) {
	case *syntax.FnDecl:
		if owner == "" && x.Name != nil && x.Name.Name == name {
			return x
		}
	case *syntax.RecordDecl:
		if x.Name == nil || x.Name.Name != owner || x.Body == nil {
			return nil
		}
		for _, it := range x.Body.Items {
			if fn, ok := it.(*syntax.FnDecl); ok && fn.Name != nil && fn.Name.Name == name {
				return fn
			}
		}
	}
	return nil
}

// findings renders the parse findings, then every package's, with their summary.
func (w *world) findings(t *testing.T) string {
	t.Helper()
	all := w.parse.Findings()
	sum := w.parse.Summary()
	sum.Packages = 0
	for _, name := range slices.Sorted(maps.Keys(w.bags)) {
		all = append(all, w.bags[name].Findings()...)
		sum = sum.Merge(w.bags[name].Summary())
	}
	var buf bytes.Buffer
	if err := diag.Render(&buf, w.fs, all, diag.RenderOptions{Summary: sum, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// served is a host that serves no file and finds every value valid.
type served struct{}

func (served) Load(context.Context, *syntax.LoadExpr, types.Type) (value.Value, bool) {
	return nil, false
}

func (served) Verify(context.Context, eval.Root, value.Value) bool { return true }

// stageE is the evaluator as stage E's host, as build adapts it.
type stageE struct {
	ev *eval.Evaluator
}

func (h stageE) Value(ctx context.Context, pkg, name string) (value.Value, bool) {
	return h.ev.Force(ctx, eval.Root{Pkg: pkg, Name: name})
}

func (h stageE) Call(ctx context.Context, fn check.Object, recv value.Value, args []value.Value) (value.Value, bool) {
	return h.ev.Call(ctx, fn, recv, args)
}

// reference is conform.Evaluator over today's eval, for tests: eval records no test calls yet,
// so they are given; each vector runs in a fresh evaluator whose budget is the vector's cap,
// its findings captured in one bag; TS mode checks only the integer inputs the bodies here need.
type reference struct {
	w     *world
	calls []conform.Call
}

func (r *reference) TestCalls(_ context.Context, pkg string, fns []check.Object) []conform.Call {
	var out []conform.Call
	for _, c := range r.calls {
		if c.Fn.Pkg() == pkg && slices.Contains(fns, c.Fn) {
			out = append(out, c)
		}
	}
	return out
}

func (r *reference) Evaluate(ctx context.Context, c conform.Call, m conform.Mode) conform.Outcome {
	if m.TS && !safeInputs(c) {
		return conform.Outcome{Code: diag.E8303.Def().Code}
	}
	capture := diag.NewBag(r.w.fs, c.Fn.Pkg())
	bags := check.Bags{}
	for _, p := range r.w.prog.Packages {
		bags[p.Path] = capture
	}
	ev := eval.New(r.w.prog, served{}, bags, eval.Options{Budget: m.Steps})
	v, _ := ev.Call(ctx, c.Fn, c.Recv, c.Args)
	// diag.Bag yields F2 (position) order only; for the bodies here the first by position is the
	// first reported: their entry checks sit on the parameters, before the hard error ending the call.
	for _, f := range capture.Findings() {
		switch {
		case f.Severity != diag.Error:
		case f.Code == diag.E4401.Def().Code:
			return conform.Outcome{Exceeded: conform.StepLimit}
		case f.Code == diag.E4402.Def().Code:
			return conform.Outcome{Exceeded: conform.DepthLimit}
		default:
			return conform.Outcome{Code: f.Code}
		}
	}
	return conform.Outcome{Value: v}
}

// safeInputs reports integer arguments and reads within TypeScript's safe range.
func safeInputs(c conform.Call) bool {
	vs := slices.Clone(c.Args)
	if rec, ok := c.Recv.(*value.Record); ok {
		vs = append(vs, rec.Fields...)
	}
	for _, v := range vs {
		if n, ok := v.(*value.Int); ok && (n.V > tsSafe || n.V < -tsSafe) {
			return false
		}
	}
	return true
}

// integer is an Int value, as candidates hold it.
func integer(n int64) *value.Int {
	return &value.Int{V: n, T: types.IntType}
}

// The integer limits the tables of CONFORMANCE.md §6.6 write INT64_MIN and INT64_MAX.
const (
	intMin = math.MinInt64
	intMax = math.MaxInt64
)
