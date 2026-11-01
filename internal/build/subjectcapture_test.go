package build_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"golang.org/x/tools/txtar"
)

const (
	capturedPasses = "passes"    // the outcome of the expect that lists what a subject captured
	captureSource  = "a/a.canon" // the archive file holding the tests
	foreignSource  = "b/b.canon" // the file holding verify_poisoned_foreign.txtar's tests
	testBlock      = "\ntest "   // where a test block starts in it
	poisonedValue  = "bad"       // the poisoned top-level value check_poisoned_read.txtar reads
	poisonedTable  = "things"    // the poisoned table verify_poisoned_read.txtar's refs target
	poisonedLimit  = "limit"     // the poisoned value the unstored_*_poisoned.txtar wheres read
	foreignTable   = "a.things"  // the poisoned table of package a that a world of b reads
	whereField     = "xs"        // the field whose where raises where_path_value.txtar's E4002
	checkSixth     = "sixth"     // the check raising check_hard_*.txtar's E4002
	checkShort     = "short"     // the check failing in check_soft_value.txtar
	worldsDir      = "testdata/worlds/"
	controlTest    = "control: " // a test that fails without reading a poisoned value
)

// EVALUATION.md §10.2 step 3, §10.3, §10.5 item 4, API.md F1: an instance check's E4002 or E5001 is the expect's.
func TestSubjectCapturesCheckFindings(t *testing.T) {
	for _, c := range []struct {
		file, check string
		code        diag.Code
	}{
		{"check_hard_value.txtar", checkSixth, diag.E4002.Def().Code},
		{"check_hard_package.txtar", checkSixth, diag.E4002.Def().Code},
		{"check_soft_value.txtar", checkShort, diag.E5001.Def().Code},
	} {
		t.Run(c.file, func(t *testing.T) {
			res := archiveTest(t, worldsDir+c.file)
			wantVerdicts(t, res, true, false)
			f := capturedBy(res.Tests[1], c.code)
			if f == nil {
				t.Fatalf("%s: want one failing `passes` capturing %s, got %+v", res.Tests[1].Name, c.code, res.Tests[1].Failures)
			}
			if f.Check != c.check {
				t.Errorf("%s: check %q, want %q", c.code, f.Check, c.check)
			}
			if n := len(res.Static.List); n != 0 {
				t.Errorf("the run reports %d errors, want none: %s", n, render(t, res.Static.Findings))
			}
		})
	}
}

// API.md F1: a value subject's where hard error takes the path of the field it refines.
func TestSubjectWherePath(t *testing.T) {
	res := archiveTest(t, worldsDir+"where_path_value.txtar")
	wantVerdicts(t, res, true, false)
	code := diag.E4002.Def().Code
	if f := capturedBy(res.Tests[1], code); f == nil || f.Path != whereField {
		t.Errorf("want %s at path %q captured, got %+v", code, whereField, res.Tests[1].Failures)
	}
}

// API.md F1, P8: a value subject's parts, as verify writes their paths for stage C, start bare.
func TestSubjectPartPaths(t *testing.T) {
	res := archiveTest(t, worldsDir+"subject_paths.txtar")
	wantVerdicts(t, res, false)
	var got []string
	for _, f := range res.Tests[0].Failures {
		for _, x := range f.Findings {
			got = append(got, x.Path)
		}
	}
	if want := []string{"inner", "rows[1]"}; !slices.Equal(got, want) {
		t.Errorf("paths %q, want %q", got, want)
	}
}

// EVALUATION.md §10.5 item 4, §7.3: a check's hard error that taint silences leaves nothing captured.
func TestSubjectTaintSilencesCheck(t *testing.T) {
	res := archiveTest(t, worldsDir+"check_tainted.txtar")
	wantVerdicts(t, res, true)
}

// EVALUATION.md §10.2, §4.1, DECISIONS 426: a poisoned read fails its expect or test, naming it, the same in either order.
func TestSubjectReadsPoisoned(t *testing.T) {
	for _, c := range []struct{ file, src, value string }{
		{"check_poisoned_read.txtar", captureSource, poisonedValue},
		{"verify_poisoned_read.txtar", captureSource, poisonedTable},
		{"verify_poisoned_foreign.txtar", foreignSource, foreignTable},
		{"unstored_subject_poisoned.txtar", captureSource, poisonedLimit},
		{"unstored_condition_poisoned.txtar", captureSource, poisonedLimit},
		{"unstored_statement_poisoned.txtar", captureSource, poisonedLimit},
		{"unstored_substitution_poisoned.txtar", captureSource, poisonedLimit},
	} {
		t.Run(c.file, func(t *testing.T) {
			a, err := txtar.ParseFile(worldsDir + c.file)
			if err != nil {
				t.Fatal(err)
			}
			forward, reversed := testsOf(t, a, c.src, false), testsOf(t, a, c.src, true)
			fails := make([]bool, len(forward.Tests))
			wantVerdicts(t, forward, fails...)
			wantVerdicts(t, reversed, fails...)
			for _, tc := range forward.Tests {
				if strings.HasPrefix(tc.Name, controlTest) {
					continue
				}
				if len(tc.Failures) != 1 || tc.Failures[0].Poisoned != c.value || len(tc.Failures[0].Findings) != 0 {
					t.Errorf("%s: want one failure reading poisoned %s, nothing captured: %+v", tc.Name, c.value, tc.Failures)
				}
			}
			if f, r := failuresByTest(forward), failuresByTest(reversed); !reflect.DeepEqual(f, r) {
				t.Errorf("the test order changed the failures:\nforward  %+v\nreversed %+v", f, r)
			}
		})
	}
}

// failuresByTest is each test's failures, causes included, by name, without their locations.
func failuresByTest(res *build.TestResult) map[string][]build.ExpectFailure {
	out := map[string][]build.ExpectFailure{}
	for _, tc := range res.Tests {
		fs := slices.Clone(tc.Failures)
		for i := range fs {
			fs[i].Loc = source.Location{}
		}
		out[tc.Name] = fs
	}
	return out
}

// testsOf runs the tests of archive a, the test blocks of its file src reversed when reversed.
func testsOf(t *testing.T, a *txtar.Archive, src string, reversed bool) *build.TestResult {
	t.Helper()
	fsys := archiveFS(a)
	if reversed {
		key := "p/" + src
		fsys[key] = file(reverseTests(t, string(fsys[key].Data)))
	}
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Test(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// reverseTests is src with its test blocks, which end it, in reverse order.
func reverseTests(t *testing.T, src string) string {
	t.Helper()
	parts := strings.Split(src, testBlock)
	if len(parts) < 3 {
		t.Fatalf("%d test blocks, want two or more", len(parts)-1)
	}
	blocks := parts[1:]
	slices.Reverse(blocks)
	for i, b := range blocks {
		blocks[i] = strings.TrimRight(b, "\n") + "\n"
	}
	return parts[0] + testBlock + strings.Join(blocks, testBlock)
}

// wantVerdicts fails t unless res's tests pass or fail as want says, in order.
func wantVerdicts(t *testing.T, res *build.TestResult, want ...bool) {
	t.Helper()
	if len(res.Tests) != len(want) {
		t.Fatalf("%d tests, want %d", len(res.Tests), len(want))
	}
	for i, tc := range res.Tests {
		if tc.Passed != want[i] {
			t.Errorf("%s: passed %t, want %t: %+v", tc.Name, tc.Passed, want[i], tc.Failures)
		}
	}
}

// capturedBy is the finding of code that a test's one failure, a `passes` expect, captured; nil for none.
func capturedBy(tc build.TestCase, code diag.Code) *diag.Finding {
	if len(tc.Failures) != 1 || tc.Failures[0].Outcome != capturedPasses {
		return nil
	}
	f := tc.Failures[0]
	i := slices.IndexFunc(f.Findings, func(x diag.Finding) bool { return x.Code == code })
	if i < 0 {
		return nil
	}
	return &f.Findings[i]
}
