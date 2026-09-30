package workspace

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/fantasim/canonlang/internal/build"
)

// API.md S3 (log-2026-09-29 M4 P14): a snapshot's revision is kept while the files its loads read
// are the same; one computed before a load's read changed them is not kept.
func TestRevisionKept(t *testing.T) {
	s := newSnapFS(&clockFS{MapFS: fstest.MapFS{"f": {Data: []byte("a")}}}, nil, time.Now)
	if _, ok := s.revised(); ok {
		t.Fatal("a revision before any was computed")
	}
	at := s.inputsSeen()
	s.revise("r1", at)
	if rev, ok := s.revised(); !ok || rev != "r1" {
		t.Fatalf("revised = %q, %v", rev, ok)
	}
	s.RecordReads([]build.Read{{Display: "f", Abs: "/f"}})
	if _, ok := s.revised(); ok {
		t.Error("a file a load read keeps the revision")
	}
	s.revise("r2", at) // computed before the load's read
	if _, ok := s.revised(); ok {
		t.Error("a revision computed before a read is kept")
	}
	s.RecordReads([]build.Read{{Display: "f", Abs: "/f"}})
	s.revise("r3", s.inputsSeen())
	if rev, ok := s.revised(); !ok || rev != "r3" {
		t.Errorf("a read already recorded drops the revision: %q, %v", rev, ok)
	}
}

// API.md S5 (log-2026-09-29 M4 P14): a revision is judged from the snapshot's own entries only
// while its newest record holds nothing another snapshot merged into it: a name held from the
// other may differ from what this snapshot reads for it later.
func TestHistoryFrom(t *testing.T) {
	var h history
	one, two := newSnapFS(nil, nil, time.Now), newSnapFS(nil, nil, time.Now)
	one.ents[name{kind: kindFile, abs: "/a"}] = &entry{}
	h.add("r", one)
	if !h.from("r", one) || h.from("r", two) || h.from("q", one) {
		t.Fatal("from after one record")
	}
	one.ents[name{kind: kindFile, abs: "/b"}] = &entry{}
	one.gen++
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
