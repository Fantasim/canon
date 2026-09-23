package golden

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/txtar"
)

var update = flag.Bool(updateFlag, false, updateUsage)

// Case is one golden case: a txtar archive and the path it was read from.
type Case struct {
	Path    string
	Archive *txtar.Archive
}

// Load reads the cases matching a glob, sorted by path; matching none is an error.
func Load(glob string) ([]Case, error) {
	paths, err := filepath.Glob(glob)
	if err != nil {
		return nil, fmt.Errorf("golden: %w", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("%w: %s", errNoCases, glob)
	}
	cases := make([]Case, 0, len(paths))
	for _, p := range paths {
		a, err := txtar.ParseFile(p)
		if err != nil {
			return nil, fmt.Errorf("golden: %w", err)
		}
		cases = append(cases, Case{Path: p, Archive: a})
	}
	return cases, nil
}

// Want is the expected output: the content of the case's want file.
func (c Case) Want() ([]byte, error) {
	for _, f := range c.Archive.Files {
		if f.Name == wantFile {
			return f.Data, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", errNoWant, c.Path)
}

// Run runs fn on every case matching glob, as subtests named by file, and compares its
// output with the case's want file, or rewrites that file under -update.
func Run(t *testing.T, glob string, fn func(t *testing.T, c Case) []byte) {
	t.Helper()
	cases, err := Load(glob)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(filepath.Base(c.Path), func(t *testing.T) {
			check := c.check
			if *update {
				check = c.rewrite
			}
			if err := check(fn(t, c)); err != nil {
				t.Error(err)
			}
		})
	}
}

// check compares got with the want file.
func (c Case) check(got []byte) error {
	want, err := c.Want()
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("%w: %s\n--- want\n%s\n--- got\n%s", errDiffers, c.Path, want, got)
	}
	return nil
}

// rewrite replaces the want file (or adds it) and writes the archive back.
func (c Case) rewrite(got []byte) error {
	files := make([]txtar.File, 0, len(c.Archive.Files)+1)
	for _, f := range c.Archive.Files {
		if f.Name != wantFile {
			files = append(files, f)
		}
	}
	c.Archive.Files = append(files, txtar.File{Name: wantFile, Data: got})
	if err := os.WriteFile(c.Path, txtar.Format(c.Archive), filePerm); err != nil {
		return fmt.Errorf("golden: %w", err)
	}
	return nil
}
