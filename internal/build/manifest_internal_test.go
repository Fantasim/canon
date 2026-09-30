package build

import (
	"bytes"
	"path"
	"strings"
	"testing"
)

const manifestLineage = "testdata/manifest/inputs.txtar"

// WIRE.md §10, IMPLEMENTATION-PLAN §7.6: along an edit lineage, warm manifests are cold ones, changed by an input only.
func TestManifestWarmEqualsCold(t *testing.T) {
	z := archiveAnalyzer(t, manifestLineage)
	z.opt.Layers = []string{"dev"}
	m := z.fs.base.(roFS)
	put := func(name, data string) func() {
		return func() { m[strings.TrimPrefix(path.Join(archiveRoot, name), "/")] = srcFile(data) }
	}
	warm, cold := z.pair(t)
	same(t, "first", warm, cold)
	prev := warm.Manifest()
	for _, st := range []struct {
		name    string
		do      func()
		changed bool
	}{
		{"unchanged", func() {}, false},
		{"unread file edited", put("res/unread.json", "[2]"), false},
		{"item edited", put("res/items/a.json", `{"name": "a2", "icon": "a.png"}`), true},
		{"item added", put("res/items/c.json", `{"name": "c", "icon": "sub/b.png"}`), true},
		{"icon added", put("res/icons/sub/f.png", ""), true},
		{"conf edited", put("items/data/conf.json", `{"limit": 9}`), true},
		{"unchanged again", func() {}, false},
	} {
		st.do()
		warm, cold = z.pair(t)
		same(t, st.name, warm, cold)
		if !st.changed && warm.r.ev.LoadsReplayed() == 0 {
			t.Errorf("%s: no load replayed: the warm path is not exercised", st.name)
		}
		got := warm.Manifest()
		if !bytes.Equal(got, prev) != st.changed {
			t.Errorf("%s: manifest changed %t, want %t:\n%s", st.name, !bytes.Equal(got, prev), st.changed, got)
		}
		prev = got
	}
}

// WIRE.md §2.3, §10: a file read under two forms keeps its first site's in package order, though b's load ran first.
func TestManifestFirstReference(t *testing.T) {
	fsys := roFS{
		"p/project.canon":     srcFile("project acme {\n  canon: \"0.1\"\n\n  roots {\n    res: \"res\"\n  }\n}\n"),
		"p/a/a.canon":         srcFile("/// A.\npackage a\n\nimport b { Item, items }\n\n/// N.\nlet n: Int = items.len()\n\n/// One.\nlet one: Item = load(\"../res/items/a.json\")\n"),
		"p/b/b.canon":         srcFile("/// B.\npackage b\n\n/// An item.\nrecord Item {\n  /// Name.\n  name: String\n}\n\n/// Items.\nlet items: [Item] = load.dir(\"@res/items/*.json\")\n"),
		"p/res/items/a.json":  srcFile(`{"name": "a"}`),
		"p/res/items/zz.json": srcFile(`{"name": "z"}`),
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
	for _, want := range []string{" res/items/a.json\n", " @res/items/zz.json\n", " @res/items/*.json\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("no line ending %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, " @res/items/a.json\n") {
		t.Errorf("a.json keeps the form b's later site gave it:\n%s", got)
	}
}
