package canon_test

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// sharedWarmSrc is p: xs, built by its own evaluation, repeats the entry bad and shares the list
// ws among its entries; r holds ws twice below a record, and zs repeats a local let's records.
const sharedWarmSrc = "package p\n\nenum Element { fire, retired wind }\n\ntype Es = [Element]\n\n" +
	"record Box {\n  e: Element\n  es: Es = []\n}\n\nrecord R {\n  a: Es\n  b: Es\n  boxes: [Box]\n}\n\n" +
	"fn mk() -> [Box] {\n  let ws: Es = [wind]\n  let good: Box = { e: fire, es: ws }\n  let bad: Box = { e: wind }\n" +
	"  return [good, good, bad, { e: fire, es: ws }, good, bad]\n}\n\nlet xs: [Box] = mk()\n\n" +
	"local let ws: Es = [wind]\n\nlocal let bad: Box = { e: wind, es: ws }\n\n" +
	"let r: R = { a: ws, b: ws, boxes: [bad, bad] }\n\nlet zs: [Box] = [bad, bad, { e: fire, es: ws }]\n"

// sharedWarmView is p's findings, code, path and message, sorted.
func sharedWarmView(t *testing.T, p *canon.Project) string {
	t.Helper()
	res, err := p.Check(context.Background(), "p")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range res.Findings {
		out = append(out, f.Code+" "+f.Path+" "+f.Message)
	}
	slices.Sort(out)
	return strings.Join(out, "\n")
}

// API.md W7, DECISIONS 509: a check after an edit of another file, which replays the entries of
// xs and zs from the memo, reports what a cold check reports, though their entries repeat and
// share values a walk verifies once.
func TestSharedVerifyWarmEqualsCold(t *testing.T) {
	law := maps.Clone(editLaw)
	law[srcMain], law["p/n.canon"] = sharedWarmSrc, "package p\n\nlet n: Int = 1\n"
	opts := project(law)
	warm, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = warm.Close() })
	cold := sharedWarmView(t, warm)
	if retired := string(diag.E3506.Def().Code); !strings.Contains(cold, retired) {
		t.Fatalf("EVALUATION.md §5: cold findings lack %s:\n%s", retired, cold)
	}
	for step := 2; step <= 3; step++ {
		if _, err := warm.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{canon.Set("p:n", canon.Int(int64(step)))}}); err != nil {
			t.Fatalf("API.md E19: %v", err)
		}
		if got := sharedWarmView(t, warm); got != cold {
			t.Errorf("API.md W7: after edit %d warm\n%s\ncold\n%s", step, got, cold)
		}
	}
}
