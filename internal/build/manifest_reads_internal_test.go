package build

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// WIRE.md §10, VIEWMODEL.md 12.3: a build's manifest holds what the drivers-only program's host read for a view model.
func TestManifestDriversHost(t *testing.T) {
	fsys := roFS{
		"law/project.canon": srcFile("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     srcFile(internalA + "\nemit view { out: \"out/a.view.json\" }\n"),
		"law/b/b.canon": srcFile("package b\n\nimport a\n\nlet bkinds: table a.Kind = load(\"kinds.json\")\n\n" +
			"record Step {\n  kind: ref bkinds\n  target: a.Target(kind)?\n}\n\nlet steps: [Step] = []\n"),
		"law/b/kinds.json": srcFile(`{"k2": {"goal": "visit"}}`),
	}
	p, err := Open(fsys, "/law", Options{})
	if err != nil {
		t.Fatal(err)
	}
	checked, err := p.Analyze(t.Context(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	built, err := p.Build(t.Context(), BuildOptions{Packages: []string{"a"}, Check: true})
	if err != nil {
		t.Fatal(err)
	}
	const line = " b/kinds.json\n"
	if !strings.Contains(string(built.Manifest), line) {
		t.Errorf("the build's manifest lacks the drivers' read %q:\n%s", line, built.Manifest)
	}
	if strings.Contains(string(checked.Manifest()), line) {
		t.Errorf("check builds no view model, yet its manifest has %q", line)
	}
}

// WIRE.md §10 (log M4 B5-r): a root with no path relative to the project's is its hash, never a path.
func TestManifestRootWithoutRelativePath(t *testing.T) {
	sum := sha256.Sum256([]byte("/w/res"))
	for _, c := range []struct{ base, dir, want string }{
		{"/p", "/p/res", "res"},
		{"/p", "/w/res", "../w/res"},
		{"p", "/w/res", "sha256:" + hex.EncodeToString(sum[:])}, // filepath.Rel refuses a relative base against an absolute dir
	} {
		if got := relativeDir(c.base, c.dir); got != c.want {
			t.Errorf("relativeDir(%q, %q) = %q, want %q", c.base, c.dir, got, c.want)
		}
	}
}

// WIRE.md §2.3, §10: a listed folder takes the display of the asset root written first in package then source order.
func TestManifestListingFirstSite(t *testing.T) {
	item := func(head, root string) string {
		return head + "/// R.\nrecord R {\n  /// Icon.\n  icon: asset(\"" + root +
			"\", ext: [png])\n}\n\n/// Rs.\nlet rs: [R] = [{icon: \"a.png\"}]\n"
	}
	fsys := roFS{ // a forces c's list, so c's asset check lists the folder first
		"p/project.canon":   srcFile("project acme {\n  canon: \"0.1\"\n\n  roots {\n    res: \"res\"\n  }\n}\n"),
		"p/a/a.canon":       srcFile(item("/// A.\npackage a\n\nimport c\n\n/// N.\nlet n: Int = c.rs.len()\n\n", "../res/icons")),
		"p/c/c.canon":       srcFile(item("/// C.\npackage c\n\n", "@res/icons")),
		"p/res/icons/a.png": srcFile(""),
	}
	p, err := Open(fsys, "/p", Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := a.Result().Summary.Errors; n != 0 {
		t.Fatalf("%d errors: %v", n, a.Result().List)
	}
	got := string(a.Manifest())
	if !strings.Contains(got, " res/icons\n") || strings.Contains(got, " @res/icons\n") {
		t.Errorf("the listing is not displayed as package a writes it:\n%s", got)
	}
}
