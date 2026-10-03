package canon_test

import (
	"context"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/testkit/fixture"
)

// copyRenames is a temporary copy of the renames example, with what it imports, on disk.
func copyRenames(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := fixture.CopyRenames(dir, "../examples"); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeFile writes text at rel under dir, its directories made.
func writeFile(t *testing.T, dir, rel, text string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// tree is every file under dir but the project's .canon directory, by slash path.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".canon" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// changedFiles are the files of after whose bytes differ from before's, or that one lacks.
func changedFiles(before, after map[string]string) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(before)) {
		if after[name] != before[name] {
			out = append(out, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(after)) {
		if _, ok := before[name]; !ok {
			out = append(out, name)
		}
	}
	return out
}

// openDisk opens the project in dir on the real file system.
func openDisk(t *testing.T, dir string, opts canon.Options) *canon.Project {
	t.Helper()
	opts.Cache = "off"
	p, err := canon.Open(dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// renameOnce applies RenameName(name, newName) to p alone.
func renameOnce(p *canon.Project, name, newName string) (*canon.EditResult, error) {
	return p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.RenameName(name, newName)}})
}
