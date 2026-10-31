package build_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"golang.org/x/tools/txtar"
)

// testsFile is the expected output of a world case: each test's outcome and failures.
const testsFile = "tests.txt"

// EVALUATION.md §10.5: the archive's first line names the items of §10.5 it proves.
func TestPackageSubjects(t *testing.T) {
	golden.Run(t, "testdata/worlds/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		p, err := build.Open(archiveFS(c.Archive), "/p", build.Options{Layers: fields(c.Archive, layersFile)})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.Test(context.Background(), nil, nil)
		if err != nil {
			return fmt.Appendf(nil, "test error (internal %t): %v\n", errors.Is(err, build.ErrInternal), err)
		}
		return []byte(renderTests(t, res))
	}, golden.Expected(testsFile))
}

// renderTests prints each test's outcome, its failures with their findings, then the static errors.
func renderTests(t *testing.T, res *build.TestResult) string {
	t.Helper()
	var b strings.Builder
	for _, tc := range res.Tests {
		fmt.Fprintf(&b, "%s %q: passed %t\n", tc.Package, tc.Name, tc.Passed)
		for _, f := range tc.Failures {
			fmt.Fprintf(&b, "  line %d: %s", f.Loc.Line, cmp.Or(f.Expect, "stopped"))
			if f.Poisoned != "" {
				fmt.Fprintf(&b, " (reads poisoned %s)", f.Poisoned)
			}
			b.WriteString("\n")
			writeFindings(&b, "    ", f.Findings)
			writeFindings(&b, "    cause ", f.Cause)
		}
	}
	b.WriteString("\n")
	b.WriteString(render(t, res.Static.Findings))
	return b.String()
}

// writeFindings prints each finding on one line: code, severity, path, check and message.
func writeFindings(b *strings.Builder, prefix string, fs []diag.Finding) {
	for _, f := range fs {
		fmt.Fprintf(b, "%s%s %s path %q check %q: %s\n", prefix, f.Code, f.Severity, f.Path, f.Check, strings.ReplaceAll(f.Message, "\n", "; "))
	}
}

const (
	vectorsProject = "project acme {\n  canon: \"0.1\"\n}\n"
	vectorsLib     = `/// A.
package a

/// A healing potion.
record Potion {
  /// Hit points restored.
  heal: Int

  /// What a player missing missingHp gets back.
  export fn healFor(self, missingHp: Int) -> Int { return min(heal, max(missingHp, 0)) }
}

/// The potions.
local let potions: [Potion] = [{ heal: 300 }]

check {
  for p in potions {
    if p.healFor(1000) > 400 {
      fail(p, "potion heals {p.healFor(1000)}, above 400")
    }
  }
}

test "healFor never overheals" {
  let p: Potion = { heal: 500 }
  expect p.healFor(200) == 200
}

emit cpp { out: "out/", namespace: "acme::gen", mode: data }
`
	// a package subject whose check calls healFor on receivers and with an argument no test call has
	vectorsPackageTest = `
test "the package check over other potions" {
  expect package passes
  expect package { potions: [{ heal: 777 }] } fails "above 400"
}
`
	// the control: the same receiver and argument in a call a test makes, which vectors record
	vectorsCallTest = `
test "a call" {
  let p: Potion = { heal: 777 }
  expect p.healFor(1000) == 777
}
`
)

// vectorsProjectOf is the project of package a holding vectorsLib and tests.
func vectorsProjectOf(t *testing.T, tests string) *build.Project {
	t.Helper()
	fsys := mapFS{"p/project.canon": file(vectorsProject), "p/a/a.canon": file(vectorsLib + tests)}
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// conformanceOf is the conformance file the build of vectorsLib and tests writes.
func conformanceOf(t *testing.T, tests string) string {
	t.Helper()
	res, err := vectorsProjectOf(t, tests).Build(context.Background(), build.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range res.Outputs {
		if strings.Contains(o.Path, conformanceMark) {
			return string(o.Content)
		}
	}
	t.Fatalf("no conformance file in %d outputs: %s", len(res.Outputs), render(t, res.Findings))
	return ""
}

// CONFORMANCE.md §6.1, EVALUATION.md §10.5 item 6: a world's calls give no vector; a test's call does.
func TestVectorsIgnorePackageSubjects(t *testing.T) {
	bare := conformanceOf(t, "")
	if with := conformanceOf(t, vectorsPackageTest); with != bare {
		t.Errorf("vectors with a package subject:\n%s\nwant those without:\n%s", with, bare)
	}
	if called := conformanceOf(t, vectorsCallTest); called == bare {
		t.Errorf("a test's call of healFor on { heal: 777 } left the vectors unchanged:\n%s", called)
	}
	res, err := vectorsProjectOf(t, vectorsPackageTest).Test(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range res.Tests {
		if !tc.Passed {
			t.Errorf("%s: failed: %+v", tc.Name, tc.Failures)
		}
	}
}

// EVALUATION.md §10.5 items 2 to 4, log-2026-10-31 N5-BUILD ruling: no world captures E3102 or E6004; the run reports each.
func TestRunReportsWhatNoWorldCaptures(t *testing.T) {
	for _, c := range []struct {
		file string
		code diag.Code
	}{
		{"testdata/worlds/codes.txtar", diag.E3102.Def().Code},
		{"testdata/worlds/layer.txtar", diag.E6004.Def().Code},
	} {
		t.Run(c.file, func(t *testing.T) {
			res := archiveTest(t, c.file)
			is := func(f diag.Finding) bool { return f.Code == c.code }
			if n := len(slices.DeleteFunc(slices.Clone(res.Static.List), func(f diag.Finding) bool { return !is(f) })); n != 1 {
				t.Errorf("the run reports %d %s, want 1: %s", n, c.code, render(t, res.Static.Findings))
			}
			unmatched := 0
			for _, tc := range res.Tests {
				for _, f := range tc.Failures {
					if slices.ContainsFunc(f.Findings, is) {
						t.Errorf("%s: a world captured %s: %+v", tc.Name, c.code, f)
					}
					if f.Outcome == "fails" && len(f.Findings) == 0 {
						unmatched++
					}
				}
			}
			if unmatched == 0 {
				t.Errorf("no `fails %s` got no finding: %+v", c.code, res.Tests)
			}
		})
	}
}

// EVALUATION.md §10.4, log-2026-10-31 N5-BUILD ruling: a broken test's causes are its phase 1-2 errors, never E3102.
func TestBrokenTestIgnoresCodes(t *testing.T) {
	res := archiveTest(t, "testdata/worlds/broken_codes.txtar")
	var got []diag.Code
	for _, tc := range res.Tests {
		for _, f := range tc.Failures {
			for _, x := range f.Findings {
				got = append(got, x.Code)
			}
		}
	}
	if want := []diag.Code{diag.E3002.Def().Code}; !slices.Equal(got, want) {
		t.Errorf("the broken test fails with %v, want %v", got, want)
	}
	codes := diag.E3102.Def().Code
	if !slices.ContainsFunc(res.Static.List, func(f diag.Finding) bool { return f.Code == codes }) {
		t.Errorf("the run lost %s: %s", codes, render(t, res.Static.Findings))
	}
}

// archiveTest is canon test over the project of the txtar file, its layers active.
func archiveTest(t *testing.T, file string) *build.TestResult {
	t.Helper()
	a, err := txtar.ParseFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p, err := build.Open(archiveFS(a), "/p", build.Options{Layers: fields(a, layersFile)})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Test(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
