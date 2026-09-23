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

// Case is one golden case: a txtar archive, the path it was read from and the name of the
// archive file that holds its expected output.
type Case struct {
	Path     string
	Archive  *txtar.Archive
	Expected string
}

// Option adjusts how cases are loaded and compared.
type Option func(*config)

type config struct {
	expected string
}

// Expected names the file of the expected output, not want (IMPLEMENTATION-PLAN.md §7.2).
func Expected(name string) Option {
	return func(c *config) { c.expected = name }
}

// Load reads the cases matching a glob, sorted by path; matching none is an error.
func Load(glob string, opts ...Option) ([]Case, error) {
	cfg := config{expected: wantFile}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.expected == "" {
		return nil, errNoExpected
	}
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
		cases = append(cases, Case{Path: p, Archive: a, Expected: cfg.expected})
	}
	return cases, nil
}

// Want is the expected output: the content of the case's Expected file.
func (c Case) Want() ([]byte, error) {
	for _, f := range c.Archive.Files {
		if f.Name == c.Expected {
			return f.Data, nil
		}
	}
	return nil, fmt.Errorf("%w %s: %s", errNoWant, c.Expected, c.Path)
}

// Run runs fn on every case matching glob, as subtests named by file, and compares its
// output with the case's Expected file, or rewrites that file under -update.
func Run(t *testing.T, glob string, fn func(t *testing.T, c Case) []byte, opts ...Option) {
	t.Helper()
	cases, err := Load(glob, opts...)
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

// check compares got with the Expected file.
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

// rewrite replaces the Expected file (or adds it) and writes the archive back.
func (c Case) rewrite(got []byte) error {
	files := make([]txtar.File, 0, len(c.Archive.Files)+1)
	for _, f := range c.Archive.Files {
		if f.Name != c.Expected {
			files = append(files, f)
		}
	}
	c.Archive.Files = append(files, txtar.File{Name: c.Expected, Data: got})
	if err := os.WriteFile(c.Path, txtar.Format(c.Archive), filePerm); err != nil {
		return fmt.Errorf("golden: %w", err)
	}
	return nil
}
