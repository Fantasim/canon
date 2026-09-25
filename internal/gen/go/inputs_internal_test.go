package gogen

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/txtar"
)

// TestE8302Message: e8302 renders testdata/findings/E8302_1.txtar's findings.txt exactly.
func TestE8302Message(t *testing.T) {
	paths, err := filepath.Glob("testdata/findings/E8302_*.txtar")
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob testdata/findings/E8302_*.txtar: %v", err)
	}
	for _, p := range paths {
		a, err := txtar.ParseFile(p)
		if err != nil {
			t.Fatal(err)
		}
		want := findingsOf(a)
		code, msg := e8302("a.Gen.apiKey")
		got := "panic: " + code + ": " + msg + "\n"
		if got != want {
			t.Errorf("%s: e8302 = %q, want %q", p, got, want)
		}
	}
}

// findingsOf is the "findings.txt" file of a: "" when the archive has none.
func findingsOf(a *txtar.Archive) string {
	for _, f := range a.Files {
		if f.Name == "findings.txt" {
			return string(f.Data)
		}
	}
	return ""
}
