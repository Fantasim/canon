package gostyle

import (
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
)

const wantBigFileLines = 512

func TestSizeRules(t *testing.T) {
	fs := runFixture(t, "testdata/size")
	counts := countByRule(fs)

	wantCounts := map[string]int{
		ruleFnLength:    2,
		ruleFnParams:    1,
		ruleFnResults:   1,
		ruleFnNesting:   1,
		ruleNakedReturn: 1,
		ruleFileLength:  1,
	}
	for rule, want := range wantCounts {
		if counts[rule] != want {
			t.Errorf("%s: got %d findings, want %d (%+v)", rule, counts[rule], want, only(fs, rule))
		}
	}
	if got := counts[rulePkgSize]; got != 0 {
		t.Errorf("pkg-size: got %d findings in a small package, want 0", got)
	}

	assertValue(t, fs, ruleFnParams, 6)
	assertValue(t, fs, ruleFnResults, 4)
	assertValue(t, fs, ruleFnNesting, 4)
	assertValue(t, fs, ruleNakedReturn, 1)
	assertValue(t, fs, ruleFileLength, wantBigFileLines)
}

func TestPkgSizeRule(t *testing.T) {
	fs := runFixture(t, "testdata/pkgsize")
	got := only(fs, rulePkgSize)
	if len(got) != 1 {
		t.Fatalf("pkg-size: got %d findings, want 1 (%+v)", len(got), got)
	}
	if got[0].File != "." {
		t.Errorf("pkg-size: File = %q, want %q (the package dir)", got[0].File, ".")
	}
}

// assertValue fails unless exactly one finding of rule has the given Value.
func assertValue(t *testing.T, fs []finding.Finding, rule string, want int) {
	t.Helper()
	got := only(fs, rule)
	if len(got) != 1 {
		t.Fatalf("%s: got %d findings, want 1", rule, len(got))
	}
	if got[0].Value != want {
		t.Errorf("%s: Value = %d, want %d", rule, got[0].Value, want)
	}
}

func TestPkgDocRules(t *testing.T) {
	if fs := runFixture(t, "testdata/pkgdoc"); len(only(fs, rulePkgDoc))+len(only(fs, rulePkgExample)) != 0 {
		t.Errorf("a package with doc.go and an Example got findings: %+v", fs)
	}
	fs := runFixture(t, "testdata/size")
	if doc, ex := only(fs, rulePkgDoc), only(fs, rulePkgExample); len(doc) != 1 || len(ex) != 1 || doc[0].File != "." {
		t.Errorf("pkg-doc = %+v, pkg-example = %+v, want one of each on \".\"", doc, ex)
	}
}
