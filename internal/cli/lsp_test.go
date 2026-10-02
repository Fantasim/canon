package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
)

func lspFrame(body string) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

// CLI.md §3.13, §2.5: canon lsp's exit codes, LSP 3.17's exit without shutdown included.
func TestLSPCommand(t *testing.T) {
	initialize := lspFrame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	shutdown := lspFrame(`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`)
	exit := lspFrame(`{"jsonrpc":"2.0","method":"exit"}`)
	cases := []struct {
		name, stdin string
		args        []string
		code        int
		stdout      string
		stderr      string
	}{
		{"shutdown then exit", initialize + shutdown + exit, nil, 0, `"id":2,"result":null`, ""},
		{"exit without shutdown", initialize + exit, nil, 1, `"positionEncoding":"utf-16"`, ""},
		{"no input", "", nil, 1, "", ""},
		{"a broken header", "Content-Length: x\r\n\r\n", nil, 1, "", "canon: lsp: invalid message header"},
		{"an argument", "", []string{"x"}, 2, "", "canon: lsp: takes no arguments"},
	}
	for _, tc := range cases {
		var out, errs bytes.Buffer
		env := cli.Env{Stdin: strings.NewReader(tc.stdin), Stdout: &out, Stderr: &errs, Dir: t.TempDir()}
		code := cli.Main(context.Background(), append([]string{"lsp"}, tc.args...), env)
		if code != tc.code || !strings.Contains(out.String(), tc.stdout) || !strings.HasPrefix(errs.String(), tc.stderr) {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", tc.name, code, out.String(), errs.String())
		}
	}
}
