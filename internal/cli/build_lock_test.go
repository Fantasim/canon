package cli_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/cli"
)

const lockTestProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    out: \"out\"\n  }\n}\n"
const lockTestPackage = "/// A.\npackage a\n\n/// Tier.\nrecord Tier {\n  /// Weight.\n  weight: Int = 1\n}\n\n" +
	"/// Tiers.\nlet tiers: stable table Tier = { low {} }\n\nemit json { out: \"@out/\" }\n"

// DECISIONS 201: a lock is listed only when it gains lines; building again with nothing new
// shows no lock section, and the text report lists no output (everything is unchanged).
func TestBuildSecondTimeUnchanged(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "project.canon"), lockTestProject)
	write(t, filepath.Join(dir, "a", "a.canon"), lockTestPackage)
	args := []string{"build", "--project", dir}
	first := runBuildCLI(t, args)
	if first.code != 0 || !bytes.Contains(first.stdout, []byte("lock:\n  a/canon.lock\n")) || !bytes.Contains(first.stdout, []byte("json:\n")) {
		t.Fatalf("first build: exit %d, stdout %q", first.code, first.stdout)
	}
	again := runBuildCLI(t, args)
	if again.code != 0 || bytes.Contains(again.stdout, []byte("lock:")) || bytes.Contains(again.stdout, []byte("json:")) {
		t.Errorf("second build: exit %d, stdout %q", again.code, again.stdout)
	}
}

// DECISIONS 201, meta/decisions/log-2026-09-24.md "Chosen while resuming": a lock a --check
// build would append is listed with the `stale ` prefix and counted into the summary's stale
// count, since --check writes nothing.
func TestBuildCheckListsLockStale(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "project.canon"), lockTestProject)
	write(t, filepath.Join(dir, "a", "a.canon"), lockTestPackage)
	text := runBuildCLI(t, []string{"build", "--project", dir, "--check"})
	if text.code != 1 || !bytes.Contains(text.stdout, []byte("stale a/canon.lock\n")) {
		t.Fatalf("text --check: exit %d, stdout %q", text.code, text.stdout)
	}
	js := runBuildCLI(t, []string{"build", "--project", dir, "--check", "--format", "json"})
	if js.code != 1 || !bytes.Contains(js.stdout, []byte(`"stale":2`)) {
		t.Errorf("json --check: exit %d, stdout %q", js.code, js.stdout)
	}
}

// DECISIONS 201, CLI.md §2.3: -q with --format json also drops the output and lock lines, the summary (with its written/stale counts) kept.
func TestBuildQuietJSONDropsLines(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "project.canon"), lockTestProject)
	write(t, filepath.Join(dir, "a", "a.canon"), lockTestPackage)
	got := runBuildCLI(t, []string{"build", "--project", dir, "-q", "--format", "json"})
	if got.code != 0 || bytes.Contains(got.stdout, []byte(`"output"`)) || bytes.Contains(got.stdout, []byte(`"lock"`)) ||
		!bytes.Contains(got.stdout, []byte(`"written":2`)) {
		t.Errorf("quiet json: exit %d, stdout %q", got.code, got.stdout)
	}
}

// DECISIONS 201: JSON lists every output, unchanged included, unlike the text report.
func TestBuildJSONListsUnchanged(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "project.canon"), lockTestProject)
	write(t, filepath.Join(dir, "a", "a.canon"), lockTestPackage)
	args := []string{"build", "--project", dir, "--format", "json"}
	if first := runBuildCLI(t, args); first.code != 0 {
		t.Fatalf("first build: exit %d, stdout %q", first.code, first.stdout)
	}
	again := runBuildCLI(t, args)
	if again.code != 0 || !bytes.Contains(again.stdout, []byte(`"status":"unchanged"`)) {
		t.Errorf("second build: exit %d, stdout %q", again.code, again.stdout)
	}
}

type cliResult struct {
	code           int
	stdout, stderr []byte
}

func runBuildCLI(t *testing.T, args []string) cliResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Main(context.Background(), args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: ""})
	return cliResult{code: code, stdout: stdout.Bytes(), stderr: stderr.Bytes()}
}
