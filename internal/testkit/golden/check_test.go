package golden

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/txtar"
)

func tempCase(t *testing.T, content string, opts ...Option) Case {
	t.Helper()
	path := filepath.Join(t.TempDir(), "case.txtar")
	if err := os.WriteFile(path, []byte(content), filePerm); err != nil {
		t.Fatal(err)
	}
	cases, err := Load(path, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return cases[0]
}

// IMPLEMENTATION-PLAN.md 7.1: goldens compare byte for byte.
func TestCheckFailsOnADifference(t *testing.T) {
	c := tempCase(t, "-- want --\nx\n")
	if err := c.check([]byte("x\n")); err != nil {
		t.Errorf("equal output: %v", err)
	}
	if err := c.check([]byte("x")); !errors.Is(err, errDiffers) {
		t.Errorf("got %v, want %v", err, errDiffers)
	}
	if err := tempCase(t, "-- in --\nx\n").check(nil); !errors.Is(err, errNoWant) {
		t.Errorf("got %v, want %v", err, errNoWant)
	}
}

// IMPLEMENTATION-PLAN.md 7.1: -update rewrites the want file and keeps the inputs.
func TestRewriteReplacesTheWantFile(t *testing.T) {
	c := tempCase(t, "comment\n-- in --\nx\n-- want --\nold\n")
	if err := c.rewrite([]byte("new\n")); err != nil {
		t.Fatal(err)
	}
	a, err := txtar.ParseFile(c.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(txtar.Format(a)); got != "comment\n-- in --\nx\n-- want --\nnew\n" {
		t.Errorf("rewritten archive:\n%s", got)
	}
}

// IMPLEMENTATION-PLAN.md §7.2: Expected("findings.txt") compares and rewrites that file only.
func TestExpectedNamesTheFile(t *testing.T) {
	const archive = "-- want --\nin\n-- findings.txt --\nold\n"
	c := tempCase(t, archive, Expected("findings.txt"))
	if err := c.check([]byte("old\n")); err != nil {
		t.Errorf("equal output: %v", err)
	}
	if err := c.check([]byte("in\n")); !errors.Is(err, errDiffers) {
		t.Errorf("got %v, want %v", err, errDiffers)
	}
	if err := c.rewrite([]byte("new\n")); err != nil {
		t.Fatal(err)
	}
	a, err := txtar.ParseFile(c.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(txtar.Format(a)); got != "-- want --\nin\n-- findings.txt --\nnew\n" {
		t.Errorf("rewritten archive:\n%s", got)
	}
	if err := tempCase(t, archive, Expected("other")).check(nil); !errors.Is(err, errNoWant) {
		t.Errorf("got %v, want %v", err, errNoWant)
	}
	if _, err := Load(c.Path, Expected("")); !errors.Is(err, errNoExpected) {
		t.Errorf("got %v, want %v", err, errNoExpected)
	}
}

func TestLoadMatchingNothingFails(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "*.txtar")); !errors.Is(err, errNoCases) {
		t.Errorf("got %v, want %v", err, errNoCases)
	}
}
