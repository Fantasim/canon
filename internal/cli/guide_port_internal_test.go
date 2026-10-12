package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	portTopic      = "port"
	portPrefix     = "guide/legacyport/"
	portExampleDir = "../../examples/guide/legacyport"
	portGoldenDir  = "expected"
)

// DECISIONS 3, 21: the project of `canon guide port` is examples/guide/legacyport, file for file.
func TestGuidePortIsTheExample(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range allFences(t) {
		rel, ok := strings.CutPrefix(f.arg, portPrefix)
		if f.topic != portTopic || !ok {
			continue
		}
		seen[rel] = true
		want, err := os.ReadFile(filepath.Join(portExampleDir, filepath.FromSlash(rel)))
		switch {
		case err != nil:
			t.Errorf("%s: %v", f, err)
		case string(want) != f.body:
			t.Errorf("%s: differs from %s", f, filepath.Join(portExampleDir, rel))
		}
	}
	err := filepath.WalkDir(portExampleDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == portGoldenDir {
			return fs.SkipDir
		}
		rel, relErr := filepath.Rel(portExampleDir, path)
		if relErr != nil || d.IsDir() || seen[filepath.ToSlash(rel)] {
			return relErr
		}
		t.Errorf("%s is in the example but not in `canon guide port`", rel)
		return nil
	})
	if err != nil || len(seen) == 0 {
		t.Fatalf("walk: %v (%d files in the guide)", err, len(seen))
	}
}
