package build

import (
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const tiersCase = "testdata/incremental/tiers.txtar"

// IMPLEMENTATION-PLAN §7.6, API.md S2, TYPES.md §3.1, DECISIONS 380
func TestTiersFollowProjectCanon(t *testing.T) {
	z := archiveAnalyzer(t, tiersCase)
	entry := path.Join(archiveRoot, "pair/items/one.canon")
	project := path.Join(archiveRoot, "project.canon")
	edit := func(file, from, to string) {
		data, err := z.fs.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		z.fs.set(file, []byte(strings.Replace(string(data), from, to, 1)))
	}
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	if countCode(warm, diag.E2007.Def().Code) == 0 {
		t.Fatalf("first: no tier finding: %v", warm.Result().List)
	}
	steps := []struct {
		name    string
		do      func()
		want    int
		lineage bool // the warm analysis continues its predecessor's epoch (the Recheck path)
	}{
		{"entry edited", func() { edit(entry, "n: 1", "n: 2") }, 1, true},
		{"entry edited again", func() { edit(entry, "n: 2", "n: 3") }, 1, true},
		{"pair moved", func() { edit(project, "low: [pair]\n    high: [same]", "low: [same]\n    high: [pair]") }, 0, false},
	}
	for _, s := range steps {
		prev := warm.r.epoch
		s.do()
		warm, cold = z.pair(t)
		same(t, s.name, warm, cold)
		if got := countCode(warm, diag.E2007.Def().Code); got != s.want {
			t.Errorf("%s: %d tier findings, want %d: %v", s.name, got, s.want, warm.Result().List)
		}
		if kept := warm.r.epoch == prev; kept != s.lineage {
			t.Errorf("%s: epoch kept %v, want %v", s.name, kept, s.lineage)
		}
	}
}
