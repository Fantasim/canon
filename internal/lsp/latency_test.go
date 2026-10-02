package lsp

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

var flagBench = flag.Int("lsp.bench", 0, "entries of a benchmark project to measure diagnostics latency on (0: skip)")

// The latency target of IMPLEMENTATION-PLAN §6 M5, and the edits measured.
const (
	latencyTarget = 500 * time.Millisecond
	latencyEdits  = 20
)

// IMPLEMENTATION-PLAN §6 M5: opt-in, `go test ./internal/lsp -run TestLatency -lsp.bench 7000`.
func TestLatency(t *testing.T) {
	// benchgen writes the project, as `make bench-edit` does; each edit makes or fixes a type
	// error in one entry file, timed from the change to the publication.
	if *flagBench <= 0 {
		t.Skip("opt-in: -lsp.bench <entries>")
	}
	dir := benchProject(t, *flagBench)
	s := sessionIn(t, dir)
	done := s.start()
	entry := firstEntry(t, dir)
	must(t, s.initialize(nil))
	start := time.Now()
	must(t, s.open([]string{entry}))
	must(t, s.wait(nil))
	t.Logf("first pass after open (cold check of every package): %v", time.Since(start))
	text, err := os.ReadFile(filepath.Join(dir, entry))
	must(t, err)
	broken := strings.Replace(string(text), "  tier: ", "  tier: \"x\" // ", 1)
	var times []time.Duration
	for i := range latencyEdits {
		s.files["edit"] = []byte(broken)
		if i%2 == 1 {
			s.files["edit"] = text
		}
		start := time.Now()
		must(t, s.change([]string{entry, "edit"}))
		must(t, s.wait(nil))
		times = append(times, time.Since(start))
	}
	must(t, s.in.Close())
	<-done
	slices.Sort(times)
	p95 := times[len(times)*95/100-1]
	t.Logf("%d entries, %d edits of %s, from the change to the publication (debounce %v included): min %v, median %v, p95 %v, max %v",
		*flagBench, latencyEdits, entry, debounce, times[0], times[len(times)/2], p95, times[len(times)-1])
	if p95 > latencyTarget {
		t.Errorf("p95 from the last change to publication %v, over the %v target", p95, latencyTarget)
	}
}

// benchProject writes a benchmark project of n entries with benchgen, seed 1.
func benchProject(t *testing.T, n int) string {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	out := filepath.Join(dir, "bench")
	cmd := exec.Command("go", "run", "./internal/testkit/cmd/benchgen", "-seed", "1", "-n", strconv.Itoa(n), "-out", out)
	cmd.Dir = filepath.Join("..", "..")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("benchgen: %v\n%s", err, msg)
	}
	return out
}

// firstEntry is the first entry file of the items package, relative to dir.
func firstEntry(t *testing.T, dir string) string {
	matches, err := filepath.Glob(filepath.Join(dir, "items", "*", "ITM_*.canon"))
	must(t, err)
	if len(matches) == 0 {
		t.Fatal("no entry file")
	}
	slices.Sort(matches)
	rel, err := filepath.Rel(dir, matches[0])
	must(t, err)
	return filepath.ToSlash(rel)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
