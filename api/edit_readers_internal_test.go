package canon

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// The loader case: a loads b's JSON file and spends as much as its first number says.
const (
	readersBudget = 200
	readersA      = "/// A.\npackage a\n\n/// B's numbers.\nlet ys: [Int] = load(\"../b/b.json\")\n\n/// As costly as ys says.\nlet heavy: Int = [i for i in 0..ys[0]].len()\n"
	readersB      = "/// B.\npackage b\n\n/// Numbers.\nlet xs: [Int] = load(\"b.json\")\n"
	readersSet    = "b:xs[0]"
	readersSmall  = 2
	readersBig    = 1000
)

// API.md E17, E18 (log-2026-09-29 M4 P14-r2): a package owns every file it reads, so an edit of
// b's JSON file re-checks a, which loads it too, and its findings hold a's; Value and Evaluate,
// qualified or not, then agree.
func TestEditRechecksLoaders(t *testing.T) {
	fsys := newWriteFS(map[string][]byte{
		sweepRoot + "/project.canon": fmt.Appendf(nil, sweepProject, readersBudget),
		sweepRoot + "/a/a.canon":     []byte(readersA), sweepRoot + "/b/b.canon": []byte(readersB), sweepRoot + "/b/b.json": []byte(sweepJSON0),
	}, nil)
	p, err := Open(sweepRoot, Options{FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	ctx := context.Background()
	var res *EditResult
	for _, n := range []int64{readersSmall, readersBig} {
		if res, err = p.Edit(ctx, Edit{Ops: []Op{Set(readersSet, Int(n))}, AllowErrors: true, Normalize: true}); err != nil || !res.Applied {
			t.Fatalf("Edit %d: %v", n, err)
		}
	}
	if !slices.ContainsFunc(res.Findings, func(f Finding) bool { return f.Package == "a" && f.Code == string(diag.E4401.Def().Code) }) {
		t.Errorf("API.md E18: the edit's findings lack a's exhausted budget: %+v", res.Findings)
	}
	_, verr := p.Value(ctx, "b:xs")
	for _, path := range []string{"b:xs", "xs"} {
		if _, err := p.Evaluate(ctx, EvalRequest{Path: path}); (err == nil) != (verr == nil) {
			t.Errorf("API.md V13: Evaluate(%s): %v, Value(b:xs): %v", path, err, verr)
		}
	}
}
