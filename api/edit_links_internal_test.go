package canon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// The link cases, on the OS: a reads b's JSON file through a link, or through a root that aliases
// b's; b reads it by its own name.
const (
	linksDirPerm  = 0o750
	linksFilePerm = 0o600
	linksAHeavy   = "\n/// As costly as ys says.\nlet heavy: Int = [i for i in 0..ys[0]].len()\n"
	linksProject  = "project acme {\n  canon: \"0.1\"\n  budget: %d\n%s}\n"
	linksRoots    = "  roots {\n    shared: \"shared\"\n    alias: \"alias\"\n  }\n"
)

// API.md E17, E18 (log-2026-09-29 M4 P14-r3): a package owns the files it reads by their real
// paths, so an edit of b's JSON file re-checks a when a reads it through a link, or through another
// root aliasing its directory, and the edit's findings hold a's.
func TestEditRechecksLinkedLoaders(t *testing.T) {
	for _, c := range []struct {
		name, roots, aLoad, bLoad, file string
		link                            [2]string // a link to make, its name and its target
	}{
		{name: "link", aLoad: "data.json", bLoad: "b.json", file: "b/b.json", link: [2]string{"a/data.json", "../b/b.json"}},
		{name: "aliased root", roots: linksRoots, aLoad: "@alias/b.json", bLoad: "@shared/b.json", file: "shared/b.json", link: [2]string{"alias", "shared"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			linksWrite(t, dir, map[string]string{
				"project.canon": fmt.Sprintf(linksProject, readersBudget, c.roots),
				"a/a.canon":     "/// A.\npackage a\n\n/// B's numbers.\nlet ys: [Int] = load(\"" + c.aLoad + "\")\n" + linksAHeavy,
				"b/b.canon":     "/// B.\npackage b\n\n/// Numbers.\nlet xs: [Int] = load(\"" + c.bLoad + "\")\n",
				c.file:          sweepJSON0,
			})
			if err := os.Symlink(c.link[1], filepath.Join(dir, filepath.FromSlash(c.link[0]))); err != nil {
				t.Skipf("no symbolic links here: %v", err)
			}
			linksCheck(t, dir)
		})
	}
}

// linksWrite writes files below dir.
func linksWrite(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files { //canon:unordered each file is written alone
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), linksDirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), linksFilePerm); err != nil {
			t.Fatal(err)
		}
	}
}

// linksCheck edits b's first number twice and fails unless the last edit's findings hold a's
// exhausted budget.
func linksCheck(t *testing.T, dir string) {
	t.Helper()
	p, err := Open(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	var res *EditResult
	for _, n := range []int64{readersSmall, readersBig} {
		if res, err = p.Edit(context.Background(), Edit{Ops: []Op{Set(readersSet, Int(n))}, AllowErrors: true, Normalize: true}); err != nil || !res.Applied {
			t.Fatalf("Edit %d: %v", n, err)
		}
	}
	if !slices.ContainsFunc(res.Findings, func(f Finding) bool { return f.Package == "a" && f.Code == string(diag.E4401.Def().Code) }) {
		t.Errorf("API.md E18: the edit's findings lack a's exhausted budget: %+v", res.Findings)
	}
}
