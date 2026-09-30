package build

import (
	"path"
	"strings"
	"testing"
)

// NFR-02 (log-2026-09-29 M4 P14): a load parses a JSON source unchanged since a run parsed it no
// more; an edited one it parses alone, warm as cold.
func TestJSONTreesKept(t *testing.T) {
	z := archiveAnalyzer(t, loadFormsCase)
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	first := z.cache.gen.json.made
	if first == 0 {
		t.Fatal("no JSON source parsed through the cache")
	}
	name := strings.TrimPrefix(path.Join(archiveRoot, loadItemFile), "/")
	m := z.fs.base.(roFS)
	m[name] = srcFile(strings.Replace(string(m[name].Data), `"cost": 5`, `"cost": 6`, 1))
	warm, cold = z.pair(t)
	same(t, "json edited", warm, cold)
	if got := z.cache.gen.json.made; got != first+1 {
		t.Errorf("after one JSON source edited, %d parsed, want %d", got, first+1)
	}
}
