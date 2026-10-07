package cli_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
)

// FORMATTER.md §15, IMPLEMENTATION-PLAN §6 M4 acceptance 1: `fmt --check` of the examples lists nothing and exits 0.
func TestExamplesFmtCheck(t *testing.T) {
	dir, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	env := cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: dir}
	code := cli.Main(context.Background(), append([]string{"fmt", "--check"}, exampleRoots(t)...), env)
	if code != 0 || stdout.Len() > 0 || stderr.Len() > 0 {
		t.Errorf("exit %d\n--- stdout\n%s--- stderr\n%s", code, stdout.String(), stderr.String())
	}
}
