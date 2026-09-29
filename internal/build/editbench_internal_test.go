package build

import (
	"bufio"
	"context"
	"flag"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/project"
)

// benchDir is a project benchgen wrote (IMPLEMENTATION-PLAN §7.6); TestEditRecheckTimes skips without it.
var benchDir = flag.String("canon.bench", "", "a directory benchgen wrote, for TestEditRecheckTimes")

const (
	benchEdits    = 20
	benchCost     = "cost: "
	benchEntryDir = "items/IK_QUEST"
	percentile    = 95
	hundred       = 100
	peakRSSLine   = "VmHWM:"
)

// IMPLEMENTATION-PLAN §7.6 NFR-01/NFR-02, evidence only: edit then re-analyze times on the benchmark.
func TestEditRecheckTimes(t *testing.T) {
	if *benchDir == "" {
		t.Skip("no -canon.bench directory")
	}
	for _, sel := range [][]string{{"items"}, {"items", "twin"}, nil} {
		z, p, edit := benchEditor(t)
		times := make([]time.Duration, 0, benchEdits)
		for i := range benchEdits {
			edit(i)
			start := time.Now()
			if _, err := p.WithCache(z.cache).Analyze(context.Background(), sel); err != nil {
				t.Fatal(err)
			}
			times = append(times, time.Since(start))
		}
		logTimes(t, sel, times)
	}
	t.Logf("peak RSS %s", peakRSS())
	z, p, edit := benchEditor(t)
	for i := range benchEdits / 2 {
		edit(i)
		warm, err := p.WithCache(z.cache).Analyze(context.Background(), []string{"items"})
		if err != nil {
			t.Fatal(err)
		}
		if i == benchEdits/2-1 {
			cold, err := p.Analyze(context.Background(), []string{"items"})
			if err != nil {
				t.Fatal(err)
			}
			same(t, "last edit", warm, cold)
		}
	}
}

// benchEditor opens the benchmark project with a new cache, and edits one entry's cost.
func benchEditor(t *testing.T) (*analyzer, *Project, func(int)) {
	t.Helper()
	z := &analyzer{fs: newEditFS(project.OS()), dir: *benchDir, cache: NewCache()}
	list, err := os.ReadDir(path.Join(*benchDir, benchEntryDir))
	if err != nil || len(list) == 0 {
		t.Fatalf("no entry in %s: %v", benchEntryDir, err)
	}
	target := path.Join(*benchDir, benchEntryDir, list[len(list)/2].Name())
	text, err := z.fs.ReadFile(target)
	at := strings.Index(string(text), benchCost)
	if err != nil || at < 0 {
		t.Fatalf("%s: %v, cost at %d", target, err, at)
	}
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	end := at + strings.IndexByte(string(text[at:]), '\n')
	return z, p, func(i int) {
		z.fs.set(target, slices.Concat(text[:at], []byte(benchCost+strconv.Itoa(i+1)), text[end:]))
	}
}

// logTimes logs the first (cold) analysis, then p50 and p95 of the others.
func logTimes(t *testing.T, sel []string, times []time.Duration) {
	t.Helper()
	warm := slices.Sorted(slices.Values(times[1:]))
	p95 := warm[(len(warm)*percentile+hundred-1)/hundred-1]
	t.Logf("%v: first %v, then p50 %v p95 %v max %v", sel, times[0], warm[len(warm)/2], p95, warm[len(warm)-1])
}

// peakRSS is the process's peak resident set, as Linux reports it; "?" elsewhere.
func peakRSS() string {
	f, err := os.Open(path.Join("/proc", "self", "status"))
	if err != nil {
		return "?"
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if rest, ok := strings.CutPrefix(s.Text(), peakRSSLine); ok {
			return strings.TrimSpace(rest)
		}
	}
	return "?"
}
