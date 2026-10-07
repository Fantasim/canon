package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/cli"
	"github.com/fantasim/canonlang/internal/diag"
)

const examplesDir = "../../examples"

// expectedDir and findingsFile are exampleFindings' special names (IMPLEMENTATION-PLAN §7.1/§7.2).
const (
	expectedDir  = "expected"
	findingsFile = "findings.txt"
	fixturesDir  = "_fixtures"
)

// exampleRoots redirects every root of examples/project.canon (examples/_fixtures/README.md):
// the read roots to their fixtures, the written ones to a new directory.
func exampleRoots(t *testing.T) []string {
	t.Helper()
	roots := exampleRootMap(t)
	var args []string
	for _, name := range slices.Sorted(maps.Keys(roots)) {
		args = append(args, "--root", name+"="+roots[name])
	}
	return args
}

// exampleRootMap is exampleRoots as Options.Roots, the written roots' directories made: a
// required root exists (DECISIONS 332).
func exampleRootMap(t *testing.T) map[string]string {
	t.Helper()
	out := t.TempDir()
	roots := map[string]string{"resource": "_fixtures/resource", "client": "_fixtures/client"}
	for _, name := range []string{"source", "services", "sovcommon", "web", "parity", "generated"} {
		roots[name] = filepath.ToSlash(filepath.Join(out, name))
		if err := os.MkdirAll(filepath.Join(out, name), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	return roots
}

func checkExample(t *testing.T, args ...string) (int, string) {
	t.Helper()
	dir, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args = append(append([]string{"check", "--project", dir}, exampleRoots(t)...), args...)
	code := cli.Main(context.Background(), args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: dir})
	if stderr.Len() > 0 {
		t.Errorf("%q: stderr %s", args, stderr.String())
	}
	return code, stdout.String()
}

// owners maps each code to the package that reports it (ERRORS.md, Package column).
func owners() map[diag.Code]string {
	out := map[diag.Code]string{}
	for _, d := range diag.Registry {
		out[d.Code] = d.Package
	}
	return out
}

// M1 acceptance 1, examples/_fixtures/README.md: `canon check` of every example package, every
// root redirected, prints no finding of the parser or of project.canon.
func TestExamplesParse(t *testing.T) {
	dir, _ := filepath.Abs(examplesDir)
	p, err := canon.Open(dir, canon.Options{Roots: exampleRootMap(t)})
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := p.Packages(context.Background())
	if err != nil || len(pkgs) == 0 {
		t.Fatal(err)
	}
	owner := owners()
	for _, pkg := range pkgs {
		_, out := checkExample(t, "--format", "json", pkg.Name)
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if !strings.HasPrefix(lines[len(lines)-1], `{"summary":`) {
			t.Errorf("%s: no summary line: %q", pkg.Name, out)
		}
		for _, line := range lines[:len(lines)-1] {
			var f canon.Finding
			if err := json.Unmarshal([]byte(line), &f); err != nil {
				t.Fatalf("%s: %v", pkg.Name, err)
			}
			if o := owner[diag.Code(f.Code)]; slices.Contains([]string{"syntax", "project"}, o) {
				t.Errorf("%s: %s[%s] %s:%d:%d %s", pkg.Name, f.Severity, f.Code, f.File, f.Line, f.Col, f.Message)
			}
		}
	}
}

// exampleFindings is the dot-joined name of every example with an expected/findings.txt, sorted (IMPLEMENTATION-PLAN §7.2).
func exampleFindings(t *testing.T) []string {
	t.Helper()
	root, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == fixturesDir {
			return fs.SkipDir
		}
		if !d.IsDir() && d.Name() == findingsFile && filepath.Base(filepath.Dir(path)) == expectedDir {
			rel, relErr := filepath.Rel(root, filepath.Dir(filepath.Dir(path)))
			if relErr != nil {
				return relErr
			}
			names = append(names, strings.ReplaceAll(filepath.ToSlash(rel), "/", "."))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(names)
	return names
}

// wantExitError is CLI.md §2.5's exit code 1: at least one error finding.
const wantExitError = 1

// wantExit is the exit code want's own summary line implies (CLI.md §2.5), not assumed clean.
func wantExit(want string) int {
	lines := strings.Split(strings.TrimSuffix(want, "\n"), "\n")
	if n, _, _ := strings.Cut(lines[len(lines)-1], " "); n != "0" {
		return wantExitError
	}
	return 0
}

// `canon check <pkg>` prints exactly its findings.txt for every example (IMPLEMENTATION-PLAN §6 M3 item 1; replaces TestResourceFindings/DECISIONS 173 and TestTeamboardFindings/M1 acceptance 2).
func TestExampleFindings(t *testing.T) {
	for _, pkg := range exampleFindings(t) {
		t.Run(pkg, func(t *testing.T) {
			path := filepath.Join(examplesDir, filepath.FromSlash(strings.ReplaceAll(pkg, ".", "/")), expectedDir, findingsFile)
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			code, out := checkExample(t, pkg)
			got := durations.ReplaceAllString(out, "(…)")
			if code != wantExit(string(want)) || got != string(want) {
				t.Errorf("exit %d\n--- want\n%s--- got\n%s", code, want, got)
			}
		})
	}
}
