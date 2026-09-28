package build

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// I18N.md W1, VIEWMODEL.md N4: an unselected import's own bag stays clean of these two, unlike
// the Result (DECISIONS 196) or Analysis.Bag (VIEWMODEL.md J15), which hide it either way.
func TestUnselectedImportNoW1701OrW1640(t *testing.T) {
	ctx := context.Background()
	fsys := roFS{
		"law/project.canon": srcFile("project acme {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n"),
		"law/b/b.canon":     srcFile("/// B.\npackage b\n\n/// M.\nlet m: Int = 1\n\nemit view { out: \"out/b.view.json\" }\n"),
		"law/a/a.canon":     srcFile("/// A.\npackage a\n\nimport b\n\n/// N.\nlet n: Int = b.m\n"),
	}
	p, err := Open(fsys, "/law", Options{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.prepare(ctx, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.check(ctx); err != nil {
		t.Fatal(err)
	}
	bag := r.bags["b"]
	if bag == nil {
		t.Fatal("no bag for b")
	}
	silenced := []diag.Code{diag.W1701.Def().Code, diag.W1640.Def().Code}
	for _, f := range bag.Findings() {
		if slices.Contains(silenced, f.Code) {
			t.Errorf("b (imported, not selected) must draw neither of %v: %s %q", silenced, f.Code, f.Message)
		}
	}
}
