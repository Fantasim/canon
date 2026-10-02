package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

// CLI.md §3.14, §2.5: the binary runs internal/cli's command line.
func TestRun(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"version"}, &stdout, &stderr); code != 0 ||
		!strings.HasPrefix(stdout.String(), "canon ") || stderr.Len() != 0 {
		t.Errorf("version: exit %d, %q, %q", code, stdout.String(), stderr.String())
	}
	if code := run(context.Background(), []string{"frobnicate"}, &stdout, &stderr); code != 2 {
		t.Errorf("unknown command: exit %d", code)
	}
}

// CLI.md §2.3: a pipe is not a terminal, so --color auto does not colour into it.
func TestIsTerminalPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if isTerminal(w) {
		t.Error("a pipe is a terminal")
	}
	if isTerminal(&strings.Builder{}) {
		t.Error("a non-file writer is a terminal")
	}
}
