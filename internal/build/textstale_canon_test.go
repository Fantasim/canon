package build_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

// staleRoot is where the out root goes, outside the project, so a stale `.canon` there is no source.
const staleRoot = "../ext"

// CODEGEN.md §2.9, DECISIONS 418, 336: a stale `.canon` text file only drops out of the list; another stale name is removed.
func TestTextStaleKeepsCanonName(t *testing.T) {
	for _, c := range []struct {
		name string
		kept bool
	}{
		{"x.canon", true},
		{"x.txt", false},
	} {
		fsys := textTree(textEmit)
		fsys["ext/sql/"+c.name] = file(c.name)
		fsys["p/a/canon.outputs"] = file(outputsFor("a", map[string]string{
			"@out/sql/Schema.sql": schemaText, "@out/sql/_version.sql": versionText, "@out/sql/" + c.name: c.name,
		}))
		p, err := build.Open(fsys, "/p", build.Options{Roots: map[string]string{"out": staleRoot}})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Build(context.Background(), build.BuildOptions{})
		if err != nil || res.Summary.Errors != 0 {
			t.Fatalf("%s: %v, %v", c.name, err, codes(res.List))
		}
		if _, kept := fsys["ext/sql/"+c.name]; kept != c.kept || string(fsys["p/a/canon.outputs"].Data) != textOutputs {
			t.Errorf("CODEGEN.md §2.9, DECISIONS 418: %s kept %v, want %v; canon.outputs %q", c.name, kept, c.kept, fsys["p/a/canon.outputs"].Data)
		}
	}
}
