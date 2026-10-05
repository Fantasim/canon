package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
)

// TestHelp: DECISIONS 322, CLI.md §2.5.
func TestHelp(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		code    int
		onOut   string
		onErr   string
		pointer bool
	}{
		{"help command", []string{"help"}, 0, "usage: canon", "", true},
		{"short flag", []string{"-h"}, 0, "usage: canon", "", true},
		{"long flag", []string{"--help"}, 0, "usage: canon", "", true},
		{"command short flag", []string{"check", "-h"}, 0, "-max-warnings", "", true},
		{"command long flag", []string{"build", "--help"}, 0, "-target", "", true},
		{"unknown flag", []string{"check", "--nope"}, 2, "", "canon: ", false},
		{"unknown command", []string{"frobnicate"}, 2, "", "unknown command", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errs bytes.Buffer
			code := cli.Main(context.Background(), c.args, cli.Env{Stdout: &out, Stderr: &errs, Dir: t.TempDir()})
			if code != c.code {
				t.Fatalf("exit %d, want %d", code, c.code)
			}
			if !strings.Contains(out.String(), c.onOut) || !strings.Contains(errs.String(), c.onErr) {
				t.Fatalf("stdout %q stderr %q", out.String(), errs.String())
			}
			if c.pointer != strings.Contains(out.String(), "canon guide") {
				t.Fatalf("guide pointer mismatch: %q", out.String())
			}
			if c.code == 0 && errs.Len() != 0 {
				t.Fatalf("stderr not empty: %q", errs.String())
			}
			if c.code == 2 && out.Len() != 0 {
				t.Fatalf("stdout not empty: %q", out.String())
			}
		})
	}
}
