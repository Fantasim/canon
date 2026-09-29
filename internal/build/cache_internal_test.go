package build

import (
	"context"
	"fmt"
	"path"
	"sync"
	"testing"
)

const parallelSnapshots = 8

// IMPLEMENTATION-PLAN §7.6 NFR-02, API.md S7-S8: snapshots analyzed at once through one cache.
func TestCacheConcurrentSnapshots(t *testing.T) {
	base := archiveAnalyzer(t, entriesCase)
	two := path.Join(archiveRoot, "a/items/two.canon")
	text, err := base.fs.ReadFile(two)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	dumps := make([][2]string, parallelSnapshots)
	for i := range parallelSnapshots {
		z := &analyzer{fs: newEditFS(base.fs.base), dir: base.dir, cache: base.cache}
		edited := fieldNumber.ReplaceAll(text, fmt.Appendf(nil, ": %d", i%3))
		z.fs.set(two, edited)
		wg.Add(1)
		go func() {
			defer wg.Done()
			dumps[i] = analyzeTwice(t, z)
		}()
	}
	wg.Wait()
	for i, d := range dumps {
		if d[0] != d[1] {
			t.Errorf("snapshot %d: warm differs from cold", i)
		}
	}
}

// analyzeTwice is the dumps of a warm and a cold analysis of z, "" for a failed one.
func analyzeTwice(t *testing.T, z *analyzer) [2]string {
	ctx := context.Background()
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Error(err)
		return [2]string{}
	}
	var out [2]string
	for i, q := range []*Project{p.WithCache(z.cache), p} {
		a, err := q.Analyze(ctx, nil)
		if err != nil {
			t.Error(err)
			return [2]string{}
		}
		out[i] = dumpAnalysis(t, a)
	}
	return out
}

// IMPLEMENTATION-PLAN §7.6 NFR-02 (log-2026-09-29 "U12 review PASS (b)"): a grown file set starts anew.
func TestCacheCompacts(t *testing.T) {
	z := archiveAnalyzer(t, earlierCase)
	before, _ := z.pair(t)
	old := z.cache.gen
	old.mu.Lock()
	old.total = compactFloor + 1
	old.live = 0
	old.mu.Unlock()
	after, cold := z.pair(t)
	if z.cache.gen == old || after.r.s.gen == old || before.r.s.gen != old {
		t.Fatal("the grown file set was not replaced for the next snapshot only")
	}
	same(t, "after compaction", after, cold)
	same(t, "begun before", before, cold)
	again, cold := z.pair(t)
	if z.cache.gen != after.r.s.gen {
		t.Error("a file set that did not grow was replaced")
	}
	same(t, "next", again, cold)
}
