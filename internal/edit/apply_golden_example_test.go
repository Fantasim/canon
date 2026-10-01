package edit_test

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"
)

// goldenExamples holds the examples an edit golden's `example:` option names.
const goldenExamples = "../../examples"

// generatedDirs are the top-level directories of an example holding the compiler's output.
var generatedDirs = []string{"expected", "out"}

// loadExample makes the sources of c's example the files before (IMPLEMENTATION-PLAN §7.9).
func (c *goldenCase) loadExample(t *testing.T) {
	t.Helper()
	root := filepath.Join(goldenExamples, filepath.FromSlash(c.example))
	err := fs.WalkDir(os.DirFS(root), ".", func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && slices.Contains(generatedDirs, p):
			return fs.SkipDir
		case d.IsDir():
			return nil
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			return err
		}
		name := "law/" + path.Join(c.example, p)
		if _, taken := c.fsys[name]; taken { // an archive file may not shadow one
			t.Fatalf("example %s: the archive holds %s, a file of the example", c.example, name)
		}
		c.fsys[name] = file(string(b))
		return nil
	})
	if err != nil {
		t.Fatalf("example %s: %v", c.example, err)
	}
}
