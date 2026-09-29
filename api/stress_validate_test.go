package canon_test

import (
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// stressFixture is three states: each edit sets one tier's label.
func stressFixture() (*stressLog, map[canon.Revision]int) {
	log := &stressLog{states: []stressState{
		{labels: []string{"A", "B", "C"}, rev: "r0"},
		{labels: []string{"x", "B", "C"}, rev: "r1"},
		{labels: []string{"x", "y", "C"}, rev: "r2"},
	}}
	index, _ := log.index()
	return log, index
}

// first is labels read of the first tier alone.
func first(label string) []string { return []string{label, "", ""} }

// The gate of TestStress is not weak: validate refuses a stale read, of a revision or of
// content, a reader going back, an unknown revision and a state no edit had reached (API.md S1,
// S10).
func TestStressValidate(t *testing.T) {
	log, index := stressFixture()
	for _, c := range []struct {
		name string
		obs  []observation
		ok   bool
	}{
		{"labels of any state the read overlapped", []observation{{lo: 0, hi: 2, labels: first("A")}, {lo: 0, hi: 2, labels: first("x")}}, true},
		{"the state in flight", []observation{{lo: 1, hi: 2, rev: "r2", labels: []string{"x", "y", "C"}}}, true},
		{"a label an edit returned before replaced", []observation{{lo: 1, hi: 2, labels: first("A")}}, false},
		{"a revision an edit returned before replaced", []observation{{lo: 2, hi: 2, rev: "r1"}}, false},
		{"stale content at a fresh revision", []observation{{lo: 0, hi: 2, rev: "r2", labels: first("A")}}, false},
		{"labels of no one state", []observation{{lo: 0, hi: 2, labels: []string{"A", "y", "C"}}}, false},
		{"going back to an older revision", []observation{{lo: 0, hi: 2, rev: "r1"}, {lo: 0, hi: 2, rev: "r0"}}, false},
		{"going back to an older label", []observation{{lo: 0, hi: 2, labels: []string{"", "y", ""}}, {lo: 0, hi: 2, labels: first("A")}}, false},
		{"a revision no edit made", []observation{{lo: 0, hi: 2, rev: "r9"}}, false},
		{"a revision not yet in flight", []observation{{lo: 0, hi: 1, rev: "r2"}}, false},
		{"a label not yet in flight", []observation{{lo: 0, hi: 1, labels: []string{"", "y", ""}}}, false},
	} {
		if err := log.validate(index, c.obs); (err == nil) != c.ok {
			t.Errorf("%s: %v, want ok %v", c.name, err, c.ok)
		}
	}
}

// The watcher's gate refuses an event of another cause, with an error or an error finding, a
// gap, a repeat, a missing event and fn running twice at once (API.md W13, W15).
func TestStressEvents(t *testing.T) {
	_, index := stressFixture()
	edit := func(rev canon.Revision) canon.Event { return canon.Event{Cause: canon.CauseEdit, Revision: rev} }
	failed, erring := edit("r2"), edit("r2")
	failed.Err = errors.New("re-check failed")
	erring.Summary.Errors = 1
	for _, c := range []struct {
		name    string
		events  []canon.Event
		overlap bool
		ok      bool
	}{
		{"every edit once, in order", []canon.Event{edit("r1"), edit("r2")}, false, true},
		{"an external event", []canon.Event{edit("r1"), {Cause: canon.CauseExternal, Revision: "r2"}}, false, false},
		{"an event with an error", []canon.Event{edit("r1"), failed}, false, false},
		{"an event with an error finding", []canon.Event{edit("r1"), erring}, false, false},
		{"a gap", []canon.Event{edit("r2")}, false, false},
		{"out of order", []canon.Event{edit("r2"), edit("r1")}, false, false},
		{"a repeat", []canon.Event{edit("r1"), edit("r1"), edit("r2")}, false, false},
		{"a missing last event", []canon.Event{edit("r1")}, false, false},
		{"fn twice at once", []canon.Event{edit("r1"), edit("r2")}, true, false},
	} {
		if err := checkEvents(index, c.events, len(index)-1, c.overlap); (err == nil) != c.ok {
			t.Errorf("%s: %v, want ok %v", c.name, err, c.ok)
		}
	}
}
