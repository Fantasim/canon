package eval_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"golang.org/x/tools/txtar"
)

// fuzzBudget keeps each fuzzed program short.
const fuzzBudget = 100_000

// IMPLEMENTATION-PLAN §7.7: evaluating any program never panics; seeds are examples and cases.
func FuzzEval(f *testing.F) {
	for _, p := range []string{"sovcommon/time/time.canon", "teamboard/taxonomy.canon", "features/matching/matching.canon"} {
		if b, err := os.ReadFile(filepath.Join(examplesDir, p)); err == nil {
			f.Add(string(b))
		}
	}
	cases, _ := filepath.Glob("testdata/*/*.txtar")
	for _, c := range cases {
		a, err := txtar.ParseFile(c)
		if err != nil {
			f.Fatal(err)
		}
		for _, file := range a.Files {
			if filepath.Ext(file.Name) == canonExt {
				f.Add(string(file.Data))
			}
		}
	}
	f.Fuzz(func(t *testing.T, src string) {
		b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(src)}), eval.Options{Budget: fuzzBudget})
		_ = runTests(t, b)
	})
}
