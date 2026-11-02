package canon_test

import (
	"context"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// staticSelected is package a: a W1003, a header its test reads with a W7101, and an import of b.
const staticSelected = `/// A.
package a

import b

/// The header's codes.
local let codes = load.defines("data/a.h")

/// A const named against its convention.
const version = 7

test "reads the header and b" {
  expect codes.len() == 1
  expect b.ONE == 1
}
`

// staticImported is package b, only imported: its phase-2 warning (W3401) is not reported, its error is.
const staticImported = `/// B.
package b

/// One.
const ONE = 1

/// Never none.
local let never: Int = ONE ?? 2

local let broken = nothing
`

// API.md B5, DECISIONS 449: Check holds the errors of phases 1-2 of every loaded package and the
// warnings of the selected ones, as Check reports them (R2), the W7101 of a header the tests read
// included, with their summary: b's W3401 is left out, b's error is in, a's warnings are in.
func TestTestResultCheckHoldsStaticWarnings(t *testing.T) {
	p, err := canon.Open("/law", project(map[string]string{
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"a/a.canon":     staticSelected,
		"a/data/a.h":    "#define A_ONE 1\n#define FOO(x) ((x)+1)\n",
		"b/b.canon":     staticImported,
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	res, err := p.Test(context.Background(), canon.TestOptions{Packages: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range res.Check.Findings {
		got = append(got, f.Package+" "+f.Code+" "+f.File)
	}
	want := []string{
		"a " + string(diag.W1003.Def().Code) + " a/a.canon",
		"a " + string(diag.W7101.Def().Code) + " a/data/a.h",
		"b " + string(diag.E2102.Def().Code) + " b/b.canon",
	}
	s := res.Check.Summary
	if !slices.Equal(got, want) || s.Errors != 1 || s.Warnings != len(want)-1 || s.Packages != 1 ||
		!slices.Equal(res.Check.Packages, []string{"a"}) {
		t.Errorf("Check findings %q (want %q), summary %+v, packages %q", got, want, s, res.Check.Packages)
	}
	if res.Passed != 1 || res.Failed != 0 {
		t.Errorf("passed %d failed %d: %+v", res.Passed, res.Failed, res.Tests)
	}
}

// API.md B5, F7, DECISIONS 449: the moved W7101 is kept as the package's bag keeps a finding: under
// MaxFindings 1, a keeps its first finding in F2 order (W1003, in a/a.canon) and counts W7101 as
// dropped; b keeps W3401 and so drops its error, which is still counted.
func TestTestResultCheckTruncatesMovedWarnings(t *testing.T) {
	o := project(map[string]string{
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"a/a.canon":     staticSelected,
		"a/data/a.h":    "#define A_ONE 1\n#define FOO(x) ((x)+1)\n",
		"b/b.canon":     staticImported,
	})
	o.MaxFindings = 1
	p, err := canon.Open("/law", o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	res, err := p.Test(context.Background(), canon.TestOptions{Packages: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range res.Check.Findings {
		got = append(got, f.Package+" "+f.Code)
	}
	s := res.Check.Summary
	want := []canon.Truncation{{Package: "a", Warnings: 1}, {Package: "b", Errors: 1}}
	if !slices.Equal(got, []string{"a " + string(diag.W1003.Def().Code)}) || s.Errors != 1 || s.Warnings != 2 ||
		!slices.Equal(s.Truncated, want) {
		t.Errorf("Check findings %q, summary %+v (want truncated %+v)", got, s, want)
	}
}

// testReads is package a: its let reads a.h, which canon check reads too; its test alone reads b.h.
const testReads = `/// A.
package a

/// The header's codes.
local let codes = load.defines("data/a.h")

/// Warns, as canon check's package checks run.
warn codes.len() > 1 else "one code only"

test "reads both headers" {
  let more = load.defines("data/b.h")
  expect more.len() == 1
  expect codes.len() == 1
}
`

// API.md B5, DECISIONS 449: Check holds canon check's warnings of the selected packages (W5001 of a package-level warn) and the W7101 of a header only a test reads, a header both read reported once.
func TestTestResultCheckHoldsTestReads(t *testing.T) {
	p, err := canon.Open("/law", project(map[string]string{
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"a/a.canon":     testReads,
		"a/data/a.h":    "#define A_ONE 1\n#define FOO(x) ((x)+1)\n",
		"a/data/b.h":    "#define B_ONE 1\n#define BAR(x) ((x)+1)\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	res, err := p.Test(context.Background(), canon.TestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range res.Check.Findings {
		got = append(got, f.Code+" "+f.File)
	}
	w5001, w7101 := string(diag.W5001.Def().Code), string(diag.W7101.Def().Code)
	want := []string{w5001 + " a/a.canon", w7101 + " a/data/a.h", w7101 + " a/data/b.h"}
	if !slices.Equal(got, want) || res.Check.Summary.Warnings != len(want) || res.Failed != 0 {
		t.Errorf("Check findings %q (want %q), summary %+v, failed %d", got, want, res.Check.Summary, res.Failed)
	}
}
