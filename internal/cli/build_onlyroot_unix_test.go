//go:build unix

package cli_test

import (
	"bytes"
	"context"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
	"github.com/fantasim/canonlang/internal/diag"
)

const (
	onlyRootProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    out: \"out\"\n    admin: \"../admin\"\n  }\n" +
		"  consumer_roots: [admin]\n  go_module {\n    out: \"example.com/out\"\n    admin: \"example.com/admin\"\n  }\n}\n"
	onlyRootSource = "/// A.\npackage a\n\n/// Tier.\nrecord Tier {\n  /// Weight.\n  weight: Int = 1\n}\n\n" +
		"/// Tiers.\nlet tiers: stable table Tier = { low {} }\n\nemit go { out: [\"@out/go/a\", \"@admin/go/a\"] }\n"
	readOnlyFile = 0o444
	readOnlyDir  = 0o555
	writableFile = 0o644
	writableDir  = 0o755
)

// tree is every file under dir by slash path relative to it, with its content.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(name)
		rel, _ := filepath.Rel(dir, name)
		out[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// setModes makes every file and directory under dir file or dir; the project's tree read-only, or writable again.
func setModes(t *testing.T, dir string, file, sub fs.FileMode) {
	t.Helper()
	err := filepath.WalkDir(dir, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.Chmod(name, sub)
		}
		return os.Chmod(name, file)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// DECISIONS 343 (Sovereign handoff 2026-10-08 item 2): with the consumer root @admin absent,
// build and build --check --max-warnings 0 pass; then, the project read-only,
// build --only-root admin --root admin=<dir> creates <dir> and writes exactly the Go copy there.
func TestOnlyRootFromReadOnlyCheckout(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes through read-only modes")
	}
	tmp := t.TempDir()
	project := filepath.Join(tmp, "p")
	writeFiles(t, project, map[string]string{"project.canon": onlyRootProject, "a/a.canon": onlyRootSource})
	canonOK(t, project, "build")
	canonOK(t, project, "build", "--check", "--max-warnings", "0")
	if _, err := os.Stat(filepath.Join(tmp, "admin")); !os.IsNotExist(err) {
		t.Fatalf("a plain build created the consumer root: %v", err)
	}
	before := tree(t, project)
	setModes(t, project, readOnlyFile, readOnlyDir)
	t.Cleanup(func() { setModes(t, project, writableFile, writableDir) })
	missing := filepath.Join(tmp, "nowhere", "a") // its parent missing: E1013, exit 1, nothing created
	var stdout, stderr bytes.Buffer
	args := []string{"build", "--only-root", "admin", "--root", "admin=" + filepath.ToSlash(missing)}
	if code := cli.Main(context.Background(), args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: project}); code != 1 || !strings.Contains(stdout.String(), "["+string(diag.E1013.Def().Code)+"]") {
		t.Fatalf("parent missing: exit %d, stdout %q", code, stdout.String())
	}
	if _, err := os.Stat(filepath.Dir(missing)); !os.IsNotExist(err) {
		t.Fatalf("--only-root created a root's parent: %v", err)
	}
	admin := filepath.Join(tmp, "a") // absent, its parent present: created
	canonOK(t, project, "build", "--only-root", "admin", "--root", "admin="+filepath.ToSlash(admin))
	if after := tree(t, project); !maps.Equal(before, after) {
		t.Error("--only-root changed the project")
	}
	got := slices.Sorted(maps.Keys(tree(t, admin)))
	if want := []string{"go/a/a.gen.go", "go/a/rt/rt.go"}; !slices.Equal(got, want) {
		t.Errorf("written under the consumer root: %v, want %v", got, want)
	}
	if out := tree(t, project); out["out/go/a/a.gen.go"] == "" {
		t.Error("the plain build did not write @out")
	}
}

// writeFiles writes files under dir, creating their directories.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for _, name := range slices.Sorted(maps.Keys(files)) {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), writableDir); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(files[name]), writableFile); err != nil {
			t.Fatal(err)
		}
	}
}
