package check_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// checkText parses one file and checks it alone: Check must never panic on input.
func checkText(t *testing.T, text []byte) {
	t.Helper()
	fs := &source.FileSet{}
	src, err := fs.Add("x/x.canon", "/x/x.canon", text)
	if err != nil {
		return
	}
	f := syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, ""))
	check.Check(context.Background(), exampleProject(), []*syntax.File{f}, check.Bags{}, literalFolder{})
}

// IMPLEMENTATION-PLAN §7.7: every example with any one line deleted checks without a panic.
func TestNoPanicOnBrokenExamples(t *testing.T) {
	for _, f := range loadExamples(t).files {
		lines := strings.Split(string(f.Src.Content), "\n")
		for i := range lines {
			mut := append(append([]string{}, lines[:i]...), lines[i+1:]...)
			checkText(t, []byte(strings.Join(mut, "\n")))
		}
	}
}

// IMPLEMENTATION-PLAN §7.7: the checker never panics, seeded with the examples.
func FuzzCheck(f *testing.F) {
	err := filepath.WalkDir(examplesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != canonExt || d.Name() == projectFile {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f.Add(data)
		return nil
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, text []byte) {
		checkText(t, text)
	})
}
