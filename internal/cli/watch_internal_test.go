package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// errRerun is a failure of a re-check that carries no findings.
var errRerun = errors.New("rerun failed")

// newWatcher is a watcher over an invocation writing to the returned buffers, its previous state the given one.
func newWatcher(ctx context.Context, prev *cycleState, rerun func() (*cycleState, error)) (*watcher, *bytes.Buffer, *bytes.Buffer) {
	var out, errs bytes.Buffer
	inv := &invocation{ctx: ctx, env: Env{Stdout: &out, Stderr: &errs}, opt: newOptions()}
	return &watcher{inv: inv, prev: prev, rerun: rerun}, &out, &errs
}

// cleanState is a check's state without findings, its build having written the files given.
func cleanState(wrote ...string) *cycleState {
	s := &cycleState{check: &canon.CheckResult{}, wrote: map[string]bool{}}
	for _, f := range wrote {
		s.wrote[f] = true
	}
	return s
}

// meta/decisions/log-2026-09-29.md "M4 U6-r" 4: an event carrying an error and files reports the error, then runs again.
func TestWatchEventErrorRuns(t *testing.T) {
	runs := 0
	w, out, errs := newWatcher(context.Background(), cleanState(), func() (*cycleState, error) { runs++; return cleanState(), nil })
	if err := w.handle(canon.Event{Err: errRerun, Files: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || !strings.Contains(errs.String(), "rerun failed") || !strings.HasPrefix(out.String(), "-- 1 files changed") {
		t.Errorf("runs %d, stdout %q, stderr %q", runs, out.String(), errs.String())
	}
}

// meta/decisions/log-2026-09-29.md "M4 U6-r" 6: an interrupt during a run prints nothing, so Main's line is the only one.
func TestWatchInterruptDuringRerun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w, out, errs := newWatcher(ctx, cleanState(), func() (*cycleState, error) { cancel(); return nil, ctx.Err() })
	if err := w.handle(canon.Event{Files: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || errs.Len() != 0 {
		t.Errorf("stdout %q, stderr %q", out.String(), errs.String())
	}
}

// meta/decisions/log-2026-09-29.md "M4 U6-r" 1: a run that fails is a cycle whose findings are added and, once the run works again, fixed.
func TestWatchFailedRunIsACycle(t *testing.T) {
	pe := &canon.ProjectError{Err: canon.ErrNoProject, Findings: []canon.Finding{finding("X9", "project.canon", "", "m", 3)}}
	failing := true
	w, out, _ := newWatcher(context.Background(), cleanState(), func() (*cycleState, error) {
		if failing {
			return nil, pe
		}
		return cleanState(), nil
	})
	if err := w.handle(canon.Event{Files: []string{"project.canon"}}); err != nil {
		t.Fatal(err)
	}
	failing = false
	if err := w.handle(canon.Event{Files: []string{"project.canon"}}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"error[X9]  project.canon:3:1", "1 error, 0 warnings in 0 packages", "fixed: error[X9]  project.canon:3:1"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("no %q in %q", want, out.String())
		}
	}
}

// meta/decisions/log-2026-09-29.md "M4 U6-r" 1: a failed run proves nothing about the findings before it: through two failures, the first one's findings are removed by the second, and the held finding is never printed again.
func TestWatchFailuresHoldEarlierFindings(t *testing.T) {
	held := finding("X8", "a/a.canon", "", "held", 4)
	good := cleanState()
	good.shown = []canon.Finding{held}
	steps := []error{
		&canon.ProjectError{Err: canon.ErrNoProject, Findings: []canon.Finding{finding("X9", "project.canon", "", "first", 3)}},
		&canon.ProjectError{Err: canon.ErrNoProject, Findings: []canon.Finding{finding("X7", "project.canon", "", "second", 3)}},
		nil,
	}
	next := 0
	w, out, _ := newWatcher(context.Background(), good, func() (*cycleState, error) {
		err := steps[next]
		next++
		if err != nil {
			return nil, err
		}
		return good, nil
	})
	for range steps {
		if err := w.handle(canon.Event{Files: []string{"project.canon"}}); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(out.String(), "X8") || !strings.Contains(out.String(), "fixed: error[X9]") || !strings.Contains(out.String(), "fixed: error[X7]") {
		t.Errorf("stdout %q", out.String())
	}
}

// meta/decisions/log-2026-09-29.md "M4 U6-r" 2: one run answers an event the run's own writes caused; the next such event prints one line, the ones after it nothing, until a change from outside.
func TestWatchOwnWritesRunOnce(t *testing.T) {
	runs := 0
	w, out, _ := newWatcher(context.Background(), cleanState("@out/x"), func() (*cycleState, error) { runs++; return cleanState("@out/x"), nil })
	own, outside := canon.Event{Files: []string{"@out/x"}}, canon.Event{Files: []string{"a", "@out/x"}}
	for _, ev := range []canon.Event{own, own, own} {
		if err := w.handle(ev); err != nil {
			t.Fatal(err)
		}
	}
	if runs != 1 || strings.Count(out.String(), watchUnsettled) != 1 {
		t.Errorf("runs %d, stdout %q", runs, out.String())
	}
	if err := w.handle(outside); err != nil || runs != 2 {
		t.Fatalf("outside event: runs %d, %v", runs, err)
	}
	if err := w.handle(own); err != nil || runs != 3 {
		t.Errorf("own event after a change from outside: runs %d, %v", runs, err)
	}
}

// IMPLEMENTATION-PLAN.md §8.1, log-2026-09-29.md "M4 U6-r2", DECISIONS 341
func TestWatchUnsettledJSON(t *testing.T) {
	w, out, _ := newWatcher(context.Background(), cleanState("@out/x"), func() (*cycleState, error) { return cleanState("@out/x"), nil })
	w.inv.opt.format = formatJSON
	own := canon.Event{Files: []string{"@out/x"}}
	for _, ev := range []canon.Event{own, own, own} {
		if err := w.handle(ev); err != nil {
			t.Fatal(err)
		}
	}
	if got := out.String(); strings.Count(got, `"settled":false`) != 1 || !strings.HasSuffix(strings.TrimSpace(got), "}}") || !strings.Contains(got, `"summary"`) {
		t.Errorf("stdout %q", got)
	}
}

// meta/decisions/log-2026-09-29.md "M4 U6-r" 7: events waiting together are one, their files and packages merged in order.
func TestWatchQueuedEventsMerge(t *testing.T) {
	events := make(chan canon.Event, 3)
	events <- canon.Event{Files: []string{"b"}, Packages: []string{"q"}, Revision: "r2"}
	events <- canon.Event{Files: []string{"a", "b"}, Packages: []string{"p"}, Revision: "r3"}
	got := queued(events, canon.Event{Files: []string{"c"}, Revision: "r1"})
	if strings.Join(got.Files, ",") != "a,b,c" || strings.Join(got.Packages, ",") != "p,q" || got.Revision != "r3" || len(events) != 0 {
		t.Errorf("got %+v", got)
	}
}

// finding is an error finding of code at line, its file, path and message as given.
func finding(code, file, path, message string, line int) canon.Finding {
	return canon.Finding{Severity: canon.SeverityError, Code: code, Span: canon.Span{File: file, Line: line, Col: 1}, Path: path, Message: message}
}

// IMPLEMENTATION-PLAN.md §8.1: findings match between cycles by code, file, path and message, one counterpart each; a moved finding matches.
func TestUnmatched(t *testing.T) {
	a1 := finding("E1", "a", "p", "m", 1)
	a1Moved := finding("E1", "a", "p", "m", 9)
	otherFile := finding("E1", "b", "p", "m", 1)
	otherMessage := finding("E1", "a", "p", "n", 1)
	tests := []struct {
		name string
		a, b []canon.Finding
		want []canon.Finding
	}{
		{"same", []canon.Finding{a1}, []canon.Finding{a1}, nil},
		{"moved", []canon.Finding{a1Moved}, []canon.Finding{a1}, nil},
		{"file", []canon.Finding{otherFile}, []canon.Finding{a1}, []canon.Finding{otherFile}},
		{"message", []canon.Finding{otherMessage}, []canon.Finding{a1}, []canon.Finding{otherMessage}},
		{"repeated", []canon.Finding{a1, a1Moved}, []canon.Finding{a1}, []canon.Finding{a1Moved}},
		{"none", nil, []canon.Finding{a1}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := unmatched(tc.a, tc.b)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i].Line != tc.want[i].Line || got[i].File != tc.want[i].File || got[i].Message != tc.want[i].Message {
					t.Errorf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// IMPLEMENTATION-PLAN.md §8.1: `fixed: <severity>[<CODE>]  <file:line:col>`; a finding without a file has no location.
func TestFixedLine(t *testing.T) {
	if got, want := fixedLine(finding("X9", "a/a.canon", "", "m", 3)), "fixed: error[X9]  a/a.canon:3:1"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := fixedLine(finding("X9", "", "", "m", 0)), "fixed: error[X9]"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// IMPLEMENTATION-PLAN.md §8.1: a finding's line in a cycle is its own line (API.md F5) with the change last.
func TestChangedFindingJSON(t *testing.T) {
	data, err := json.Marshal(changedFinding{finding: finding("E1", "a", "", "m", 2), change: watchRemoved})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"severity":"error","code":"E1","file":"a","line":2,"col":1,"endLine":0,"endCol":0,"package":"","message":"m","change":"removed"}`
	if string(data) != want {
		t.Errorf("got  %s\nwant %s", data, want)
	}
}
