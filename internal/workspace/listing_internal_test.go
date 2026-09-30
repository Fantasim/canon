package workspace

import (
	"crypto/sha256"
	"maps"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/fantasim/canonlang/internal/build"
)

// ListedInputs is the build inputs of the listing s last computed or inherited, for the tests of
// the package's users.
func ListedInputs(s *Snapshot) []build.Read {
	if l := s.fs.listed(); l != nil {
		return l.inputs
	}
	return nil
}

// API.md S3 (log-2026-09-29 M4 P18): a listing whose displays tie, or whose scan failed, gives
// its lines in the order of the reads, so RevisionOf sorts them as it sorts a listing computed
// whole; otherwise in display order.
func TestListingLines(t *testing.T) {
	key := func(text string) *entry { return &entry{sum: sum{class: classOK, hash: sha256.Sum256([]byte(text))}} }
	missing := &entry{sum: sum{class: classMissing}}
	members := []member{{display: "b", abs: "/1", e: key("x")}, {display: "a", abs: "/2", e: key("y")}, {display: "a", abs: "/3", e: key("z")}, {display: "c", abs: "/4", e: missing}}
	inRead := []build.Listed{{Display: "b", Sum: key("x").sum.hash}, {Display: "a", Sum: key("y").sum.hash}, {Display: "a", Sum: key("z").sum.hash}}
	tied := &listing{members: members, order: []int{1, 2, 0, 3}}
	if got := tied.lines(); !reflect.DeepEqual(got, inRead) {
		t.Errorf("ties: %v", got)
	}
	failed := &listing{members: members[:2], order: []int{1, 0}, scanErr: true}
	if got := failed.lines(); !reflect.DeepEqual(got, append(inRead[:2:2], build.Listed{Display: listingDisplay, Unreadable: true})) {
		t.Errorf("a failed scan: %v", got)
	}
	plain := &listing{members: members[:2], order: []int{1, 0}}
	if got := plain.lines(); !reflect.DeepEqual(got, []build.Listed{inRead[1], inRead[0]}) {
		t.Errorf("display order: %v", got)
	}
}

// API.md S3 (log-2026-09-29 M4 P18): a snapshot and one forked from it share the files loads
// read until either records another, which only it then holds; a listing's set never changes.
func TestInputsShared(t *testing.T) {
	parent := newSnapFS(nil, nil, time.Now)
	parent.RecordReads([]build.Read{{Display: "a", Abs: "/a"}})
	child := parent.fork(nil, nil)
	set, _ := child.recordedFor(nil)
	parent.RecordReads([]build.Read{{Display: "b", Abs: "/b"}})
	child.RecordReads([]build.Read{{Display: "c", Abs: "/c"}})
	if len(parent.recorded()) != 2 || len(child.recorded()) != 2 || len(set.byAbs) != 1 {
		t.Errorf("parent %v, child %v, the listing's %v", parent.recorded(), child.recorded(), set.byAbs)
	}
}

// twins is two histories fed alike, one told from the snapshots' logs, the other by comparing
// every entry; adding to them fails the test once their records differ.
func twins(t *testing.T) func(rev string, fs *snapFS) {
	var logs, whole history
	return func(rev string, fs *snapFS) {
		t.Helper()
		logs.add(rev, fs)
		whole.exact = false // every entry compared
		whole.add(rev, fs)
		if !reflect.DeepEqual(logs.recs, whole.recs) || !maps.Equal(logs.tip, whole.tip) {
			t.Fatalf("%s: from the logs %+v, whole %+v", rev, logs.recs, whole.recs)
		}
	}
}

// API.md S4 (log-2026-09-29 M4 P18, PA3-r): a tip taken before a fork is told from the log the
// snapshot forked from gained up to the fork, and a merge that keeps a name the snapshot dropped
// leaves the tip to be compared whole next time.
func TestHistoryFromLogsAcrossForks(t *testing.T) {
	disk := fstest.MapFS{"a": {Data: []byte("1")}, "b": {Data: []byte("2")}, "c": {Data: []byte("3")}}
	s0 := newSnapFS(&clockFS{MapFS: disk}, nil, time.Now)
	add := twins(t)
	_, _ = s0.ReadFile("/a")
	add("r0", s0)
	_, _ = s0.ReadFile("/b")
	s1 := s0.fork(nil, nil)
	add("r1", s1)
	s2 := s1.fork(nil, map[name]*entry{{kind: kindFile, abs: "/b"}: nil})
	add("r1", s2)
	_, _ = s2.ReadFile("/c")
	add("r2", s2)
}

// API.md S4 (log-2026-09-29 M4 P18): the history's records told from the snapshots' logs are the
// records told by comparing every entry, through reads, merges, forks, reads of the snapshot
// forked from after the fork, a settle and a refresh.
func TestHistoryFromLogs(t *testing.T) {
	disk := fstest.MapFS{"a": {Data: []byte("1")}, "b": {Data: []byte("2")}, "d/e": {Data: []byte("3")}}
	s0 := newSnapFS(&clockFS{MapFS: disk}, nil, time.Now)
	add := twins(t)
	_, _ = s0.ReadFile("/a")
	add("r0", s0)
	_, _ = s0.ReadDir("/d")
	add("r0", s0)
	add("r1", s0)
	s1 := s0.fork(nil, map[name]*entry{{kind: kindFile, abs: "/a"}: nil})
	disk["a"] = &fstest.MapFile{Data: []byte("4")}
	_, _ = s1.ReadFile("/a")
	_, _ = s0.ReadFile("/b") // after the fork: s1 never holds it
	add("r2", s1)
	add("r3", s0)
	add("r4", s1)
	planned := s1.fork(map[string][]byte{"/b": []byte("5")}, nil)
	planned.mine = map[string]bool{"/b": true}
	_, _ = planned.ReadFile("/b")
	_, _ = planned.Stat("/b")
	add("r5", planned)
	planned.settle(nil)
	add("r6", planned)
	disk["d/e"] = &fstest.MapFile{Data: []byte("6"), ModTime: time.Unix(1, 0)}
	disk["d/f"] = &fstest.MapFile{Data: []byte("7")}
	next, _, err := planned.refresh(t.Context())
	if err != nil || next == nil {
		t.Fatalf("refresh: %v, %v", next, err)
	}
	add("r7", next)
	add("r7", planned)
}
