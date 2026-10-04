package canon_test

import (
	"context"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// API.md §13.1, B1b, CODEGEN.md §2.9, DECISIONS 295: TargetText is accepted and its outputs carry Target "text".
func TestBuildTextTarget(t *testing.T) {
	src := "/// A.\npackage a\n\n/// A query.\n@text(\"q.sql\")\nexport fn q() -> String { return \"SELECT 1;\" }\n\nemit text { out: \"@out/sql\" }\n"
	fsys := newMemFS(map[string][]byte{"/law/project.canon": []byte(buildTestProject), "/law/a/a.canon": []byte(src)})
	p := openTierProject(t, fsys)
	res, err := p.Build(context.Background(), canon.BuildOptions{Targets: []canon.Target{canon.TargetText}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Outputs) != 2 || res.Outputs[0].Path != "@out/sql/.canon-text" || res.Outputs[1].Path != "@out/sql/q.sql" {
		t.Fatalf("outputs %+v", res.Outputs)
	}
	for _, o := range res.Outputs {
		if o.Target != canon.TargetText {
			t.Errorf("%s: target %q", o.Path, o.Target)
		}
	}
	if got := string(fsys.files["/law/out/sql/q.sql"]); got != "SELECT 1;" {
		t.Errorf("q.sql = %q", got)
	}
}
