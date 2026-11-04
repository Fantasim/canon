package build

import (
	"context"
	"fmt"
	"slices"
	"testing"
)

// layersFS is package a with layersMany layers, each amending its own entry.
const layersMany = 6

func layersFS() roFS {
	fsys := roFS{
		"law/project.canon": srcFile("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon": srcFile("/// A.\npackage a\n\n/// A row.\nrecord R {\n  /// N.\n  n: Int = 0\n}\n\n" +
			"/// Rows.\nlet rs: table R = {\n  e0 { n: 0 }\n  e1 { n: 1 }\n  e2 { n: 2 }\n  e3 { n: 3 }\n  e4 { n: 4 }\n  e5 { n: 5 }\n}\n"),
	}
	for i := range layersMany {
		fsys[fmt.Sprintf("law/a/l%d.layer.canon", i)] = srcFile(fmt.Sprintf("package a\nlayer l%d\n\namend rs {\n  e%d.n: 9\n}\n", i, i))
	}
	return fsys
}

// cacheState is c's lineage heads and last epoch.
func cacheState(c *Cache) ([]*head, uint64) {
	c.mu.Lock()
	g, epoch := c.gen, c.epoch
	c.mu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.heads), epoch
}

// API.md E18 (DECISIONS 463): WithLayers(…).WithCache(nil), an edit's inactive-layer run,
// keeps the session's Recheck lineage and epoch however many layers run; through the cache the
// same runs evict it (lineageCap).
func TestWithLayersAsideKeepsLineages(t *testing.T) {
	ctx := context.Background()
	p, err := Open(layersFS(), "/law", Options{})
	if err != nil {
		t.Fatal(err)
	}
	p = p.WithCache(NewCache())
	if _, err := p.AnalyzeOnly(ctx, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	heads, epoch := cacheState(p.cache)
	for i := range layersMany {
		if _, err := p.WithLayers([]string{fmt.Sprintf("l%d", i)}).WithCache(nil).AnalyzeOnly(ctx, []string{"a"}); err != nil {
			t.Fatal(err)
		}
	}
	if gotHeads, gotEpoch := cacheState(p.cache); !slices.Equal(gotHeads, heads) || gotEpoch != epoch {
		t.Errorf("the session's cache changed: heads %v epoch %d, want %v epoch %d", gotHeads, gotEpoch, heads, epoch)
	}
	for i := range layersMany {
		if _, err := p.WithLayers([]string{fmt.Sprintf("l%d", i)}).AnalyzeOnly(ctx, []string{"a"}); err != nil {
			t.Fatal(err)
		}
	}
	if gotHeads, _ := cacheState(p.cache); slices.Contains(gotHeads, heads[0]) {
		t.Error("through the cache, the layer runs kept the session's lineage: the test has no teeth")
	}
}
