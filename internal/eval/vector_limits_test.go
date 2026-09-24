package eval_test

import (
	"context"
	"runtime"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
)

// The limits tests: a runaway's depth, how often it runs, the heap it may keep, the budgets.
const (
	runawayDepth = 60
	runs         = 8
	heapSlack    = 64 << 20
	smallCap     = 100
	stageBudget  = 20_000
	cheapDepth   = 8
)

// CONFORMANCE.md §6.5, EVALUATION.md §3.3: the step cap and the depth cut a vector; runaways keep no memory.
func TestVectorLimits(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "gauge/gauge.canon", gaugeSrc)
	g := b.values[eval.Root{Pkg: gaugePkg, Name: "g"}]
	ctx := context.Background()
	deep := eval.Call{Fn: fnObj(t, b, gaugePkg, "Gauge", "deep"), Recv: g, Args: ints(0)}
	if o := b.ev.Vector(ctx, deep, eval.VectorMode{Steps: vectorCap}); o.Exceeded != eval.DepthLimit {
		t.Errorf("deep: %+v, want the depth limit", o)
	}
	blow := eval.Call{Fn: fnObj(t, b, gaugePkg, "Gauge", "blow"), Recv: g, Args: ints(runawayDepth)}
	before := heapInUse()
	for range runs {
		for _, ts := range []bool{false, true} {
			if o := b.ev.Vector(ctx, blow, eval.VectorMode{Steps: vectorCap, TS: ts}); o.Exceeded != eval.StepLimit {
				t.Fatalf("blow, TS %t: %+v, want the step limit", ts, o)
			}
		}
	}
	if grown := heapInUse() - before; grown > heapSlack {
		t.Errorf("the heap grew by %d bytes over %d runaway vectors", grown, runs)
	}
	assertEmpty(t, b.bags)
}

func heapInUse() int64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.HeapInuse)
}

// CONFORMANCE.md §6.5: vectors spare the project budget; a value first forced inside one is charged to it.
func TestVectorBudgets(t *testing.T) {
	b := buildFiles(t, eval.Options{}, "gauge/gauge.canon", gaugeSrc)
	ctx := context.Background()
	bags := check.Bags{gaugePkg: diag.NewBag(b.prog.fs, gaugePkg)}
	ev := eval.New(b.checked, served{}, bags, eval.Options{Budget: stageBudget})
	g, ok := ev.Force(ctx, eval.Root{Pkg: gaugePkg, Name: "g"})
	if !ok {
		t.Fatal("g is poisoned")
	}
	blow := eval.Call{Fn: fnObj(t, b, gaugePkg, "Gauge", "blow"), Recv: g, Args: ints(cheapDepth)}
	if o := ev.Vector(ctx, blow, eval.VectorMode{Steps: stageBudget / runs}); o.Exceeded != eval.StepLimit {
		t.Fatalf("blow costs less than the project budget over %d runs: %+v", runs, o)
	}
	for range runs {
		if o := ev.Vector(ctx, blow, eval.VectorMode{Steps: vectorCap}); o.Value == nil {
			t.Fatalf("blow: %+v", o)
		}
	}
	heavy := eval.Call{Fn: fnObj(t, b, gaugePkg, "Gauge", "plusHeavy"), Recv: g, Args: ints(1)}
	if o := ev.Vector(ctx, heavy, eval.VectorMode{Steps: smallCap}); o.Exceeded != eval.StepLimit {
		t.Errorf("heavy first forced in the vector: %+v, want the step limit", o)
	}
	if _, ok := ev.Force(ctx, eval.Root{Pkg: gaugePkg, Name: "heavy"}); !ok {
		t.Errorf("heavy, forced after the vectors, is poisoned: %v", bags[gaugePkg].Findings())
	}
	o := ev.Vector(ctx, heavy, eval.VectorMode{Steps: smallCap})
	if n, isInt := o.Value.(*value.Int); !isInt || n.V != 201 {
		t.Errorf("heavy forced before the vector: %+v, want 201", o)
	}
	if err := ev.Err(); err != nil {
		t.Errorf("internal error: %v", err)
	}
	assertEmpty(t, bags)
}
