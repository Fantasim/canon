package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// M3 acceptance 4, CLI.md §3.7: each case's args, then --project and the roots, print its golden.
func TestExplainExamples(t *testing.T) {
	dir, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	golden.Run(t, "testdata/examples/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		text, _ := archived(c.Archive, argsFile)
		args := append(strings.Fields(text), "--project", dir)
		args = append(args, exampleRoots(t)...)
		var out, errs bytes.Buffer
		code := cli.Main(context.Background(), args, cli.Env{Stdout: &out, Stderr: &errs, Dir: dir})
		return fmt.Appendf(nil, "[exit %d]\n[stdout]\n%s[stderr]\n%s", code, out.String(), errs.String())
	})
}
