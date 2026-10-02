package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"golang.org/x/tools/txtar"
)

const (
	argsFile  = "args" // one argument per line; $TMP is the case's directory
	dirFile   = "dir"  // the directory to run in, under $TMP
	wantFile  = "want"
	stdinFile = "stdin" // the command's standard input
	envFile   = "env"   // injected Env values, one word per line: terminal, no_color
	tmpMark   = "$TMP"
)

// windowsOS is runtime.GOOS on Windows.
const windowsOS = "windows"

// unixTextCases print the operating system's own error text, which their goldens hold as Unix
// spells it (build_blocked: "open: not a directory"; Windows fails there at mkdir, in its own
// words): they run everywhere but Windows.
var unixTextCases = []string{"build_blocked.txtar"}

var (
	durations = regexp.MustCompile(`\((\d+ ms|\d+\.\d s)\)`)
	jsonMs    = regexp.MustCompile(`"ms":\d+`)
)

func control(name string) bool {
	return name == argsFile || name == dirFile || name == wantFile || name == stdinFile || name == envFile
}

func archived(a *txtar.Archive, name string) (string, bool) {
	for _, f := range a.Files {
		if f.Name == name {
			return string(f.Data), true
		}
	}
	return "", false
}

// normalize replaces the case's directory and the durations (IMPLEMENTATION-PLAN.md §8.1).
func normalize(s, tmp string) string {
	s = strings.ReplaceAll(s, filepath.ToSlash(tmp), tmpMark)
	s = durations.ReplaceAllString(s, "(…)")
	return jsonMs.ReplaceAllString(s, `"ms":0`)
}

// result is what a command line did.
type result struct {
	tmp            string
	code           int
	stdout, stderr string
}

// run runs a case's command line in a new directory holding the case's other files.
func run(t *testing.T, a *txtar.Archive) result {
	t.Helper()
	tmp := t.TempDir()
	for _, f := range a.Files {
		if control(f.Name) {
			continue
		}
		name := filepath.Join(tmp, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, f.Data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dir := tmp
	if d, ok := archived(a, dirFile); ok {
		dir = filepath.Join(tmp, strings.TrimSpace(d))
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	text, _ := archived(a, argsFile)
	args := strings.Fields(strings.ReplaceAll(text, tmpMark, filepath.ToSlash(tmp)))
	in, _ := archived(a, stdinFile)
	var out, errs bytes.Buffer
	envText, _ := archived(a, envFile)
	words := strings.Fields(envText)
	env := cli.Env{
		Stdin: strings.NewReader(in), Stdout: &out, Stderr: &errs, Dir: dir,
		Terminal: slices.Contains(words, "terminal"), NoColor: slices.Contains(words, "no_color"),
	}
	code := cli.Main(context.Background(), args, env)
	return result{tmp: tmp, code: code, stdout: out.String(), stderr: errs.String()}
}

// written lists each file the command created or changed, with its content.
func written(t *testing.T, a *txtar.Archive, tmp string) string {
	t.Helper()
	var out strings.Builder
	err := filepath.WalkDir(tmp, func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(tmp, name)
		data, err := os.ReadFile(name)
		if old, ok := archived(a, filepath.ToSlash(rel)); !ok || old != string(data) {
			fmt.Fprintf(&out, "[file %s]\n%s", filepath.ToSlash(rel), data)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// CLI.md §2, §3.1-§3.3, §3.14: each case's exit code, output and written files.
func TestCommands(t *testing.T) {
	golden.Run(t, "testdata/commands/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		if runtime.GOOS == windowsOS && slices.Contains(unixTextCases, filepath.Base(c.Path)) {
			t.Skip("the golden holds Unix's error text")
		}
		r := run(t, c.Archive)
		got := fmt.Sprintf("[exit %d]\n[stdout]\n%s[stderr]\n%s%s", r.code, r.stdout, r.stderr, written(t, c.Archive, r.tmp))
		return []byte(normalize(got, r.tmp))
	})
}

// CLI.md §2.5: an interrupt ends the command with exit 130.
func TestInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errs bytes.Buffer
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "project.canon"), []byte("project a {\n  canon: \"0.1\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code := cli.Main(ctx, []string{"check"}, cli.Env{Stdout: &out, Stderr: &errs, Dir: dir})
	if code != 130 || !slices.Contains(strings.Split(errs.String(), "\n"), "canon: interrupted") {
		t.Errorf("exit %d, stderr %q", code, errs.String())
	}
}
