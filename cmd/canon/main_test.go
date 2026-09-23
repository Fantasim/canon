package main

import (
	"context"
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
