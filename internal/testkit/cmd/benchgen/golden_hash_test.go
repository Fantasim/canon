package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// hashSeed and hashN are the golden hash's fixed generation input.
const (
	hashSeed = 7
	hashN    = 40
)

// hashPath is the pinned sha256 of the sorted seed-7, n=40 tree; go test -update rewrites it.
const hashPath = "testdata/hash.sha256"

var update = flag.Bool("update", false, "rewrite "+hashPath+" instead of comparing it")

// TestGoldenHash pins a sha256 of the sorted tree, so the benchmark project cannot change
// silently: run `go test -run TestGoldenHash -update` after a deliberate change.
func TestGoldenHash(t *testing.T) {
	out := filepath.Join(t.TempDir(), "bench")
	if err := generate(hashSeed, out, hashN); err != nil {
		t.Fatalf("generate: %v", err)
	}
	got := treeHash(t, out)
	if *update {
		if err := os.WriteFile(hashPath, []byte(got+"\n"), filePerm); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(hashPath)
	if err != nil {
		t.Fatalf("%v (run -update after a deliberate change)", err)
	}
	if string(want) != got+"\n" {
		t.Errorf("hash of seed %d, n=%d changed:\n got  %s\n want %s(run -update if intentional)",
			hashSeed, hashN, got, want)
	}
}

// treeHash hashes every file's project-relative path and content, in sorted order.
func treeHash(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	for _, rel := range relFiles(t, dir) {
		content, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(h, "%s\n", rel)
		h.Write(content)
	}
	return hex.EncodeToString(h.Sum(nil))
}
