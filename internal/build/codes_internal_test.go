package build

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// TYPES.md §8.1: a @codes enum's code used twice is checked for every loaded package, imports included.
func TestVerifyCodesCoversImportedPackages(t *testing.T) {
	ctx := context.Background()
	fsys := roFS{
		"law/project.canon": srcFile("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     srcFile("/// A.\npackage a\n\nimport b\n"),
		"law/b/b.canon":     srcFile("/// B.\npackage b\n\n/// E.\nenum E @codes(UInt8) { x = 1, y = 1 }\n"),
	}
	p, err := Open(fsys, "/law", Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(ctx, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	want := diag.E3102.Def().Code
	found := false
	for _, f := range res.List {
		if f.Code == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("no %s for the imported package's enum, got %v", want, res.List)
	}
}
