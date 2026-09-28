package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestValidate proves the CLI's stated contract (doc.go): -out is required, -n must be positive.
func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		o    options
		args int
		want error
	}{
		{"ok", options{out: "x", n: 1}, 0, nil},
		{"missing out", options{out: "", n: 1}, 0, errNoOut},
		{"zero n", options{out: "x", n: 0}, 0, errBadN},
		{"negative n", options{out: "x", n: -1}, 0, errBadN},
		{"extra args", options{out: "x", n: 1}, 1, errArgs},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validate(tt.o, tt.args); !errors.Is(err, tt.want) {
				t.Errorf("validate(%+v, %d) = %v, want %v", tt.o, tt.args, err, tt.want)
			}
		})
	}
}

// TestRunBadN proves `run` refuses a non-positive -n before writing anything (exitUsage).
func TestRunBadN(t *testing.T) {
	out := filepath.Join(t.TempDir(), "bench")
	args := []string{"-seed", "1", "-out", out, "-n", "0"}
	if code := run(args, io.Discard); code != exitUsage {
		t.Fatalf("run(-n 0) = %d, want %d", code, exitUsage)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatalf("run(-n 0) wrote %s", out)
	}
}

// TestRunNoOut proves `run` refuses a missing -out.
func TestRunNoOut(t *testing.T) {
	if code := run([]string{"-seed", "1"}, io.Discard); code != exitUsage {
		t.Fatalf("run(no -out) = %d, want %d", code, exitUsage)
	}
}

// TestCheckOutEmpty proves an absent or empty -out is accepted, a non-empty one refused.
func TestCheckOutEmpty(t *testing.T) {
	empty := t.TempDir()
	if err := checkOutEmpty(filepath.Join(empty, "new")); err != nil {
		t.Errorf("absent dir: %v", err)
	}
	if err := checkOutEmpty(empty); err != nil {
		t.Errorf("empty dir: %v", err)
	}
	dirty := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirty, "x"), []byte("x"), filePerm); err != nil {
		t.Fatal(err)
	}
	if err := checkOutEmpty(dirty); !errors.Is(err, errOutDirty) {
		t.Errorf("non-empty dir: %v, want %v", err, errOutDirty)
	}
}

// TestRunOutDirty proves a second `run` into the same -out is refused, not left to leave stale
// entries under a different case and duplicate a table key (E3101): seed 1, then seed 2.
func TestRunOutDirty(t *testing.T) {
	out := filepath.Join(t.TempDir(), "bench")
	args := func(seed string) []string { return []string{"-seed", seed, "-out", out, "-n", "20"} }
	if code := run(args("1"), io.Discard); code != exitOK {
		t.Fatalf("first run = %d, want %d", code, exitOK)
	}
	if code := run(args("2"), io.Discard); code != exitUsage {
		t.Fatalf("second run into the same -out = %d, want %d", code, exitUsage)
	}
}
