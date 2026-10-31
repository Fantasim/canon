package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
)

// failedCount reads the failed count of canon test's summary line (CLI.md §3.5).
var failedCount = regexp.MustCompile(`, (\d+) failed`)

// staticHeader opens a static error printed before the tests (CLI.md §3.5, ADR-0004).
const staticHeader = "error["

// testsExit is the exit code want implies (CLI.md §3.5).
func testsExit(want string) int {
	lines := strings.Split(strings.TrimSuffix(want, "\n"), "\n")
	m := failedCount.FindStringSubmatch(lines[len(lines)-1])
	if len(m) == 0 || m[1] != "0" || strings.Contains(want, "\n"+staticHeader) || strings.HasPrefix(want, staticHeader) {
		return wantExitError
	}
	return 0
}

// testsMismatch is empty when a `canon test` run of exit code and output is want, else the
// difference, written for a failure message.
func testsMismatch(code int, out, want string) string {
	got := durations.ReplaceAllString(out, "(…)")
	if code == testsExit(want) && got == want {
		return ""
	}
	return "exit " + strconv.Itoa(code) + "\n--- want\n" + want + "--- got\n" + got
}

// ADR-0023 §2, DECISIONS 398: `canon test <pkg>` prints exactly expected/tests.txt (§7.9).
func TestExampleTests(t *testing.T) {
	for _, pkg := range exampleExpected(t, testsFile) {
		t.Run(pkg, func(t *testing.T) {
			path := filepath.Join(examplesDir, filepath.FromSlash(strings.ReplaceAll(pkg, ".", "/")), expectedDir, testsFile)
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			code, out := commandExample(t, "test", pkg)
			if msg := testsMismatch(code, out, string(want)); msg != "" {
				t.Error(msg)
			}
		})
	}
}

// fixtureProject is a package with one passing and one failing test, and the tests.txt it prints.
const (
	fixtureProject = "project acme {\n  canon: \"0.1\"\n}\n"
	fixturePackage = "/// A.\npackage a\n\nlocal let one: Int = 1\n\ntest \"passes\" {\n  expect one == 1\n}\n\n" +
		"test \"fails\" {\n  expect one == 2\n}\n"
	fixtureWant = "FAIL  a/a.canon:10  fails\n  a/a.canon:11  expect one == 2\n  expected: 2\n  got: 1\n\n" +
		"1 passed, 1 failed (…)\n"
)

// ADR-0023 §2, DECISIONS 398: the comparison fails on a wrong file or exit code, on a fixture.
func TestExampleTestsCompareExactly(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a"), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, f := range [][2]string{{"project.canon", fixtureProject}, {"a/a.canon", fixturePackage}} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(f[0])), []byte(f[1]), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errs bytes.Buffer
	code := cli.Main(context.Background(), []string{"test"}, cli.Env{Stdout: &out, Stderr: &errs, Dir: dir})
	if msg := testsMismatch(code, out.String(), fixtureWant); msg != "" {
		t.Errorf("the right file differs: %s", msg)
	}
	wrong := strings.Replace(fixtureWant, "got: 1", "got: 2", 1)
	if testsMismatch(code, out.String(), wrong) == "" {
		t.Error("a file with a different line matched")
	}
	if testsMismatch(0, out.String(), fixtureWant) == "" {
		t.Error("exit 0 matched a file that implies exit 1")
	}
	clean := "1 passed, 0 failed (…)\n"
	if testsMismatch(1, clean, clean) == "" {
		t.Error("exit 1 matched a file that implies exit 0")
	}
	static := staticHeader + "…]  a/a.canon:4:20\n  unknown name nothing\n\n1 error, 0 warnings in 1 package (…)\n\n" + clean
	if testsExit(static) != wantExitError {
		t.Error("a file opening with a static error implies exit 0")
	}
}
