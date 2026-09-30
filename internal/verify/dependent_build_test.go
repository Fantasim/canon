package verify_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"golang.org/x/tools/txtar"
)

const (
	projectDirMode  = 0o750
	projectFileMode = 0o600
	halve           = 2
	budgetCeiling   = 100_000
	checkSteps      = 300 // a run of shared.txtar's check spins as many times, more steps than that
	outputsFile     = "outputs.txt"
	budgetFile      = "budget.txt"
	testsFile       = "tests.txt"
	thresholdFile   = "threshold.txt"
	projectFile     = "project.canon"
	outputHeader    = "-- "
	budgetLine      = "  budget: "
	projectOpen     = "project demo {\n  canon: \"0.1\"\n"
)

// writeArchive writes the case's files under a new directory, but its golden files.
func writeArchive(t *testing.T, a *txtar.Archive) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range a.Files {
		if slices.Contains([]string{findingsFile, outputsFile, budgetFile, testsFile, thresholdFile}, f.Name) {
			continue
		}
		p := filepath.Join(dir, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(p), projectDirMode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Data, projectFileMode); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// rendered is a build's findings in the golden text form.
func rendered(t *testing.T, f build.Findings) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := diag.Render(&buf, f.Files, f.List, diag.RenderOptions{Summary: f.Summary, Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// checked runs `canon check` on the project a case holds (EVALUATION.md §1: stages A to E).
func checked(t *testing.T, dir string) build.Findings {
	t.Helper()
	p, err := build.Open(build.OS(), filepath.ToSlash(dir), build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return res.Findings
}

// builtCase verifies a whole project's values, checked and evaluated for real (TYPES.md §11.6).
func builtCase(fx *fixture) {
	fx.t.Helper()
	fx.out = rendered(fx.t, checked(fx.t, writeArchive(fx.t, fx.archive)))
}

// TYPES.md §11.6: `canon build` writes a dependent value as its computed branch's, never a symbol.
func TestDependentValuesReachOutputsResolved(t *testing.T) {
	golden.Run(t, "testdata/dependent/emit_*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		p, err := build.Open(build.OS(), filepath.ToSlash(writeArchive(t, c.Archive)), build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Build(context.Background(), build.BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		out.Write(rendered(t, res.Findings))
		for _, o := range res.Outputs {
			data, err := os.ReadFile(filepath.FromSlash(o.Abs))
			if err != nil {
				t.Fatal(err)
			}
			out.WriteString(outputHeader + o.Path + "\n")
			out.Write(data)
		}
		return out.Bytes()
	}, golden.Expected(outputsFile))
}

// TYPES.md §11.6, §7.5: a name written in a check equals the converted dependent value it names.
func TestDependentValuesCompare(t *testing.T) {
	golden.Run(t, "testdata/dependent/compare.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		return rendered(t, checked(t, writeArchive(t, c.Archive)))
	}, golden.Expected(findingsFile))
}

// EVALUATION.md §12.1: one step short, stage B's last step (application, default, first where) is charged to the value.
func TestDependentStepsAreCharged(t *testing.T) {
	golden.Run(t, "testdata/dependent/budget*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		f, _ := withBudget(t, c.Archive, threshold(t, c.Archive))
		return rendered(t, f)
	}, golden.Expected(budgetFile))
}

// EVALUATION.md §12.1 (log-2026-09-29 M4 Cleanup-A-r): retyping a computed container charges no application of its own.
func TestComputedTypesChargeNothing(t *testing.T) {
	golden.Run(t, "testdata/dependent/charges_*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		return []byte(fmt.Sprintf("threshold %d\n", threshold(t, c.Archive)))
	}, golden.Expected(thresholdFile))
}

// EVALUATION.md §4.2, §8.1: shared instances share their stage-B copies, so stage C checks each once.
func TestSharedInstancesVerifyOnce(t *testing.T) {
	cases, err := golden.Load("testdata/dependent/shared*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		a := c.Archive
		plain := &txtar.Archive{Files: []txtar.File{{Name: a.Files[0].Name, Data: a.Files[1].Data}}}
		dep := &txtar.Archive{Files: a.Files[:1]}
		td, tp := threshold(t, dep), threshold(t, plain)
		t.Logf("%s: thresholds %d with a dependent type, %d without", c.Path, td, tp)
		if td < tp || td-tp >= checkSteps {
			t.Errorf("%s: threshold %d with a dependent type, %d without; want at most %d more", c.Path, td, tp, checkSteps)
		}
	}
}

// threshold is the largest budget the case's package exhausts.
func threshold(t *testing.T, a *txtar.Archive) int {
	t.Helper()
	lo, hi := 1, budgetCeiling
	if _, out := withBudget(t, a, hi); out {
		t.Fatalf("budget %d is exhausted", hi)
	}
	for lo+1 < hi {
		mid := (lo + hi) / halve
		if _, out := withBudget(t, a, mid); out {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// EVALUATION.md §10.2, §7.2: subjects and per-verification bags see a remembered instance's findings at their paths.
func TestDependentTests(t *testing.T) {
	golden.Run(t, "testdata/dependent/test_*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		p, err := build.Open(build.OS(), filepath.ToSlash(writeArchive(t, c.Archive)), build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Test(context.Background(), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		return testsText(res)
	}, golden.Expected(testsFile))
}

// testsText is each test's outcome, then each failing expect with what it captured and the
// errors that poisoned the value it read.
func testsText(res *build.TestResult) []byte {
	var b bytes.Buffer
	for _, tc := range res.Tests {
		fmt.Fprintf(&b, "%s passed=%t\n", tc.Name, tc.Passed)
		for _, f := range tc.Failures {
			fmt.Fprintf(&b, "  expect %q %s got %q poisoned %q\n", f.Expect, f.Outcome, f.Got, f.Poisoned)
			for _, x := range f.Findings {
				fmt.Fprintf(&b, "    finding %s %s: %s\n", x.Code, x.Path, x.Message)
			}
			for _, x := range f.Cause {
				fmt.Fprintf(&b, "    cause %s %s: %s\n", x.Code, x.Path, x.Message)
			}
		}
	}
	return b.Bytes()
}

// withBudget checks the case's package under a project of that budget; true: E4401.
func withBudget(t *testing.T, a *txtar.Archive, budget int) (build.Findings, bool) {
	t.Helper()
	b := *a
	project := projectOpen + budgetLine + strconv.Itoa(budget) + "\n}\n"
	b.Files = append([]txtar.File{{Name: projectFile, Data: []byte(project)}}, a.Files...)
	f := checked(t, writeArchive(t, &b))
	return f, slices.ContainsFunc(f.List, func(x diag.Finding) bool { return x.Code == diag.E4401.Def().Code })
}
