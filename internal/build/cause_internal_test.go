package build

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/eval"
)

// prompt bounds how long a cancelled call may take to return (API.md S11).
const prompt = 5 * time.Second

// poisoned is the earlier case's root whose cause is a division by zero.
var poisoned = eval.Root{Pkg: "a", Name: "half"}

// API.md R6, S11 (log-2026-09-29 M4 U8-r): a memoized analysis's Cause returns ctx.Err() promptly
// when cancelled, keeps nothing, and a later call computes the cold analysis's causes.
func TestCauseCancelled(t *testing.T) {
	z := archiveAnalyzer(t, earlierCase)
	warm, cold := z.pair(t)
	if !warm.r.memoized {
		t.Fatal("the warm analysis replays no memo")
	}
	done, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if _, err := warm.Cause(done, poisoned); !errors.Is(err, context.Canceled) || time.Since(start) > prompt {
		t.Fatalf("a cancelled Cause: %v after %v", err, time.Since(start))
	}
	if warm.causes != nil {
		t.Fatal("a cancelled cause run was kept")
	}
	got, err := warm.Cause(context.Background(), poisoned)
	want, _ := cold.Cause(context.Background(), poisoned)
	if err != nil || len(got) == 0 || !reflect.DeepEqual(got, want) {
		t.Errorf("Cause after a cancelled one: %v, %v; cold %v", got, err, want)
	}
}

// API.md S7, S11: Cause runs its cold evaluation outside the analysis's lock, beside a view model
// and a cancellation.
func TestCauseBesideViewModel(t *testing.T) {
	if testing.Short() {
		t.Skip("every example")
	}
	z := examplesAnalyzer(t)
	z.pair(t)
	warm, _ := z.pair(t)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for _, run := range []func(){
		func() { _, _ = warm.Cause(ctx, eval.Root{Pkg: "studio", Name: "units"}) },
		func() { _, _ = warm.Cause(context.Background(), eval.Root{Pkg: "studio", Name: "units"}) },
		func() { _, _ = warm.ViewModel(context.Background(), "studio") },
		cancel,
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run()
		}()
	}
	wg.Wait()
	if warm.causes == nil {
		t.Error("no cause run was kept")
	}
}
