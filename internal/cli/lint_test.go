package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
)

// lintProject is a project whose one record keeps two legacy prefixes, each with a fix.
var lintProject = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
	"a/a.canon":     "/// A.\npackage a\n\n/// A unit.\nrecord Unit {\n  /// Health.\n  dwHealth: Int\n  /// Mana.\n  dwMana: Int = 0\n}\n\n/// The units.\nlet units: [Unit] = [{ dwHealth: 3 }]\n",
}

// fixLine is the line canon lint --fix prints first (CLI.md §3.19).
type fixLine struct {
	Fix struct {
		Code    string          `json:"code"`
		File    string          `json:"file"`
		Line    int             `json:"line"`
		Col     int             `json:"col"`
		Request json.RawMessage `json:"request"`
	} `json:"fix"`
}

// canonIn runs a command line in dir with stdin and returns its exit code and output lines.
func canonIn(dir, stdin string, args ...string) (int, []string) {
	var out, errs bytes.Buffer
	env := cli.Env{Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: &errs, Dir: dir}
	code := cli.Main(context.Background(), args, env)
	return code, strings.Split(strings.TrimSuffix(out.String()+errs.String(), "\n"), "\n")
}

// fixesOf reads the fix lines of a lint --fix output, which come before every other line.
func fixesOf(t *testing.T, lines []string) []fixLine {
	t.Helper()
	var fixes []fixLine
	for _, l := range lines {
		if !strings.HasPrefix(l, `{"fix":`) {
			break
		}
		var f fixLine
		if err := json.Unmarshal([]byte(l), &f); err != nil {
			t.Fatal(err)
		}
		fixes = append(fixes, f)
	}
	return fixes
}

// CLI.md §3.19, API.md L10: a fix piped into canon edit applies; the other is then stale (S5): lint again.
func TestLintFixPipesIntoEdit(t *testing.T) {
	dir := t.TempDir()
	for name, text := range lintProject {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, lines := canonIn(dir, "", "lint", "--fix")
	fixes := fixesOf(t, lines)
	if code != 0 || len(fixes) != 2 {
		t.Fatalf("lint --fix: exit %d, %d fixes, want 0 and 2:\n%s", code, len(fixes), strings.Join(lines, "\n"))
	}
	if code, lines := canonIn(dir, string(fixes[0].Fix.Request), "edit"); code != 0 {
		t.Fatalf("edit of the first fix: exit %d:\n%s", code, strings.Join(lines, "\n"))
	}
	if code, _ := canonIn(dir, string(fixes[1].Fix.Request), "edit"); code != 1 {
		t.Errorf("edit of the second fix after the first: exit %d, want 1 (stale, API.md S5)", code)
	}
	code, lines = canonIn(dir, "", "lint", "--fix")
	fixes = fixesOf(t, lines)
	if code != 0 || len(fixes) != 1 {
		t.Fatalf("lint again: exit %d, %d fixes, want 0 and 1:\n%s", code, len(fixes), strings.Join(lines, "\n"))
	}
	if code, lines := canonIn(dir, string(fixes[0].Fix.Request), "edit"); code != 0 {
		t.Fatalf("edit of the fix linted again: exit %d:\n%s", code, strings.Join(lines, "\n"))
	}
	code, lines = canonIn(dir, "", "lint", "--fix")
	if code != 0 || len(lines) != 1 || !strings.HasPrefix(lines[0], `{"summary":{"errors":0,"warnings":0,`) {
		t.Errorf("lint after every fix: exit %d:\n%s", code, strings.Join(lines, "\n"))
	}
}
