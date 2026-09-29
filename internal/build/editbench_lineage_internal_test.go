package build

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// IMPLEMENTATION-PLAN §7.6 NFR-01/NFR-02, evidence only: selections analyzed in turn through one cache.
func TestEditRecheckTimesAlternating(t *testing.T) {
	if *benchDir == "" {
		t.Skip("no -canon.bench directory")
	}
	sels := [][]string{{"items"}, {"items", "twin"}, nil}
	z, p, edit := benchEditor(t)
	times := make([][]time.Duration, len(sels))
	epochs := make([][]uint64, len(sels))
	for i := range benchEdits {
		edit(i)
		for j, sel := range sels {
			start := time.Now()
			a, err := p.WithCache(z.cache).Analyze(context.Background(), sel)
			if err != nil {
				t.Fatal(err)
			}
			times[j] = append(times[j], time.Since(start))
			epochs[j] = append(epochs[j], a.r.epoch)
		}
	}
	for j, sel := range sels {
		logTimes(t, []string{fmt.Sprint("in turn ", sel)}, times[j])
		t.Logf("%v: epochs %v", sel, epochs[j])
	}
	t.Logf("peak RSS %s", peakRSS())
}
