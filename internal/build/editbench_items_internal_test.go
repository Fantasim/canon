package build

import (
	"context"
	"flag"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"
)

// benchItems is the benchmark's item package.
const benchItems = "items"

// benchProfile names the CPU and heap profiles of the warm runs of TestEditRecheckItems.
var benchProfile = flag.String("canon.benchprof", "", "a path prefix for TestEditRecheckItems's warm profiles")

// IMPLEMENTATION-PLAN §7.6 NFR-01/NFR-02, evidence only: [items] edit then re-analyze times.
func TestEditRecheckItems(t *testing.T) {
	if *benchDir == "" {
		t.Skip("no -canon.bench directory")
	}
	z, p, edit := benchEditor(t)
	times := make([]time.Duration, 0, benchEdits)
	for i := range benchEdits {
		edit(i)
		if i == 1 {
			defer startProfile(t)()
		}
		start := time.Now()
		if _, err := p.WithCache(z.cache).Analyze(context.Background(), []string{benchItems}); err != nil {
			t.Fatal(err)
		}
		times = append(times, time.Since(start))
	}
	logTimes(t, []string{benchItems}, times)
	verified, checked := z.replays()
	t.Logf("entries replayed: stage B %d, stage C %d; peak RSS %s", verified, checked, peakRSS())
}

// startProfile starts the warm CPU profile when asked; the function returned stops it and
// writes the heap's allocations.
func startProfile(t *testing.T) func() {
	t.Helper()
	if *benchProfile == "" {
		return func() {}
	}
	cpu, err := os.Create(*benchProfile + ".cpu")
	if err != nil {
		t.Fatal(err)
	}
	runtime.MemProfileRate = 64 << 10
	if err := pprof.StartCPUProfile(cpu); err != nil {
		t.Fatal(err)
	}
	return func() {
		pprof.StopCPUProfile()
		_ = cpu.Close()
		mem, err := os.Create(*benchProfile + ".mem")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = mem.Close() }()
		if err := pprof.Lookup("allocs").WriteTo(mem, 0); err != nil {
			t.Fatal(err)
		}
	}
}
