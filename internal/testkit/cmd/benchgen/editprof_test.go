//go:build linux

package main

import (
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"strings"
	"testing"
	"time"
)

// The opt-in profile of the NFR-01 edit loop (log-2026-09-29 M4 P14): Sets drawn as TestBenchEdit
// draws them, each Edit and each Evaluate after it timed, the warm ones profiled.
var (
	flagProf    = flag.Int("benchgen.editprof", 0, "run TestEditProfile: this many Sets on a fresh benchmark project")
	flagProfN   = flag.Int("benchgen.editprof.n", defaultN, "the entries of TestEditProfile's benchmark project")
	flagProfOut = flag.String("benchgen.editprof.out", "", "a path prefix for TestEditProfile's CPU and heap profiles")
)

const (
	profWarm     = 3        // Sets left out of the profiles and the percentiles: the first analyses
	profHeapRate = 64 << 10 // the heap profile's sampling rate
	profMedian   = 50       // the percentile printed beside the p95
	profCPUExt   = ".cpu"   // the CPU profile's suffix
	profHeapExt  = ".mem"   // the heap profile's suffix
	profAllocs   = "allocs" // the heap profile's name
	profUnit     = "ms"     // the unit times are printed in
	profFmt      = "%-16s p50 %7.1f %s  p95 %7.1f %s  max %7.1f %s  (n=%d)"
	profSetFmt   = "Set %-8s %-48s Edit %7.1f ms  Evaluate %6.1f ms"
	profPkgSep   = ":"        // a value path's package ends here (API.md P1)
	profEditName = "Edit"     // the Edit line's name
	profEvalName = "Evaluate" // the Evaluate line's name
)

// NFR-01 evidence, not a gate: the p50, p95 and maximum of an Edit and an Evaluate, warm.
func TestEditProfile(t *testing.T) {
	if *flagProf <= 0 {
		t.Skip("guarded by -benchgen.editprof")
	}
	dir := filepath.Join(t.TempDir(), "bench")
	if err := generate(benchSeed, dir, *flagProfN); err != nil {
		t.Fatalf("generate: %v", err)
	}
	target := benchTarget{name: "bench", dir: dir, guarded: true, canonical: true}
	p := openClean(t, target)
	var r report
	s := newSetter(t, p, &r, target)
	for len(s.edits) < profWarm {
		if f, ok := s.draw(t); ok {
			s.set(t, f)
		}
	}
	s.edits, s.evals = nil, nil
	stop := startProf(t)
	byPkg := map[string][]time.Duration{} // each Edit's time, by the package of the value it set
	for len(s.edits) < *flagProf {
		f, ok := s.draw(t)
		if !ok {
			continue
		}
		n := len(s.edits)
		s.set(t, f)
		if len(s.edits) > n {
			pkg, _, _ := strings.Cut(f.path, profPkgSep)
			byPkg[pkg] = append(byPkg[pkg], s.edits[n])
			t.Logf(profSetFmt, f.kind, f.path, ms(s.edits[n]), ms(s.evals[n]))
		}
	}
	stop()
	logPercentiles(t, profEditName, s.edits)
	logPercentiles(t, profEvalName, s.evals)
	for _, pkg := range slices.Sorted(maps.Keys(byPkg)) {
		logPercentiles(t, profEditName+" "+pkg, byPkg[pkg])
	}
	t.Logf("rejected %d, not editable %d", len(s.rejects), s.notEditable)
}

// logPercentiles logs the p50, p95 and maximum of ds under name.
func logPercentiles(t *testing.T, name string, ds []time.Duration) {
	t.Helper()
	t.Logf(profFmt, name, percentile(ds, profMedian), profUnit, percentile(ds, benchPercentile), profUnit,
		percentile(ds, benchPercent), profUnit, len(ds))
}

// ms is d in milliseconds.
func ms(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// percentile is the nearest-rank pth percentile of ds in milliseconds; 0 for no sample.
func percentile(ds []time.Duration, p int) float64 {
	if len(ds) == 0 {
		return 0
	}
	sorted := slices.Sorted(slices.Values(ds))
	return float64(sorted[max((len(sorted)*p+benchPercent-1)/benchPercent-1, 0)]) / float64(time.Millisecond)
}

// startProf starts the CPU profile when asked; the function returned stops it and writes the
// heap's allocations.
func startProf(t *testing.T) func() {
	t.Helper()
	if *flagProfOut == "" {
		return func() {}
	}
	cpu, err := os.Create(*flagProfOut + profCPUExt)
	if err != nil {
		t.Fatal(err)
	}
	rate := runtime.MemProfileRate
	runtime.MemProfileRate = profHeapRate
	if err := pprof.StartCPUProfile(cpu); err != nil {
		t.Fatal(err)
	}
	return func() {
		pprof.StopCPUProfile()
		_ = cpu.Close()
		defer func() { runtime.MemProfileRate = rate }()
		mem, err := os.Create(*flagProfOut + profHeapExt)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = mem.Close() }()
		if err := pprof.Lookup(profAllocs).WriteTo(mem, 0); err != nil {
			t.Fatal(fmt.Errorf("heap profile: %w", err))
		}
	}
}
