package vscodegrammar

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// exampleFiles lists every .canon source of the examples, relative to examplesRoot, sorted,
// goldens excluded.
func exampleFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(examplesRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == expectedDir {
			return filepath.SkipDir
		}
		if d.IsDir() || filepath.Ext(p) != project.SourceExt {
			return nil
		}
		rel, err := filepath.Rel(examplesRoot, p)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// IMPLEMENTATION-PLAN.md 8.4 Highlighting: the grammar is snapshot-tested against every example.
func TestScopeSnapshots(t *testing.T) {
	g, err := Load(grammarPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range exampleFiles(t) {
		src := filepath.Join(examplesRoot, rel)
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			text, err := os.ReadFile(src)
			if err != nil {
				t.Fatal(err)
			}
			got, err := g.Tokenize(string(text))
			if err != nil {
				t.Fatal(err)
			}
			compareGolden(t, filepath.Join(goldenRoot, rel+goldenExt), got)
		})
	}
	checkOrphans(t)
}

// checkOrphans deletes (under -update) or reports the snapshots whose example is gone.
func checkOrphans(t *testing.T) {
	t.Helper()
	known := map[string]bool{}
	for _, rel := range exampleFiles(t) {
		known[filepath.Join(goldenRoot, rel+goldenExt)] = true
	}
	err := filepath.WalkDir(goldenRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || known[p] {
			return err
		}
		if golden.Updating() {
			return os.Remove(p)
		}
		t.Errorf("orphan snapshot, no example: %s", p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if golden.Updating() {
		removeEmptyDirs(t)
	}
}

// removeEmptyDirs deletes, deepest first, the directories an orphan left empty under goldenRoot.
func removeEmptyDirs(t *testing.T) {
	t.Helper()
	var dirs []string
	err := filepath.WalkDir(goldenRoot, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && p != goldenRoot {
			dirs = append(dirs, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if entries, err := os.ReadDir(dirs[i]); err == nil && len(entries) == 0 {
			if err := os.Remove(dirs[i]); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func compareGolden(t *testing.T, path, got string) {
	t.Helper()
	if golden.Updating() {
		if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), fileMode); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Fatalf("scope snapshot differs, run with -update and read the diff: %s", path)
	}
}

// No example highlights as invalid: the scope column of no token starts an invalid.* scope.
func TestExamplesHaveNoIllegalScopes(t *testing.T) {
	g, err := Load(grammarPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range exampleFiles(t) {
		src := filepath.Join(examplesRoot, rel)
		text, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		got, err := g.Tokenize(string(text))
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range strings.Split(got, lineSeparator) {
			if _, scopes, ok := strings.Cut(l[strings.LastIndex(l, scopeSeparator)+1:], " "); ok && strings.Contains(scopes, "invalid.") {
				t.Errorf("%s: an example highlights as invalid: %s", src, l)
			}
		}
	}
}
