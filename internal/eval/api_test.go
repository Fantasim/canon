package eval_test

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// object is the declaration named name in package pkg.
func object(t *testing.T, b *build, pkg, name string) check.Object {
	t.Helper()
	for _, p := range b.checked.Packages {
		for _, obj := range p.Decls {
			if p.Path == pkg && obj.Name() == name {
				return obj
			}
		}
	}
	t.Fatalf("no %s.%s", pkg, name)
	return nil
}

// EVALUATION.md §2.3: stage E computes canTransition for every pair of statuses.
func TestCall(t *testing.T) {
	b := runBuild(t, fromExamples(t, "teamboard", "sovcommon/roles", "sovcommon/ui"), eval.Options{}, "teamboard")
	fn := object(t, b, "teamboard", "canTransition")
	statuses, ok := b.ev.Force(context.Background(), eval.Root{Pkg: "teamboard", Name: "statuses"})
	if !ok {
		t.Fatal("statuses poisoned")
	}
	rt := fn.Type().(*types.FuncType).Params[0]
	var yes []string
	for _, from := range statuses.(*value.Table).Entries {
		for _, to := range statuses.(*value.Table).Entries {
			args := []value.Value{&value.Ref{T: rt, Key: from.Ident.Key}, &value.Ref{T: rt, Key: to.Ident.Key}}
			v, ok := b.ev.Call(context.Background(), fn, nil, args)
			if !ok {
				t.Fatalf("canTransition(%s, %s) failed", from.Ident.Key.S, to.Ident.Key.S)
			}
			if v.(*value.Bool).V {
				yes = append(yes, from.Ident.Key.S+">"+to.Ident.Key.S)
			}
		}
	}
	want := "open>taken open>wont_do open>duplicate taken>open taken>fixed taken>wont_do taken>duplicate fixed>open fixed>verified verified>open wont_do>open duplicate>open"
	if got := strings.Join(yes, " "); got != want {
		t.Errorf("transitions:\n%s\nwant\n%s", got, want)
	}
}

// EVALUATION.md §2.3: a precomputation's error names the fn and its inputs in its outermost frame.
func TestCallError(t *testing.T) {
	src := "/// A.\npackage a\n\n/// F.\nexport fn f(n: Int) -> Int { return 10 / n }\n"
	b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(src)}), eval.Options{})
	zero := &value.Int{V: 0, T: types.IntType}
	if _, ok := b.ev.Call(context.Background(), object(t, b, "a", "f"), nil, []value.Value{zero}); ok {
		t.Fatal("f(0) succeeded")
	}
	if out := b.findings(t); !strings.Contains(out, "["+codeOf(diag.E4102)+"]") || !strings.Contains(out, "in f(0) (a/a.canon:5)") {
		t.Errorf("findings:\n%s", out)
	}
}

// DECISIONS 148: Where at no step cost, Poison, and the invalid marks of EVALUATION.md §7.3.
func TestVerifierSurface(t *testing.T) {
	src := "/// A.\npackage a\n\nlocal let x: Int where it > 3 = 5\n"
	b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(src)}), eval.Options{Budget: 10})
	pred := object(t, b, "a", "x").Type().(*types.Refined).Where
	for n, want := range map[int64]bool{2: false, 5: true} {
		holds, ok := b.ev.Where(context.Background(), pred, &value.Int{V: n, T: types.IntType}, "")
		if !ok || holds != want {
			t.Errorf("where %d: holds %t ok %t", n, holds, ok)
		}
	}
	root := eval.Root{Pkg: "a", Name: "x"}
	v, ok := b.ev.Force(context.Background(), root)
	if !ok {
		t.Fatal("x poisoned")
	}
	b.ev.MarkInvalid(v)
	if !b.ev.Invalid(v) || b.ev.Invalid(&value.Int{V: 5, T: types.IntType}) {
		t.Error("invalid marks are per instance")
	}
	b.ev.Poison(root)
	if _, ok := b.ev.Force(context.Background(), root); ok {
		t.Error("a poisoned value is read")
	}
}

// EVALUATION.md §12.2: the default budget.
func TestDefaultBudget(t *testing.T) {
	if eval.DefaultBudget != 100_000_000 {
		t.Errorf("DefaultBudget = %d", eval.DefaultBudget)
	}
}

// DOCTRINE §5, IMPLEMENTATION-PLAN §7.5: the same output whatever GOMAXPROCS, side by side.
func TestDeterminism(t *testing.T) {
	run := func() string {
		b := runBuild(t, fromExamples(t), eval.Options{})
		return b.dump() + b.findings(t) + runTests(t, b)
	}
	want := run()
	for _, procs := range []int{1, 8} {
		old := runtime.GOMAXPROCS(procs)
		var wg sync.WaitGroup
		outs := make([]string, 4)
		for i := range outs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				outs[i] = run()
			}()
		}
		wg.Wait()
		runtime.GOMAXPROCS(old)
		for _, got := range outs {
			if got != want {
				t.Fatalf("GOMAXPROCS %d: output differs", procs)
			}
		}
	}
}
