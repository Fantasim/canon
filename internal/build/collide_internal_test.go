package build

import (
	"context"
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// collideA is package a: v, emitted to out/ by a json emit, and a load in out/; collideB is
// package b: w, emitted by a json emit in file mode into out/ too.
const (
	collideA = "/// A.\npackage a\n\n/// V.\nlet v: Int = 1\n\n/// Fed.\nlet fed: String = load.text(\"@out/%s\")\n\n" +
		"emit json { out: \"@out/\", values: [v] }\n"
	collideB = "/// B.\npackage b\n\n/// W.\nlet w: Int = 2\n\nemit json { out: \"@out/%s\", values: [w] }\n"
	collideP = "project acme {\n  canon: \"0.1\"\n  roots {\n    out: \"out\"\n  }\n}\n"
)

// DECISIONS 355 and 341: a check names the files of json emits sharing a directory, for E8152 and
// E8026, from their layout: no json document is encoded, however many entries it holds.
func TestCheckEncodesNoJSONForItsNames(t *testing.T) {
	cases := []struct {
		name, loaded, file string
		e8152, e8026       int
	}{
		{"sharing, nothing collides, a load below", "note.txt", "w.json", 0, 0},
		{"a load of a json output", "v.json", "w.json", 0, 1},
		{"a file-mode emit on the directory-mode file", "note.txt", "v.json", 1, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fsys := roFS{
				"p/project.canon": srcFile(collideP),
				"p/a/a.canon":     srcFile(fmt.Sprintf(collideA, c.loaded)),
				"p/b/b.canon":     srcFile(fmt.Sprintf(collideB, c.file)),
				"p/out/note.txt":  srcFile("n\n"),
				"p/out/v.json":    srcFile("1\n"),
			}
			r := analyzedRun(t, fsys)
			got := codeCounts(r)
			if len(got) != c.e8152+c.e8026 || got[diag.E8152.Def().Code] != c.e8152 || got[diag.E8026.Def().Code] != c.e8026 {
				t.Errorf("findings %v; want %d collisions, %d loads of an output", got, c.e8152, c.e8026)
			}
			if n := len(r.writes().generated); n > 0 {
				t.Errorf("the check ran %d generators for file names", n)
			}
		})
	}
}

// analyzedRun is the run of a check of the project at /p in fsys.
func analyzedRun(t *testing.T, fsys roFS) *run {
	t.Helper()
	p, err := Open(fsys, "/p", Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	r, err := p.prepare(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.analyze(ctx); err != nil {
		t.Fatal(err)
	}
	return r
}

// codeCounts is how many findings of each code the run's bags hold.
func codeCounts(r *run) map[diag.Code]int {
	out := map[diag.Code]int{}
	for _, bag := range r.bags { //canon:unordered counted
		for _, f := range bag.Findings() {
			out[f.Code]++
		}
	}
	return out
}
