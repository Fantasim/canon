package eval

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
)

// stagedEntry is an evaluator using a new memo in epoch 1, and an entry kept in its store.
func stagedEntry(t *testing.T) (*Evaluator, *memoEntry) {
	t.Helper()
	m := NewMemo()
	e := &Evaluator{memo: &memoUse{m: m, gen: m.begin(1, liveness{})}}
	en := &memoEntry{size: memoNodeBytes}
	e.memo.store(memoKey{decl: &syntax.EntryDecl{}}, en)
	return e, en
}

// log-2026-09-29 M4 P12-r: what stages B and C keep on an entry is counted in its bytes and replaced, not added twice.
func TestAttachCounted(t *testing.T) {
	e, en := stagedEntry(t)
	g := e.memo.gen
	if !e.Attach(en, Verified, "b", 2) || !e.Attach(en, Checked, "c", 3) || !e.Attach(en, Checked, "c2", 1) {
		t.Fatal("attach refused on a kept entry")
	}
	want := memoNodeBytes * (1 + 2 + 1)
	if en.size != want || g.bytes != want || e.Attached(en, Verified) != "b" || e.Attached(en, Checked) != "c2" {
		t.Errorf("entry %d bytes, store %d, want %d; attached %v %v", en.size, g.bytes, want, e.Attached(en, Verified), e.Attached(en, Checked))
	}
	if e.Attach(en, Verified, "big", memoBytes) || e.Attached(en, Verified) != nil {
		t.Error("a stage result past memoBytes: want it refused")
	}
}

// log-2026-09-29 M4 P12-r: an entry evicted, or its epoch forgotten, keeps no stage result.
func TestAttachForgotten(t *testing.T) {
	e, en := stagedEntry(t)
	e.Attach(en, Checked, "c", 1)
	e.memo.store(memoKey{decl: &syntax.EntryDecl{}}, &memoEntry{size: memoBytes}) // evicts en
	if e.Attached(en, Checked) != nil || e.Attach(en, Checked, "c", 1) {
		t.Error("evicted entry: want no stage result, none attached")
	}
	e, en = stagedEntry(t)
	e.Attach(en, Verified, "b", 1)
	if e.memo.m.Forget(1); e.Attached(en, Verified) != nil || e.Attach(en, Verified, "b", 1) {
		t.Error("forgotten epoch: want no stage result, none attached")
	}
	if e.Attached(e, Verified) != nil || (&Evaluator{}).Attached(en, Verified) != nil {
		t.Error("no token, no memo: want nothing")
	}
}

// EVALUATION.md §3.3, IMPLEMENTATION-PLAN §7.6: a replay past the frame limit is refused, nothing charged.
func TestReplayChecksDepth(t *testing.T) {
	e, _ := stagedEntry(t)
	e.index = &index{pkg: map[*syntax.File]string{}}
	tr := &CheckTrace{charge: charge{pkg: "a", name: "c"}, steps: 3, need: 2}
	e.counter = &counter{budget: 1 << 20, spent: map[charge]int64{}}
	if e.depth = maxDepth - 1; e.ReplayChecks(context.Background(), []*CheckTrace{tr}) || e.steps != 0 {
		t.Error("past the frame limit: want the replay refused, no step charged")
	}
	if e.depth = 0; !e.ReplayChecks(context.Background(), []*CheckTrace{tr}) || e.steps != tr.steps {
		t.Errorf("within it: want the replay made, %d steps charged", tr.steps)
	}
}
