package workspace

import (
	"testing"
	"testing/fstest"
	"time"
)

// API.md S3, S10 (DECISIONS 330): a snapshot has one revision, a function of its files, so the
// first computed is kept; a fork, a new snapshot, starts without one.
func TestRevisionKept(t *testing.T) {
	s := newSnapFS(&clockFS{MapFS: fstest.MapFS{"f": {Data: []byte("a")}}}, nil, time.Now)
	if _, ok := s.revised(); ok {
		t.Fatal("a revision before any was computed")
	}
	s.revise("r1")
	if rev, ok := s.revised(); !ok || rev != "r1" {
		t.Fatalf("revised = %q, %v", rev, ok)
	}
	if _, ok := s.fork(nil, nil).revised(); ok {
		t.Error("a fork inherits its parent's revision")
	}
}

// API.md S5 (log-2026-09-29 M4 P14): a revision is judged from the snapshot's own entries only
// while its newest record holds nothing another snapshot merged into it: a name held from the
// other may differ from what this snapshot reads for it later.
func TestHistoryFrom(t *testing.T) {
	var h history
	one, two := newSnapFS(nil, nil, time.Now), newSnapFS(nil, nil, time.Now)
	put(one, name{kind: kindFile, abs: "/a"}, &entry{})
	h.add("r", one)
	if !h.from("r", one) || h.from("r", two) || h.from("q", one) {
		t.Fatal("from after one record")
	}
	put(one, name{kind: kindFile, abs: "/b"}, &entry{})
	h.add("r", one)
	if !h.from("r", one) {
		t.Error("a record gaining its own snapshot's entries is not its own")
	}
	h.add("r", two)
	if h.from("r", two) || h.from("r", one) {
		t.Error("a record merged from two snapshots is judged from one")
	}
	h.add("q", two)
	if !h.from("q", two) {
		t.Error("a new record is not its snapshot's own")
	}
}
