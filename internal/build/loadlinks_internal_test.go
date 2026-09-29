package build

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

// linksSource is a package whose loads read through symbolic links, one glob matching nothing.
const linksSource = `/// A.
package a

/// An item.
record Item {
  /// Sku.
  sku: String
  /// Cost.
  cost: Int = 2
}

/// Items.
let items: [Item] keyed by sku = load.dir("@resource/items/*.json")

/// None.
let nothing: [Item] keyed by sku = load.dir("@resource/none/*.json")

/// Cfg.
record Cfg {
  /// Level.
  level: Int = 1
}

/// The settings.
let cfg: Cfg = load("@resource/cfg.json")

/// Note.
let note: String = load.text("@resource/note.txt")
`

// Where the links case writes its files.
const (
	linksProject = "project.canon"
	linksItemOut = "resource/items/out.json"
	linksCfg     = "resource/cfg.json"
	linksNone    = "resource/none/n.json"
	linksDirMode = 0o755
	linksMode    = 0o644
)

// linkTree is the links case on disk: the project directory, beside a file outside it.
type linkTree struct {
	t        *testing.T
	dir, out string
}

func (lt linkTree) write(name, data string) {
	p := filepath.Join(lt.dir, name)
	if err := os.MkdirAll(filepath.Dir(p), linksDirMode); err != nil {
		lt.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), linksMode); err != nil {
		lt.t.Fatal(err)
	}
}

// link makes name a symbolic link to target, replacing what was there.
func (lt linkTree) link(target, name string) {
	p := filepath.Join(lt.dir, name)
	_ = os.Remove(p)
	if err := os.Symlink(target, p); err != nil {
		lt.t.Skipf("no symbolic links here: %v", err)
	}
}

func (lt linkTree) remove(name string) {
	if err := os.Remove(filepath.Join(lt.dir, name)); err != nil {
		lt.t.Fatal(err)
	}
}

// newLinkTree writes the links case under a new temporary directory.
func newLinkTree(t *testing.T) linkTree {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lt := linkTree{t: t, dir: filepath.Join(root, "proj"), out: filepath.Join(root, "outside.json")}
	lt.write(linksProject, "project acme {\n  canon: \"0.1\"\n\n  roots {\n    resource: \"resource\"\n  }\n}\n")
	lt.write("a/a.canon", linksSource)
	lt.write("resource/items/i1.json", `{"sku": "i1", "cost": 5}`)
	if err := os.MkdirAll(filepath.Join(lt.dir, filepath.Dir(linksNone)), linksDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lt.out, []byte(`{"sku": "o"}`), linksMode); err != nil {
		t.Fatal(err)
	}
	lt.link(lt.out, linksItemOut)
	for name, data := range map[string]string{"resource/c1.json": `{"level": 3}`, "resource/c2.json": `{"level": 4}`, "resource/c3.json": `{"level": 3}`} { //canon:unordered files written
		lt.write(name, data)
	}
	lt.link("c1.json", linksCfg)
	lt.write("resource/note.txt", "hi\n")
	return lt
}

// linkStep changes the links case on disk; want are the codes the analysis must report.
type linkStep struct {
	name string
	do   func()
	want []diag.Code
}

// WIRE.md §6.5, IMPLEMENTATION-PLAN §7.6 (log-2026-09-29 M4 P3-r): loads through links replayed or run again, warm as cold.
func TestIncrementalLoadLinks(t *testing.T) {
	lt := newLinkTree(t)
	z := &analyzer{fs: newEditFS(project.OS()), dir: filepath.ToSlash(lt.dir), cache: NewCache()}
	w7107, w7115 := diag.W7107.Def().Code, diag.W7115.Def().Code
	nop := func() {}
	for _, st := range []linkStep{
		{"first", nop, []diag.Code{w7107, w7115}},
		{"unchanged", nop, []diag.Code{w7107, w7115}},
		{"cfg retargeted to other content", func() { lt.link("c2.json", linksCfg) }, nil},
		{"cfg retargeted to the same content", func() { lt.link("c3.json", linksCfg) }, nil},
		{"cfg back", func() { lt.link("c1.json", linksCfg) }, nil},
		{"outside link dangling", func() { lt.link(lt.out+".gone", linksItemOut) }, []diag.Code{w7107, w7115}},
		{"unchanged again", nop, []diag.Code{w7107, w7115}},
		{"glob directory gains a file", func() { lt.write(linksNone, `{"sku": "n"}`) }, []diag.Code{w7115}},
		{"glob directory loses it", func() { lt.remove(linksNone) }, []diag.Code{w7107}},
		{"link removed", func() { lt.remove(linksItemOut) }, []diag.Code{w7107}},
		{"unchanged last", nop, []diag.Code{w7107}},
	} {
		st.do()
		var warm, cold *Analysis
		n := grown(z, func() { warm, cold = z.pair(t) })
		same(t, st.name, warm, cold)
		unchangedReplays(t, st.name, warm, n)
		for _, code := range st.want {
			if !slices.ContainsFunc(warm.Result().List, func(f diag.Finding) bool { return f.Code == code }) {
				t.Errorf("%s: no %s", st.name, code)
			}
		}
	}
}
