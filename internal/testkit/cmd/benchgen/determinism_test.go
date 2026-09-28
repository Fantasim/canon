package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// determinismN is small enough to run every time, large enough to draw every field kind once.
const determinismN = 40

// TestDeterminism proves the same seed and the same n always write the same bytes.
func TestDeterminism(t *testing.T) {
	a := filepath.Join(t.TempDir(), "a")
	b := filepath.Join(t.TempDir(), "b")
	if err := generate(7, a, determinismN); err != nil {
		t.Fatalf("generate a: %v", err)
	}
	if err := generate(7, b, determinismN); err != nil {
		t.Fatalf("generate b: %v", err)
	}
	assertSameTree(t, a, b)
}

// assertSameTree fails the test at the first path or byte difference between a and b.
func assertSameTree(t *testing.T, a, b string) {
	t.Helper()
	relA, relB := relFiles(t, a), relFiles(t, b)
	if len(relA) != len(relB) {
		t.Fatalf("file counts differ: %d vs %d", len(relA), len(relB))
	}
	for i, rel := range relA {
		if rel != relB[i] {
			t.Fatalf("file %d differs: %s vs %s", i, rel, relB[i])
		}
		ca, err := os.ReadFile(filepath.Join(a, rel))
		if err != nil {
			t.Fatal(err)
		}
		cb, err := os.ReadFile(filepath.Join(b, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(ca, cb) {
			t.Fatalf("%s differs between runs", rel)
		}
	}
}

// relFiles is every file under dir, project-relative, sorted (byte order: DOCTRINE.md §5).
func relFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}
