package build

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const dirTestRoot = "/law"

// lawFS is a project under dirTestRoot holding files, keyed by project-relative path.
func lawFS(files map[string]string) roFS {
	fsys := roFS{"law/project.canon": srcFile("project acme {\n  canon: \"0.1\"\n}\n")}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		fsys["law/"+name] = srcFile(files[name])
	}
	return fsys
}

// dirConflictFS is the reviewer's partial-sibling repro: a/b holds a.b's own file and a file of
// the unloaded package a, whose other declaration (N) sits in a different directory.
func dirConflictFS() roFS {
	return lawFS(map[string]string{
		"a/a.canon":   "/// A.\npackage a\n\n/// N.\nconst N = 3\n",
		"a/b/b.canon": "/// B.\npackage a.b\n",
		"a/b/extra.canon": "/// Extra.\npackage a\n\n/// R.\nrecord R {\n" +
			"  /// Cap.\n  cap: Int(0..=N) = 0\n}\n",
	})
}

// neitherOwnFS is zz/y/w holding zz and zz.y, neither being zz.y.w, the directory's own.
func neitherOwnFS() roFS {
	return lawFS(map[string]string{
		"zz/y/w/one.canon": "package zz\n",
		"zz/y/w/two.canon": "package zz.y\n",
	})
}

// threeFS is the reviewer's q/r/s holding q, q.r and q.r.s, the last the directory's own.
func threeFS() roFS {
	return lawFS(map[string]string{
		"q/r/s/a.canon": "package q\n",
		"q/r/s/b.canon": "package q.r.s\n",
		"q/r/s/c.canon": "package q.r\n",
	})
}

// TYPES.md §3.1: a/b holding a.b (its own) and a is legal; the unloaded a is never checked.
func TestDirConflictsNeverLoadSibling(t *testing.T) {
	ctx := context.Background()
	p, err := Open(dirConflictFS(), dirTestRoot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(ctx, []string{"a.b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.List) != 0 {
		t.Fatalf("findings: %v, want none", res.List)
	}
	r, err := p.prepare(ctx, []string{"a.b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.check(ctx); err != nil {
		t.Fatal(err)
	}
	if len(r.prog.Packages) != 1 || r.prog.Packages[0].Path != "a.b" {
		t.Fatalf("program packages: %v, want only a.b", r.prog.Packages)
	}
	if _, ok := r.bags["a"]; ok {
		t.Error("the unloaded sibling a got a bag")
	}
}

// TYPES.md §3.1, DECISIONS 217: E2001 is in each selection's Result iff neither is dir's own.
func TestSelectEitherSide(t *testing.T) {
	ctx := context.Background()
	rows := []struct {
		name string
		fsys func() roFS
		sel  []string
		want bool
	}{
		{"neither own, select zz", neitherOwnFS, []string{"zz"}, true},
		{"neither own, select zz.y", neitherOwnFS, []string{"zz.y"}, true},
		{"neither own, select all", neitherOwnFS, nil, true},
		{"own and ancestor, select a.b", dirConflictFS, []string{"a.b"}, false},
		{"own and ancestor, select a", dirConflictFS, []string{"a"}, false},
		{"own and ancestor, select all", dirConflictFS, nil, false},
		{"own and two ancestors, select q", threeFS, []string{"q"}, false},
		{"own and two ancestors, select q.r", threeFS, []string{"q.r"}, false},
		{"own and two ancestors, select q.r.s", threeFS, []string{"q.r.s"}, false},
		{"own and two ancestors, select all", threeFS, nil, false},
	}
	code := diag.E2001.Def().Code
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			p, err := Open(row.fsys(), dirTestRoot, Options{})
			if err != nil {
				t.Fatal(err)
			}
			res, err := p.Check(ctx, row.sel)
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			got := slices.ContainsFunc(res.List, func(f diag.Finding) bool { return f.Code == code })
			if got != row.want || !row.want && len(res.List) != 0 {
				t.Errorf("%s reported %v, want %v; findings %v", code, got, row.want, res.List)
			}
		})
	}
}
