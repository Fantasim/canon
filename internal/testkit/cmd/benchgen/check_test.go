package main

import (
	"context"
	"flag"
	"path/filepath"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// prN is IMPLEMENTATION-PLAN.md §7.6's pull-request size, reported but not gating.
const prN = 1000

// flagFull runs TestCheckCleanDefaultN at the full 7,000-entry size; `make bench` sets it.
var flagFull = flag.Bool("benchgen.full", false, "run the default-n check (slow; make bench)")

// TestCheckClean proves the generated project checks with zero findings, warnings included.
func TestCheckClean(t *testing.T) {
	checkGenerated(t, 1, prN)
}

// TestCheckCleanDefaultN runs the same check at the default 7,000 entries; opt-in via
// -benchgen.full (make bench), since it is too heavy for the `make check` gate.
func TestCheckCleanDefaultN(t *testing.T) {
	if !*flagFull {
		t.Skip("guarded by -benchgen.full: run with `make bench`")
	}
	checkGenerated(t, 1, defaultN)
}

// checkGenerated writes the project, then checks it in-process, and fails on any finding at all.
func checkGenerated(t *testing.T, seed uint64, n int) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "bench")
	start := time.Now()
	if err := generate(seed, out, n); err != nil {
		t.Fatalf("generate: %v", err)
	}
	t.Logf("generate(seed=%d, n=%d): %s", seed, n, time.Since(start))

	p, err := canon.Open(out, canon.Options{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = p.Close() }()

	start = time.Now()
	res, err := p.Check(context.Background())
	t.Logf("Check: %s", time.Since(start))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(res.Findings) > 0 {
		t.Errorf("%d errors, %d warnings (want 0 findings):", res.Summary.Errors, res.Summary.Warnings)
		for _, f := range res.Findings {
			t.Errorf("  %s %s:%d: %s", f.Code, f.File, f.Line, f.Message)
		}
	}
}
