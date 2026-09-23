package syntax_test

import (
	"bytes"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	_ "github.com/fantasim/canonlang/internal/testkit/golden"
)

const (
	examplesDir = "../../examples"
	astDir      = "testdata/ast"
	projectFile = "project.canon"
	findingsSep = "== findings\n"
)

// example is one .canon file of examples/, parsed.
type example struct {
	rel  string
	file *syntax.File
	bag  *diag.Bag
	fs   *source.FileSet
}

// parseText parses text as the file at path (project.canon by its base name).
func parseText(t testing.TB, path string, text []byte) (*syntax.File, *diag.Bag, *source.FileSet) {
	t.Helper()
	fs := &source.FileSet{}
	src, err := fs.Add(path, "/"+path, text)
	if err != nil {
		t.Fatal(err)
	}
	kind := syntax.FileSource
	if filepath.Base(path) == projectFile {
		kind = syntax.FileProject
	}
	bag := diag.NewBag(fs, strings.ReplaceAll(filepath.Dir(path), "/", "."))
	return syntax.Parse(src, kind, bag), bag, fs
}

// examples parses every .canon file under examples/, in path order.
func examples(t testing.TB) []example {
	t.Helper()
	var out []example
	err := filepath.WalkDir(examplesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".canon" {
			return err
		}
		rel, err := filepath.Rel(examplesDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, bag, fset := parseText(t, filepath.ToSlash(rel), data)
		out = append(out, example{rel: filepath.ToSlash(rel), file: f, bag: bag, fs: fset})
		return nil
	})
	if err != nil || len(out) == 0 {
		t.Fatalf("examples: %d files, %v", len(out), err)
	}
	return out
}

// IMPLEMENTATION-PLAN §4.1: the tokens with their trivia print every example back byte for byte.
func TestExamplesRoundTrip(t *testing.T) {
	for _, ex := range examples(t) {
		if got := printTokens(ex.file); !bytes.Equal(got, ex.file.Src.Content) {
			t.Errorf("%s: tokens and trivia do not reproduce the file", ex.rel)
		}
	}
}

// IMPLEMENTATION-PLAN §6 M1 item 1: each example's tree and findings equal its AST golden.
func TestExamplesAST(t *testing.T) {
	update := flag.Lookup("update").Value.(flag.Getter).Get().(bool)
	for _, ex := range examples(t) {
		t.Run(ex.rel, func(t *testing.T) {
			got := []byte(dumpFile(ex.file) + findingsSep + renderFindings(t, ex.bag, ex.fs))
			path := filepath.Join(astDir, filepath.FromSlash(ex.rel)+".txt")
			if update {
				writeGolden(t, path, got)
				return
			}
			want, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, want) {
				t.Errorf("%s differs from its golden (go test -update): %v\n%s", path, err, got)
			}
		})
	}
}

// GRAMMAR.md §10: every example tree holds together (see checkTree).
func TestExamplesTreeInvariants(t *testing.T) {
	for _, ex := range examples(t) {
		checkTree(t, ex.rel, ex.file, len(ex.bag.Findings()) == 0)
	}
}

func writeGolden(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, filePerm); err != nil {
		t.Fatal(err)
	}
}

// renderFindings is the text form of a bag's findings with its summary (API.md §4.4).
func renderFindings(t *testing.T, bag *diag.Bag, files diag.Files) string {
	t.Helper()
	var buf bytes.Buffer
	opt := diag.RenderOptions{Summary: bag.Summary(), Golden: true}
	if err := diag.Render(&buf, files, bag.Findings(), opt); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// printTokens writes a file's tokens with their trivia.
func printTokens(f *syntax.File) []byte {
	var out bytes.Buffer
	text := func(start, end source.Pos) { out.Write(f.Src.Content[start:end]) }
	for _, tok := range f.Tokens {
		for _, tr := range tok.Leading {
			text(tr.Start, tr.End)
		}
		text(tok.Start, tok.End)
		for _, tr := range tok.Trailing {
			text(tr.Start, tr.End)
		}
	}
	return out.Bytes()
}
