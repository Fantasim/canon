package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
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

// loadExamples force a `load`, which fails the whole check with build.ErrLoad until M3
// (DECISIONS 196), so they are checked separately, not by TestExamplesParse.
var loadExamples = map[string]bool{
	"balance.parity":          true,
	"features.codes":          true,
	"features.csv":            true,
	"features.embedded":       true,
	"features.legacycpp":      true,
	"features.text":           true,
	"game.items":              true,
	"pipeline":                true,
	"resource.adventurequest": true,
	"resource.events":         true,
	"resource.farm":           true,
	"resource.heistia":        true,
	"resource.rules":          true,
	"resource.vocab":          true,
}

// exampleRoots redirects every root of examples/project.canon (examples/_fixtures/README.md):
// the read roots to their fixtures, the written ones to a new directory.
func exampleRoots(t *testing.T) []string {
	t.Helper()
	out := t.TempDir()
	args := []string{"--root", "resource=_fixtures/resource", "--root", "client=_fixtures/client"}
	for _, name := range []string{"source", "services", "sovcommon", "web", "parity", "generated"} {
		args = append(args, "--root", name+"="+filepath.ToSlash(filepath.Join(out, name)))
	}
	return args
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

// M1 acceptance 1, examples/_fixtures/README.md: `canon check` of every example package that
// does not force a `load` (loadExamples, DECISIONS 196), every root redirected, prints no
// finding of the parser or of project.canon.
func TestExamplesParse(t *testing.T) {
	dir, _ := filepath.Abs(examplesDir)
	p, err := canon.Open(dir, canon.Options{})
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := p.Packages(context.Background())
	if err != nil || len(pkgs) == 0 {
		t.Fatal(err)
	}
	owner := owners()
	for _, pkg := range pkgs {
		if loadExamples[pkg.Name] {
			continue
		}
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

// DECISIONS 196: `canon check` of a loadExamples package fails as a whole with build.ErrLoad
// (exit 2), not with per-package findings, until the load package exists (M3).
func TestExamplesLoadUntilM3(t *testing.T) {
	dir, _ := filepath.Abs(examplesDir)
	names := slices.Sorted(maps.Keys(loadExamples))
	for _, name := range names {
		args := append(append([]string{"check", "--project", dir}, exampleRoots(t)...), name)
		var stdout, stderr bytes.Buffer
		code := cli.Main(context.Background(), args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: dir})
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "load is not supported") {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", name, code, stdout.String(), stderr.String())
		}
	}
}

// M1 acceptance 2: `canon check teamboard` exits 0 and prints exactly its expected findings.
func TestTeamboardFindings(t *testing.T) {
	want, err := os.ReadFile(filepath.Join(examplesDir, "teamboard", "expected", "findings.txt"))
	if err != nil {
		t.Fatal(err)
	}
	code, out := checkExample(t, "teamboard")
	if got := durations.ReplaceAllString(out, "(…)"); code != 0 || got != string(want) {
		t.Errorf("exit %d\n--- want\n%s--- got\n%s", code, want, got)
	}
}
