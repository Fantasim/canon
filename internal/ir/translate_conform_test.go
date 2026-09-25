package ir_test

import (
	"context"
	"maps"
	"math"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// potion is CONFORMANCE.md §6.6's Potion, and a method reading a precomputed method of self.
const potion = `/// Potions.
package pipeline

/// A healing potion.
record Potion {
  /// Its name.
  name: String
  /// Hit points restored.
  heal: Int(1..=100_000)

  /// A tenth of heal.
  export fn tenth(self) -> Int { return heal / 10 }

  /// What a player missing missingHp gets back.
  export fn healFor(self, missingHp: Int) -> Int { return min(heal, max(missingHp, 0)) }

  /// The damage a thrown potion does at level.
  export fn damageAt(self, level: Int(1..=150)) -> Int { return heal * level / 100 }

  /// A tenth of heal, times n.
  export fn scaledTenth(self, n: Int) -> Int { return tenth() * n }
}

/// The potion of the tests.
let p: Potion = { name: "small", heal: 500 }

emit cpp { out: "@features/pipeline" }
emit ts { out: "@features/pipeline/potion.ts" }
`

// vectorRow is one expected vector: the reads of self, the argument, the Go/C++ and TS outcomes.
type vectorRow struct {
	read, arg    int64
	want, tsWant int64
	code, tsCode diag.Code
}

var (
	codeRange = diag.E3204.Def().Code
	codeTS    = diag.E8303.Def().Code
)

// CONFORMANCE.md §6.6 (meta/decisions/log-2026-09-24.md, "conform, method-of-self reads"): the Reads and Body shapes stage E gives healFor, damageAt and scaledTenth let conform.Fill select §6.6's vectors and project a precomputed method of self; outcomes come from a stand-in interpreting the Body, not from the evaluator.
func TestIRShapesDriveConformFill(t *testing.T) {
	w := newWorld(t)
	w.add(t, "pipeline/potion.canon", []byte(potion))
	w.add(t, "pipeline.p.json", []byte(`{"name": "small", "heal": 500}`))
	pkgs := w.build(t)
	p, _ := w.Value(context.Background(), "pipeline", "p")
	methods := potionMethods(t, pkgs)
	ev := &bodyEvaluator{t: t, fns: map[check.Object]*ir.ExportFn{}}
	for _, name := range slices.Sorted(maps.Keys(methods)) {
		ev.fns[w.method(t, "Potion", name)] = methods[name]
	}
	ev.calls = []conform.Call{
		{Fn: w.method(t, "Potion", "healFor"), Recv: p, Args: []value.Value{intValue(200)}},
		{Fn: w.method(t, "Potion", "healFor"), Recv: p, Args: []value.Value{intValue(9000)}},
		{Fn: w.method(t, "Potion", "healFor"), Recv: p, Args: []value.Value{intValue(-5)}},
		{Fn: w.method(t, "Potion", "damageAt"), Recv: p, Args: []value.Value{intValue(10)}},
		{Fn: w.method(t, "Potion", "scaledTenth"), Recv: p, Args: []value.Value{intValue(3)}},
	}
	if err := conform.Fill(context.Background(), w.prog, pkgs, ev, w.bags); err != nil {
		t.Fatal(err)
	}
	checkVectors(t, methods["healFor"], []vectorRow{
		{500, 200, 200, 200, "", ""},
		{500, 9000, 500, 500, "", ""},
		{500, -5, 0, 0, "", ""},
		{500, math.MinInt64, 0, 0, "", codeTS},
		{500, -1, 0, 0, "", ""},
		{500, 0, 0, 0, "", ""},
		{500, 1, 1, 1, "", ""},
		{500, 499, 499, 499, "", ""},
		{500, 500, 500, 500, "", ""},
		{500, 501, 500, 500, "", ""},
		{500, math.MaxInt64, 500, 0, "", codeTS},
	})
	checkVectors(t, methods["damageAt"], []vectorRow{
		{500, 10, 50, 50, "", ""},
		{500, math.MinInt64, 0, 0, codeRange, codeTS},
		{500, -1, 0, 0, codeRange, codeRange},
		{500, 0, 0, 0, codeRange, codeRange},
		{500, 1, 5, 5, "", ""},
		{500, 2, 10, 10, "", ""},
		{500, 149, 745, 745, "", ""},
		{500, 150, 750, 750, "", ""},
		{500, 151, 0, 0, codeRange, codeRange},
		{500, 499, 0, 0, codeRange, codeRange},
		{500, 500, 0, 0, codeRange, codeRange},
		{500, 501, 0, 0, codeRange, codeRange},
		{500, math.MaxInt64, 0, 0, codeRange, codeTS},
	})
	if r := methods["scaledTenth"].Reads; len(r) != 1 || r[0].Name != "tenth" || len(r[0].Path) != 1 || r[0].Path[0] != "tenth" {
		t.Fatalf("scaledTenth must read the precomputed tenth as a one-segment path, got %+v", r)
	}
	if v := methods["scaledTenth"].Vectors; len(v) == 0 || v[0].Recv[0].CanonText() != "50" || v[0].Want.CanonText() != "150" {
		t.Errorf("scaledTenth's first vector must read tenth = 50 and give 150, got %+v", v)
	}
}

// potionMethods are Potion's export methods in the IR, by name; the translated ones must have a body.
func potionMethods(t *testing.T, pkgs []*ir.Package) map[string]*ir.ExportFn {
	t.Helper()
	out := map[string]*ir.ExportFn{}
	for _, p := range pkgs {
		for _, ty := range p.Types {
			if r, ok := ty.(*ir.Record); ok && r.Name == "Potion" {
				addMethods(out, r.Methods)
			}
		}
	}
	for _, name := range []string{"healFor", "damageAt", "scaledTenth"} {
		if fn := out[name]; fn == nil || fn.Body == nil {
			t.Fatalf("Potion.%s has no translated body", name)
		}
	}
	return out
}

func addMethods(out map[string]*ir.ExportFn, fns []*ir.ExportFn) {
	for _, m := range fns {
		out[m.Name] = m
	}
}

// method is the declared object of export method name of record owner.
func (w *world) method(t *testing.T, owner, name string) check.Object {
	t.Helper()
	for _, cp := range w.prog.Packages {
		for _, f := range cp.Files {
			if obj := w.methodIn(f.Decls, owner, name); obj != nil {
				return obj
			}
		}
	}
	t.Fatalf("no method %s.%s", owner, name)
	return nil
}

func (w *world) methodIn(decls []syntax.Decl, owner, name string) check.Object {
	for _, d := range decls {
		r, ok := d.(*syntax.RecordDecl)
		if !ok || r.Name.Name != owner || r.Body == nil {
			continue
		}
		for _, it := range r.Body.Items {
			if fd, isFn := it.(*syntax.FnDecl); isFn && fd.Name.Name == name {
				return w.prog.Info.Defs[fd.Name]
			}
		}
	}
	return nil
}

func checkVectors(t *testing.T, fn *ir.ExportFn, rows []vectorRow) {
	t.Helper()
	if len(fn.Vectors) != len(rows) {
		t.Fatalf("%s: %d vectors, want %d", fn.Name, len(fn.Vectors), len(rows))
	}
	for i, r := range rows {
		v := fn.Vectors[i]
		got := vectorRow{read: intOf(v.Recv[0]), arg: intOf(v.Args[0]), want: intOf(v.Want), tsWant: intOf(v.TSWant), code: v.Code, tsCode: v.TSCode}
		if got != r {
			t.Errorf("%s vector %d: got %+v, want %+v", fn.Name, i+1, got, r)
		}
	}
}

func intOf(v value.Value) int64 {
	if n, ok := v.(*value.Int); ok {
		return n.V
	}
	return 0
}

func intValue(n int64) *value.Int { return &value.Int{V: n, T: types.IntType} }

// bodyEvaluator stands in for conform.Evaluator in this test only: it interprets the few IR nodes these bodies use, after the range checks (CONFORMANCE.md §2.3), E8303 first in TS mode (§4); a precomputed method is heal / 10. It is no model of the evaluator.
type bodyEvaluator struct {
	t     *testing.T
	fns   map[check.Object]*ir.ExportFn
	calls []conform.Call
}

func (b *bodyEvaluator) TestCalls(_ context.Context, _ string, fns []check.Object) []conform.Call {
	var out []conform.Call
	for _, c := range b.calls {
		for _, f := range fns {
			if c.Fn == f {
				out = append(out, c)
			}
		}
	}
	return out
}

func (b *bodyEvaluator) Evaluate(_ context.Context, c conform.Call, m conform.Mode) conform.Outcome {
	fn := b.fns[c.Fn]
	reads := make([]int64, len(fn.Reads))
	for i, r := range fn.Reads {
		reads[i] = b.field(c.Recv, r.Path[0])
	}
	args := make([]int64, len(c.Args))
	for i, a := range c.Args {
		args[i] = intOf(a)
		if m.TS && (args[i] > tsSafe || args[i] < -tsSafe) {
			return conform.Outcome{Code: codeTS}
		}
		if r := fn.Params[i].Range; r != nil && (args[i] < r.Lo.I || args[i] > r.Hi.I) {
			return conform.Outcome{Code: codeRange}
		}
	}
	if fn.Kind == ir.FnPrecomputed {
		return conform.Outcome{Value: intValue(b.field(c.Recv, "heal") / 10)}
	}
	return conform.Outcome{Value: intValue(b.run(fn.Body, reads, args))}
}

// tsSafe is the largest integer TypeScript holds exactly (CONFORMANCE.md §4).
const tsSafe = 1<<53 - 1

// field is the Int field name of a Potion value, or the precomputed tenth.
func (b *bodyEvaluator) field(recv value.Value, name string) int64 {
	rec := recv.(*value.Record)
	fields := rec.T.Base().(*types.RecordType).Fields
	for i, f := range fields {
		if f.Name == name {
			return intOf(rec.Fields[i])
		}
	}
	return b.field(recv, "heal") / 10
}

// run interprets the nodes these bodies use.
func (b *bodyEvaluator) run(n ir.PExpr, reads, args []int64) int64 {
	switch x := n.(type) {
	case *ir.Lit:
		return intOf(x.V)
	case *ir.ParamRef:
		return args[x.Index]
	case *ir.ReadRef:
		return reads[x.Index]
	case *ir.Call:
		a, c := b.run(x.Args[0], reads, args), b.run(x.Args[1], reads, args)
		if x.Fn == ir.BuiltinMin {
			return min(a, c)
		}
		return max(a, c)
	case *ir.Binary:
		a, c := b.run(x.X, reads, args), b.run(x.Y, reads, args)
		if x.Op == ir.OpMul {
			return a * c
		}
		return a / c
	}
	b.t.Fatalf("unexpected node %T", n)
	return 0
}
