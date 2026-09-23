package jsonsrc_test

import (
	"bytes"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/jsonsrc"
)

// notNormalized are the JSON sources under examples/ that no `canon fmt --json-sources` has
// normalized yet: the fixtures copy the layout of the files they trim, and these project files
// keep one row per line.
var notNormalized = []string{
	"_fixtures/resource/Server/Item/propItem.json",
	"_fixtures/resource/Server/Npc/etc.json",
	"_fixtures/resource/Server/Skill/skills.json",
	"features/codes/data/guild_rights.json",
	"features/embedded/data/countries.json",
	"features/legacycpp/data/props.json",
}

// exampleSources are the JSON files under examples/, outside the expected/ outputs.
func exampleSources(t testing.TB) map[string][]byte {
	t.Helper()
	root := filepath.Join("..", "..", "examples")
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == "expected":
			return filepath.SkipDir
		case d.IsDir() || path.Ext(d.Name()) != ".json":
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err == nil {
			files[filepath.ToSlash(rel)], err = os.ReadFile(p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// FORMATTER.md §14.1: every example source reads cleanly and a normalized one is a fixed point.
func TestExampleSources(t *testing.T) {
	files := exampleSources(t)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		p := mustParse(t, string(files[name]))
		out := jsonsrc.Format(p.root)
		again := mustParse(t, string(out))
		if !sameTree(p.root, again.root) || !bytes.Equal(jsonsrc.Format(again.root), out) {
			t.Errorf("%s: Format changes the tree or is not idempotent", name)
		}
		fixed := bytes.Equal(out, files[name])
		if want := !slices.Contains(notNormalized, name); fixed != want {
			t.Errorf("%s: fixed point %v, want %v (update notNormalized)", name, fixed, want)
		}
	}
	for _, name := range notNormalized {
		if _, ok := files[name]; !ok {
			t.Errorf("%s is listed but not found", name)
		}
	}
}

// sameTree compares two trees by kind, text, keys and pointers, not by span.
func sameTree(a, b *jsonsrc.Node) bool {
	if a.Kind != b.Kind || a.Text != b.Text || a.Pointer() != b.Pointer() ||
		len(a.Elems) != len(b.Elems) || len(a.Members) != len(b.Members) {
		return false
	}
	for i := range a.Elems {
		if !sameTree(a.Elems[i], b.Elems[i]) {
			return false
		}
	}
	for i, m := range a.Members {
		if m.Key != b.Members[i].Key || !sameTree(m.Value, b.Members[i].Value) {
			return false
		}
	}
	return true
}
