package build

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
)

// the value canon test passes as its Builder builds package subjects (EVALUATION.md §10.5)
var _ eval.PackageBuilder = subjects{}

const (
	hardErrorCase = "testdata/worlds/hard_error.txtar"
	keptLitsCase  = "testdata/worlds/keptlits.txtar"
	stagesCase    = "testdata/worlds/stages.txtar"
)

// worldRun is canon test over the archive's project, its run kept to read its bags after.
func worldRun(t *testing.T, file string) (*run, *TestResult) {
	t.Helper()
	z := archiveAnalyzer(t, file)
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.prepare(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.test(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return r, res
}

// EVALUATION.md §10.5 items 2 to 4: a world's hard error and kept-literal finding are captured, in no bag of the run.
func TestWorldFindingsStayInTheCapture(t *testing.T) {
	for _, c := range []struct {
		file string
		code diag.Code
	}{
		{hardErrorCase, diag.E4102.Def().Code},
		{keptLitsCase, diag.E3802.Def().Code},
	} {
		t.Run(c.file, func(t *testing.T) {
			r, res := worldRun(t, c.file)
			is := func(f diag.Finding) bool { return f.Code == c.code }
			captured := false
			for _, tc := range res.Tests {
				for _, f := range tc.Failures {
					if f.Expect == "" {
						t.Errorf("%s: stopped by %+v", tc.Name, f)
					}
					captured = captured || slices.ContainsFunc(f.Findings, is)
				}
			}
			if !captured {
				t.Errorf("no failing expect captured %s: %+v", c.code, res.Tests)
			}
			for _, cp := range r.prog.Packages {
				if slices.ContainsFunc(r.host.bags[cp.Path].Findings(), is) || slices.ContainsFunc(r.bags[cp.Path].Findings(), is) {
					t.Errorf("%s: %s outside the capture", cp.Path, c.code)
				}
			}
		})
	}
}

// IMPLEMENTATION-PLAN §4.8, EVALUATION.md §10.5: cancelled while building worlds, canon test completes or reports it.
func TestWorldCancel(t *testing.T) {
	whole := newCut(math.MaxInt)
	want, err := testOutcome(whole, archiveAnalyzer(t, stagesCase))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d context checks", whole.calls)
	for k := 0; k <= whole.calls; k += max(1, whole.calls/cutPoints) {
		got, err := cutRun(testOutcome, newCut(k), archiveAnalyzer(t, stagesCase))
		if err != nil && !errors.Is(err, context.Canceled) || errors.Is(err, ErrInternal) {
			t.Errorf("cut after %d of %d: %v", k, whole.calls, err)
		}
		if err == nil && got != want {
			t.Errorf("cut after %d of %d: an incomplete result without error:\n%s", k, whole.calls, got)
		}
	}
}
