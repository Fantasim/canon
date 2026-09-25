package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/cli"
	"github.com/fantasim/canonlang/internal/diag"
)

const examplesDir = "../../examples"

// dependentBlocked force a load of a record holding a dependent type, refused loudly rather
// than silently poisoning (internal/load.hasDependent, DECISIONS 173), not a load bug.
var dependentBlocked = map[string]bool{
	"resource.adventurequest": true,
	"resource.heistia":        true,
}

// defaultBlocked force a load of a record whose field default is not a plain literal (WIRE.md §6.1).
var defaultBlocked = map[string]bool{
	"resource.events": true,
	"resource.farm":   true,
	"resource.rules":  true,
	"resource.vocab":  true,
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
// does not force a dependent-typed load (dependentBlocked), every root redirected, prints no
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
		if dependentBlocked[pkg.Name] || defaultBlocked[pkg.Name] {
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

// TestBlockedLoadsFailWithTheirCause is each blocked package's own cause text on stderr (WIRE.md §6.1).
func TestBlockedLoadsFailWithTheirCause(t *testing.T) {
	const dependentCause = "a dependent type this milestone cannot decode without the evaluator"
	const defaultCause = "a default this milestone cannot decode without the evaluator"
	cases := map[string]string{
		"resource.adventurequest": dependentCause,
		"resource.heistia":        dependentCause,
		"resource.events":         defaultCause,
		"resource.farm":           defaultCause,
		"resource.rules":          defaultCause,
		"resource.vocab":          defaultCause,
	}
	dir, err := filepath.Abs(examplesDir)
	if err != nil {
		t.Fatal(err)
	}
	for pkg, cause := range cases {
		var stdout, stderr bytes.Buffer
		args := append(append([]string{"check", "--project", dir}, exampleRoots(t)...), pkg)
		cli.Main(context.Background(), args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: dir})
		if !strings.Contains(stderr.String(), cause) {
			t.Errorf("%s: stderr %q does not contain %q", pkg, stderr.String(), cause)
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

// `canon check pipeline` exits 0 and prints exactly its expected findings, W1701 aside (i18n
// status waits for M3), so a load or wire finding it now reaches fails here, not TestExamplesParse.
func TestPipelineFindings(t *testing.T) {
	want, err := os.ReadFile(filepath.Join(examplesDir, "pipeline", "expected", "findings.txt"))
	if err != nil {
		t.Fatal(err)
	}
	code, out := checkExample(t, "pipeline")
	got := durations.ReplaceAllString(out, "(…)")
	if wantClean := withoutW1701(string(want)); code != 0 || got != wantClean {
		t.Errorf("exit %d\n--- want\n%s--- got\n%s", code, wantClean, got)
	}
}

// summaryCountsRe matches a rendered summary's leading counts (API.md F15).
var summaryCountsRe = regexp.MustCompile(`^\d+ errors?, \d+ warnings? in`)

// i18nStatusPrefix is the rendered header of a W1701 finding (the registry names the code).
var i18nStatusPrefix = "warning[" + string(diag.W1701.Def().Code) + "]"

// withoutW1701 removes want's W1701 block (i18n status) and recomputes the summary's counts,
// so any other finding still fails the comparison it is used in.
func withoutW1701(want string) string {
	parts := strings.Split(strings.TrimSuffix(want, "\n"), "\n\n")
	summary, blocks := parts[len(parts)-1], parts[:len(parts)-1]
	var kept []string
	errs, warns := 0, 0
	for _, b := range blocks {
		if strings.HasPrefix(b, i18nStatusPrefix) {
			continue
		}
		kept = append(kept, b)
		switch {
		case strings.HasPrefix(b, "error["):
			errs++
		case strings.HasPrefix(b, "warning["):
			warns++
		}
	}
	summary = summaryCountsRe.ReplaceAllString(summary, plural(errs, "error")+", "+plural(warns, "warning")+" in")
	return strings.Join(append(kept, summary), "\n\n") + "\n"
}

// plural is "<n> <noun>", singular when n is 1 (API.md F15).
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
