package build_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

// DECISIONS 356, CODEGEN.md §2.4: a stray is W8030 in a plain build, E8029 under --check, never deleted.
func TestStrayNeverDeleted(t *testing.T) {
	const stray = "{\n  \"$schema\": \"a.Old@0123abcd\",\n  \"value\": 1\n}\n"
	src := "/// A.\npackage a\n\n/// V.\nlet v: Int = 1\n\nemit json { out: \"@out/\" }\n"
	fsys := mapFS{"p/project.canon": file(textProject), "p/a/a.canon": file(src), "p/out/old.json": file(stray)}
	for _, c := range []struct {
		check bool
		want  diag.Code
	}{
		{false, diag.W8030.Def().Code},
		{true, diag.E8029.Def().Code},
		{false, diag.W8030.Def().Code},
	} {
		res := buildTree(t, fsys, build.BuildOptions{Check: c.check})
		if got := codes(res.List); !slices.Equal(got, []diag.Code{c.want}) || string(fsys["p/out/old.json"].Data) != stray {
			t.Errorf("DECISIONS 356: --check %v: %v, the file %q", c.check, got, fsys["p/out/old.json"].Data)
		}
	}
}
