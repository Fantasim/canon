package build_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

const blockedLine = "table  teamboard.statuses  blocked"

// Edits of examples/teamboard/taxonomy.canon: a status added, then also reached and shown.
var (
	addStatus = []string{
		"  wont_do { tone: neutral, label: \"Won't do\", terminal: true, next: [open], requires: [reason] }\n",
		"  wont_do { tone: neutral, label: \"Won't do\", terminal: true, next: [open], requires: [reason] }\n" +
			"  blocked { tone: neutral, label: \"Blocked\", terminal: true, next: [open] }\n",
	}
	showStatus = append(slices.Clone(addStatus), "statuses: [verified, wont_do, duplicate]", "statuses: [verified, wont_do, duplicate, blocked]",
		"next: [taken, wont_do, duplicate] }", "next: [taken, wont_do, duplicate, blocked] }")
	boom = []string{"let initialStatus:", "/// Boom.\nlet boom: Int = 1 / 0\n\n/// The first status.\nlet initialStatus:"}
)

// LOCK.md §8, API.md B4: only stable collections evaluated, no check run; a new value is W6006.
func TestLockCheck(t *testing.T) {
	for _, c := range []struct {
		name  string
		edits []string
		codes []diag.Code
	}{
		{"unchanged", nil, nil},
		{"added", addStatus, []diag.Code{diag.W6006.Def().Code}},
		{"renamed", []string{"wont_do", "rejected"}, []diag.Code{diag.W6006.Def().Code, diag.E6001.Def().Code}},
		{"other let fails", boom, nil},
	} {
		p, err := build.Open(teamboardFS(t, c.edits...), "/ex", build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		res, err := p.LockCheck(context.Background(), []string{"teamboard"})
		if err != nil {
			t.Fatal(err)
		}
		if got := codes(res.List); !slices.Equal(got, c.codes) {
			t.Errorf("%s: lock check found %v, want %v:\n%s", c.name, got, c.codes, render(t, res.Findings))
		}
	}
}

// refProbe is a stable table whose entries name an entry of a table that fails to evaluate.
const refProbe = "/// A.\npackage a\n\n/// A thing.\nrecord Thing {\n  /// N.\n  n: Int = 0\n}\n\n/// Things.\n" +
	"let things: table Thing = { t1 { n: 1 / 0 } }\n\n/// A row.\nrecord Row {\n  /// Who.\n  who: ref Thing\n}\n\n" +
	"/// Rows.\nlet rows: stable table Row = { r1 { who: t1 } }\n"

// LOCK.md §8 step 2 (log-2026-09-29 M4 U8-r): lock check verifies no ref; check does.
func TestLockCheckVerifiesNoRef(t *testing.T) {
	fsys := mapFS{"p/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"), "p/a/a.canon": file(refProbe)}
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	lc, err := p.LockCheck(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if got := codes(lc.List); !slices.Equal(got, []diag.Code{diag.W6006.Def().Code}) {
		t.Errorf("lock check found %v, want only the values not locked yet:\n%s", got, render(t, lc.Findings))
	}
	res, err := p.Check(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(codes(res.List), diag.E4102.Def().Code) {
		t.Errorf("check does not report the failing thing:\n%s", render(t, res.Findings))
	}
}

// LOCK.md §8: the error of a let lock check does not evaluate is one a check reports.
func TestLockCheckEvaluatesLess(t *testing.T) {
	p, err := build.Open(teamboardFS(t, boom...), "/ex", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), []string{"teamboard"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(codes(res.List), diag.E4102.Def().Code) {
		t.Errorf("check does not report the let lock check skips:\n%s", render(t, res.Findings))
	}
}

// API.md E20, LOCK.md §5, §6.2: the lines an analysis adds to canon.lock are a build's.
func TestLockUpdates(t *testing.T) {
	retire := []string{"  duplicate {", "  retired duplicate {", "wont_do, duplicate]", "wont_do]"}
	for _, c := range []struct {
		name  string
		edits []string
		lines []string
	}{
		{"unchanged", nil, nil},
		{"added", showStatus, []string{blockedLine}},
		{"retired", retire, []string{"table  teamboard.statuses  duplicate  retired"}},
		{"error", []string{"wont_do", "rejected"}, nil},
	} {
		p, err := build.Open(teamboardFS(t, c.edits...), "/ex", build.Options{})
		if err != nil {
			t.Fatal(err)
		}
		a, err := p.Analyze(context.Background(), []string{"teamboard"})
		if err != nil {
			t.Fatal(err)
		}
		first, err := a.LockUpdates()
		if err != nil {
			t.Fatal(err)
		}
		again, _ := a.LockUpdates()
		if !reflect.DeepEqual(first, again) {
			t.Errorf("%s: a second LockUpdates differs", c.name)
		}
		if got := lockLines(first); !slices.Equal(got, c.lines) {
			t.Errorf("%s: lines %v, want %v:\n%s", c.name, got, c.lines, render(t, a.Result().Findings))
		}
		sameAsBuild(t, c.name, p, first)
	}
}

// lockLines is the lines every lock adds, in order.
func lockLines(locks []build.Lock) []string {
	var out []string
	for _, l := range locks {
		out = append(out, l.Lines...)
	}
	return out
}

// sameAsBuild fails when a Check-mode build would write other locks than updates.
func sameAsBuild(t *testing.T, name string, p *build.Project, updates []build.Lock) {
	t.Helper()
	res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{"teamboard"}, Targets: goAndJSON, Check: true})
	if err != nil {
		t.Fatal(err)
	}
	var stale []build.Lock
	for _, l := range res.Locks {
		if l.Status == build.StatusStale {
			l.Status = build.StatusWritten
			stale = append(stale, l)
		}
	}
	if !reflect.DeepEqual(stale, updates) {
		t.Errorf("%s: LockUpdates %+v, a build writes %+v", name, updates, stale)
	}
}
