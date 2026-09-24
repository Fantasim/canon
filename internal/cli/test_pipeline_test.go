package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
)

// brokenPipeline is pipeline's test with healFor broken to `return heal`: both failing expects (CLI.md §3.5).
const brokenPipeline = `FAIL  pipeline/potion.canon:39  healFor never overheals and never goes negative
  pipeline/potion.canon:41  expect p.healFor(200) == 200
  expected: 200
  got: 500
  pipeline/potion.canon:43  expect p.healFor(-5) == 0
  expected: 0
  got: 500

0 passed, 1 failed (…)
`

// runIn runs canon with args in the project dir, every example root redirected.
func runIn(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	args = append(append(args, "--project", dir), exampleRoots(t)...)
	code := cli.Main(context.Background(), args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: dir})
	if stderr.Len() > 0 {
		t.Errorf("%q: stderr %s", args, stderr.String())
	}
	return code, normalize(stdout.String(), dir)
}

// brokenCopy copies project.canon and the packages pipeline reads, healFor broken to `return heal`.
func brokenCopy(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	for _, dir := range []string{"pipeline", "studio", "sovcommon/time"} {
		if err := os.CopyFS(filepath.Join(dst, dir), os.DirFS(filepath.Join(src, dir))); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"project.canon", "pipeline/potion.canon"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Replace(string(data), "{ return min(heal, max(missingHp, 0)) }", "{ return heal }", 1)
		if err := os.WriteFile(filepath.Join(dst, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

// IMPLEMENTATION-PLAN.md M2 acceptance 4, CLI.md §3.5: `canon test pipeline` reports 1 passed.
func TestPipelinePasses(t *testing.T) {
	dir, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	code, out := runIn(t, dir, "test", "pipeline")
	if code != 0 || out != "1 passed, 0 failed (…)\n" {
		t.Errorf("exit %d:\n%s", code, out)
	}
}

// IMPLEMENTATION-PLAN.md M2 acceptance 4, CLI.md §3.5: a copy with healFor broken fails in §3.5's format.
func TestPipelineBroken(t *testing.T) {
	code, out := runIn(t, brokenCopy(t), "test", "pipeline")
	if code != 1 || out != brokenPipeline {
		t.Errorf("exit %d:\n%s\nwant:\n%s", code, out, brokenPipeline)
	}
}

// DOCTRINE §5, CLI.md §3.5: two runs print the same bytes, text and JSON (durations aside).
func TestTestDeterministic(t *testing.T) {
	dir := brokenCopy(t)
	for _, args := range [][]string{{"test", "-v"}, {"test", "--format", "json"}} {
		code1, first := runIn(t, dir, args...)
		code2, second := runIn(t, dir, args...)
		if code1 != code2 || first != second {
			t.Errorf("%q: runs differ:\n%s\n---\n%s", args, first, second)
		}
	}
}
