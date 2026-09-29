package build

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/project"
)

// twoDisplays is a project reaching data/x.json as data/x.json (a) and @data/x.json (b), and
// data/p1.json through data (c) and through the link to it (d).
var twoDisplays = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n  roots {\n    data: \"data\"\n  }\n}\n",
	"data/x.json":   "[1, 2]\n",
	"data/p1.json":  "{\"id\": \"p1\", \"v\": 1}\n",
	"a/a.canon":     "/// A.\npackage a\n\n/// Xs.\nlet xs: [Int] = load(\"../data/x.json\")\n",
	"b/b.canon":     "/// B.\npackage b\n\n/// Xs.\nlet xs: [Int] = load(\"@data/x.json\")\n",
	"c/c.canon":     "/// C.\npackage c\n\n/// A P.\nrecord P {\n  /// Id.\n  id: String\n  /// V.\n  v: Int\n}\n\n/// Ps.\nlet ps: [P] keyed by id = load.dir(\"../data/*.json\")\n",
	"d/d.canon":     "/// D.\npackage d\n\nimport c { P }\n\n/// Ps.\nlet ps: [P] keyed by id = load.dir(\"../link/*.json\")\n",
}

// recFS records the displays each run tells it (API.md S3), by absolute name.
type recFS struct {
	*editFS
	mu   sync.Mutex
	told map[string][]string
}

func (r *recFS) RecordReads(reads []Read) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rd := range reads {
		r.told[rd.Abs] = append(r.told[rd.Abs], rd.Display)
	}
}

// listing is what the run told, in order.
func (r *recFS) listing() string {
	var sb strings.Builder
	for _, abs := range slices.Sorted(maps.Keys(r.told)) {
		fmt.Fprintf(&sb, "%s %v\n", abs, r.told[abs])
	}
	return sb.String()
}

// API.md S3, S5 (log-2026-09-29 M4 U8-r): one file reached through two displays is told and read
// by the least display this run's loads used, whatever other runs load meanwhile.
func TestLeastDisplayAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	for _, name := range slices.Sorted(maps.Keys(twoDisplays)) {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), dirMode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(twoDisplays[name]), fileMode); err != nil {
			t.Fatal(err)
		}
	}
	if os.Symlink("data", filepath.Join(dir, "link")) != nil {
		t.Skip("no symbolic links here")
	}
	base, cache := newEditFS(project.OS()), NewCache()
	sels := [][]string{{"a"}, {"b"}, {"c"}, {"d"}, nil}
	var wg sync.WaitGroup
	got := make([][2]string, parallelSnapshots*len(sels))
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j, c := range []*Cache{cache, nil} {
				got[i][j] = displaysOf(t, base, filepath.ToSlash(dir), c, sels[i%len(sels)])
			}
		}()
	}
	wg.Wait()
	for i, g := range got {
		if g[0] != g[1] {
			t.Errorf("run %d %v: warm\n%s\ncold\n%s", i, sels[i%len(sels)], g[0], g[1])
		}
	}
	if all := got[len(sels)-1][0]; !strings.Contains(all, "a: @data/x.json") || !strings.Contains(all, "d: data/p1.json") {
		t.Errorf("the whole project's reads:\n%s", got[len(sels)-1][0])
	}
}

// displaysOf is what a run of sel through cache (nil: cold) tells its file system and the read
// sets of its packages.
func displaysOf(t *testing.T, base *editFS, dir string, cache *Cache, sel []string) string {
	rec := &recFS{editFS: base, told: map[string][]string{}}
	p, err := Open(rec, dir, Options{})
	if err != nil {
		t.Error(err)
		return ""
	}
	a, err := p.WithCache(cache).Analyze(context.Background(), sel)
	if err != nil {
		t.Error(err)
		return ""
	}
	var sb strings.Builder
	sb.WriteString(rec.listing())
	for _, pkg := range a.Result().Packages {
		for _, r := range a.Reads(pkg) {
			fmt.Fprintf(&sb, "%s: %s\n", pkg, r.Display)
		}
	}
	return sb.String()
}
