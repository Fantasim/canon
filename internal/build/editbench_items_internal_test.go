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
const (
	benchItems       = "items"
	benchSampleBytes = 64 << 10 // the heap profile's sampling of the warm runs
)

// benchProfile names the CPU and heap profiles of the warm runs of TestEditRecheckItems.
var benchProfile = flag.String("canon.benchprof", "", "a path prefix for TestEditRecheckItems's warm profiles")

// IMPLEMENTATION-PLAN §7.6 NFR-01/NFR-02, evidence only: [items] edit then re-analyze times.
func TestEditRecheckItems(t *testing.T) {
	if *benchDir == "" {
		t.Skip("no -canon.bench directory")
	}
	z, p, edit := benchEditor(t)
	times := make([]time.Duration, 0, benchEdits)
	var verified, checked int
	analyze := func(i int) {
		edit(i)
		start := time.Now()
		a, err := p.WithCache(z.cache).Analyze(context.Background(), []string{benchItems})
		if err != nil {
			t.Fatal(err)
		}
		times = append(times, time.Since(start))
		v, c := replaysOf(a)
		verified, checked = verified+v, checked+c
	}
	analyze(0) // cold: recorded, left out of the profiles
	stop := startProfile(t)
	for i := 1; i < benchEdits; i++ {
		analyze(i)
	}
	stop()
	logTimes(t, []string{benchItems}, times)
	t.Logf("entries replayed: stage B %d, stage C %d; peak RSS %s", verified, checked, peakRSS())
}

// startProfile starts the warm CPU profile when asked; the function returned stops it, writes the
// heap's allocations and restores the sampling rate.
func startProfile(t *testing.T) func() {
	t.Helper()
	if *benchProfile == "" {
		return func() {}
	}
	cpu, err := os.Create(*benchProfile + ".cpu")
	if err != nil {
		t.Fatal(err)
	}
	rate := runtime.MemProfileRate
	runtime.MemProfileRate = benchSampleBytes
	if err := pprof.StartCPUProfile(cpu); err != nil {
		t.Fatal(err)
	}
	return func() {
		pprof.StopCPUProfile()
		_ = cpu.Close()
		defer func() { runtime.MemProfileRate = rate }()
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
