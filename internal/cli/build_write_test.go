package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
)

const writeFailProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    out: \"out\"\n  }\n}\n"
const writeFailPackage = "/// A.\npackage a\n\nlet x: Int = 1\n\nemit json { out: \"@out/\" }\n"

// DECISIONS 201: a build's write errors name the display path (@out/x.json), the cause wrapped
// (internal/build/write.go). A read-only output directory lets place() see the output as new
// (ReadFile there is ENOENT, not a hard error) while the later write still fails.
func TestBuildWriteErrorNamesDisplayPath(t *testing.T) {
	if runtime.GOOS == windowsOS || os.Geteuid() == 0 {
		t.Skip("Unix permission bits are not enforced here (Windows ignores a directory's read-only bit; root ignores them)")
	}
	dir := t.TempDir()
	write(t, filepath.Join(dir, "project.canon"), writeFailProject)
	write(t, filepath.Join(dir, "a", "a.canon"), writeFailPackage)
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(out, 0o750) })
	var stdout, stderr bytes.Buffer
	code := cli.Main(context.Background(), []string{"build", "--project", dir}, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: dir})
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "@out/x.json: ") || strings.Contains(stderr.String(), dir) {
		t.Errorf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
