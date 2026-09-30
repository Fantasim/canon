package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
	"github.com/fantasim/canonlang/internal/ir"
)

const jsonFixedPointN = 12

// FORMATTER.md FMT-02: the real `canon fmt --json-sources --check` changes nothing in a generated project.
func TestGeneratedJSONIsCanonical(t *testing.T) {
	out := filepath.Join(t.TempDir(), "bench")
	if err := generate(hashSeed, out, jsonFixedPointN); err != nil {
		t.Fatalf("generate: %v", err)
	}
	seen := 0
	for _, rel := range relFiles(t, out) {
		if strings.HasSuffix(rel, ir.JSONExt) {
			seen++
		}
	}
	if seen != jsonFixedPointN {
		t.Fatalf("JSON files = %d, want one per entry (%d)", seen, jsonFixedPointN)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"fmt", "--json-sources", "--check"}
	code := cli.Main(context.Background(), args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: out})
	if code != 0 {
		t.Errorf("fmt --json-sources --check exit %d, want 0 (nothing to change):\n%s%s", code, stdout.String(), stderr.String())
	}
}

// WIRE.md §7.4, DECISIONS 165: a Float is written in its canonical text on the JSON wire.
func TestJSONFloatIsCanonical(t *testing.T) {
	tests := []struct{ in, want string }{
		{"90.00", "90"},
		{"5.80", "5.8"},
		{"0.05", "0.05"},
		{"-0.25", "-0.25"},
		{"0.00", "0"},
		{"-0.00", "0"},
	}
	for _, tt := range tests {
		if got := jsonFloat(tt.in); got != tt.want {
			t.Errorf("jsonFloat(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
