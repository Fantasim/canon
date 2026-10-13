package lsp

import (
	"flag"
	"fmt"
	"path/filepath"
	"testing"
)

var flagMemVisit = flag.Int("lsp.memvisit", 0, "large synthetic projects the visit bench opens, edits and closes one after another (0: skip)")

// The visit bench: an editor visits copies of a large project in turn (worktrees, say), opening
// visitOpened buffers in each, making visitEdits edits, closing them; sampled after each.
const (
	visitOpened = 2
	visitEdits  = 20
)

// DECISIONS 368: opt-in, `go test ./internal/lsp -run TestMemoryVisitBench -lsp.memvisit 3`.
func TestMemoryVisitBench(t *testing.T) {
	if *flagMemVisit <= 0 {
		t.Skip("opt-in: -lsp.memvisit <projects>")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	c := startMem(t, base, config{debounce: memDebounce})
	var samples []sample
	for k := range *flagMemVisit {
		name := fmt.Sprintf("copy%d", k)
		writeSynth(t, filepath.Join(base, name), shapeLarge)
		e := &editing{c: c, pick: func(j int) string { return name + "/" + shapeLarge.candidate(j) }, text: map[string]string{}}
		for range visitOpened {
			e.openNext()
		}
		e.c.wait()
		for i := range visitEdits {
			e.edit(i)
		}
		for _, rel := range e.open {
			c.close(rel)
		}
		c.wait()
		c.settle()
		samples = append(samples, measure())
		t.Logf("after project %d closed: %v", k+1, samples[k])
		writeProfile(t, *flagMemProf, fmt.Sprintf("heap-visit-%d.pprof", k+1))
	}
	c.stop()
}
