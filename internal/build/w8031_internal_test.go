package build

import (
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const (
	w8031Case = "testdata/incremental/w8031.txtar"
	w8031Row  = "a/rows/two.canon"
)

// ADR-0021 §3 point 14, DECISIONS 379
func TestW8031ColdWarm(t *testing.T) {
	z := archiveAnalyzer(t, w8031Case)
	row := path.Join(archiveRoot, w8031Row)
	steps := []struct{ name, from, to string }{
		{"first", "", ""},
		{"label edited", `"two"`, `"zwei"`},
		{"label put back", `"zwei"`, `"two"`},
	}
	for _, s := range steps {
		if s.from != "" {
			data, err := z.fs.ReadFile(row)
			if err != nil {
				t.Fatal(err)
			}
			z.fs.set(row, []byte(strings.Replace(string(data), s.from, s.to, 1)))
		}
		warm, cold := z.pair(t)
		same(t, s.name, warm, cold)
		if countCode(warm, diag.W8031.Def().Code) == 0 || countCode(cold, diag.W8031.Def().Code) == 0 {
			t.Fatalf("%s: no position warning, warm %v cold %v", s.name, warm.Result().List, cold.Result().List)
		}
	}
}
