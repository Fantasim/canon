package format_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	examplesDir = "../../examples"
	projectFile = "project.canon"
	canonExt    = ".canon"
)

// parsed is one input, parsed.
type parsed struct {
	file *syntax.File
	bag  *diag.Bag
}

// parse parses text as the file at path (project.canon by its base name).
func parse(t testing.TB, path string, text []byte) parsed {
	t.Helper()
	fset := &source.FileSet{}
	src, err := fset.Add(path, "/"+path, text)
	if err != nil {
		t.Fatal(err)
	}
	bag := diag.NewBag(fset, "p")
	return parsed{file: syntax.Parse(src, kindOf(path), bag), bag: bag}
}

func kindOf(path string) syntax.FileKind {
	if filepath.Base(path) == projectFile {
		return syntax.FileProject
	}
	return syntax.FileSource
}

// formatText formats text as the file at path.
func formatText(t testing.TB, path string, text []byte) ([]byte, error) {
	t.Helper()
	fset := &source.FileSet{}
	src, err := fset.Add(path, "/"+path, text)
	if err != nil {
		t.Fatal(err)
	}
	return format.Source(src, kindOf(path), diag.NewBag(fset, "p"))
}

// example is one .canon file under examples/.
type example struct {
	path string
	data []byte
	from string // the golden case file of a corpus input, whose path is not unique
}

// key names the input for sampling: its path, under the case file of a corpus input.
func (e example) key() string { return e.from + e.path }

// exampleFiles are the .canon files under examples/, in path order.
func exampleFiles(t testing.TB) []example {
	t.Helper()
	var out []example
	err := filepath.WalkDir(examplesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != canonExt {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(examplesDir, path)
		out = append(out, example{path: filepath.ToSlash(rel), data: data})
		return err
	})
	if err != nil || len(out) == 0 {
		t.Fatalf("examples: %d files, %v", len(out), err)
	}
	return out
}

// FORMATTER.md §15 and IMPLEMENTATION-PLAN §6 M4 item 1: every example is a fixed point.
func TestExamplesAreFixedPoints(t *testing.T) {
	for _, ex := range exampleFiles(t) {
		t.Run(ex.path, func(t *testing.T) {
			got, err := formatText(t, ex.path, ex.data)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, ex.data) {
				t.Errorf("not a fixed point:\n%s", lineDiff(ex.data, got))
			}
		})
	}
}
