package build

import (
	"context"
	"fmt"
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const (
	refTargetsCase   = "testdata/incremental/reftargets.txtar"
	refTargetsWorlds = "testdata/incremental/reftargets_worlds.txtar"
	refTargetsVector = "testdata/incremental/reftargets_vector.txtar"
	refTargetsFile   = "a/a.canon"
	refTargetsGem    = "  gem { label: \"Gem\" }\n"
	refTargetsVecPkg = "b"
)

// EVALUATION.md §3.4, §10.5; DECISIONS 515: neither a world's tables nor a test literal's own nodes join the run's targets.
func TestRefTargetsWorldsAndFieldsApart(t *testing.T) {
	z := archiveAnalyzer(t, refTargetsWorlds)
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	r, err := p.prepare(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.test(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range res.Tests {
		if !tc.Passed {
			t.Errorf("%s failed: %+v", tc.Name, tc.Failures)
		}
	}
	if n := r.vix.Indexed(); n != 1 {
		t.Errorf("the run's targets hold %d collections, want 1: rewards", n)
	}
}

// ADR-0003, CONFORMANCE.md §6.5; DECISIONS 515: a vector's evaluator's targets stay out of the run's.
func TestRefTargetsVectorApart(t *testing.T) {
	z := archiveAnalyzer(t, refTargetsVector)
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{refTargetsVecPkg})
	if err != nil {
		t.Fatal(err)
	}
	if n := a.r.vix.Indexed(); n != 0 {
		t.Errorf("the run's targets hold %d collections, want none: only a vector read a.initial", n)
	}
}

// verdicts is what a canon test run says of each test: its verdict, then each failure's findings.
func verdicts(t *testing.T, z *analyzer, warm bool) string {
	t.Helper()
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	if warm {
		p = p.WithCache(z.cache)
	}
	res, err := p.Test(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, tc := range res.Tests {
		fmt.Fprintf(&sb, "%s passed %t\n", tc.Name, tc.Passed)
		for _, f := range tc.Failures {
			for _, x := range f.Findings {
				fmt.Fprintf(&sb, "  %s %s %s\n", x.Code, x.Path, x.Message)
			}
		}
	}
	return sb.String()
}

// EVALUATION.md §5; DECISIONS 446, 515: a warm canon test re-indexes a ref target an edit changed.
func TestRefTargetsReindexedAfterEdit(t *testing.T) {
	z := archiveAnalyzer(t, refTargetsCase)
	abs := path.Join(archiveRoot, refTargetsFile)
	orig, err := z.fs.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	dangling := diag.E3501.Def().Code
	for _, step := range []struct {
		name     string
		text     string
		dangling bool
	}{
		{"first", string(orig), false},
		{"gem removed", strings.Replace(string(orig), refTargetsGem, "", 1), true},
		{"gem back", string(orig), false},
	} {
		z.fs.set(abs, []byte(step.text))
		warm, cold := verdicts(t, z, true), verdicts(t, z, false)
		if warm != cold {
			t.Fatalf("%s: warm differs from cold:\nwarm:\n%scold:\n%s", step.name, warm, cold)
		}
		if got := strings.Contains(warm, string(dangling)); got != step.dangling {
			t.Errorf("%s: %s found %t, want %t:\n%s", step.name, dangling, got, step.dangling, warm)
		}
	}
}
