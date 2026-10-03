package rules

import (
	"slices"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// entries visits a top-level table's entries in order, through the memo (IMPLEMENTATION-PLAN §7.6).
func (t *traversal) entries(tv *value.Table) {
	for _, e := range tv.Entries {
		if e == nil || e.Ident == nil {
			continue
		}
		t.segs = append(t.segs, seg{form: segEntry, key: e.Ident.Key})
		t.entry(e, nil)
		t.segs = t.segs[:len(t.segs)-1]
	}
}

// elements visits a top-level list's elements in order, a record through the memo (NFR-02).
func (t *traversal) elements(l *value.List, dt types.Type) {
	namedParts(l, dt, t.lateFrom(), func(p part) bool {
		t.segs = append(t.segs, p.s)
		if rec, ok := p.v.(*value.Record); ok {
			t.entry(rec, p.t)
		} else {
			t.visit(p.v, p.t)
		}
		t.segs = t.segs[:len(t.segs)-1]
		return true
	})
}

// entry visits e, declared dt, replayed when the memo keeps stage C in it at the same segment
// and its marks and reads are the same, else visited and kept when a replay can reproduce it.
func (t *traversal) entry(e *value.Record, dt types.Type) {
	token, ok := t.memo.ev.EntryToken(e)
	if !ok || t.seen[e] || t.ctx.Err() != nil || !t.owns(token) {
		t.visit(e, dt)
		return
	}
	kept, _ := t.memo.ev.Attached(token, eval.Checked).(*entryKept)
	if kept != nil && kept.root == t.rootOf && kept.seg == t.segs[len(t.segs)-1] && t.replayEntry(e, kept) {
		// nothing marked seen: an owned entry of a value alone is reached by no other value traversed
		t.memo.replayed++
		return
	}
	t.recordEntry(token, e, dt)
}

// owns reports token's record made by the evaluation of the value traversed: no value traversed
// before holds it, since one made before could not (log-2026-09-29 M4 P12-r).
func (t *traversal) owns(token any) bool {
	owner, ok := t.memo.ev.TokenOwner(token)
	return ok && owner == t.rootOf
}

// recordEntry visits e, keeping each check run and the entry's invalid values on token.
func (t *traversal) recordEntry(token any, e *value.Record, dt types.Type) {
	marks, count := t.signature(e), t.memo.ev.InvalidCount()
	rec := &entryRec{}
	t.rec = rec
	t.visit(e, dt)
	t.rec = nil
	if rec.void || t.ctx.Err() != nil || t.memo.ev.InvalidCount() != count {
		return
	}
	rec.kept.root, rec.kept.seg, rec.kept.marks = t.rootOf, t.segs[len(t.segs)-1], marks
	t.memo.ev.Attach(token, eval.Checked, &rec.kept, rec.kept.nodes())
}

// replayEntry replays stage C in e when its invalid values are the kept ones and its check runs
// replay: their steps charged, findings reported, failed checks reported at their instances.
// False: nothing was done.
func (t *traversal) replayEntry(e *value.Record, kept *entryKept) bool {
	if !slices.Equal(t.signature(e), kept.marks) {
		return false
	}
	var instances []*value.Record
	if len(kept.failed) > 0 {
		instances = instancesOf(e)
		for _, f := range kept.failed {
			if f.instance >= len(instances) {
				return false // one token's graphs are one shape: never
			}
		}
	}
	if !t.memo.ev.ReplayChecks(t.ctx, kept.runs) {
		return false
	}
	for _, f := range kept.failed {
		run, self := runOf(kept.runs[f.run].Outcome()), instances[f.instance]
		t.report(f.c, run, self, f.at)
	}
	return true
}

// runOf is an evaluator's check run as the runner reads it.
func runOf(x eval.CheckRun) Run {
	out := Run{Aborted: x.Aborted, Failed: x.Failed, Message: x.Message}
	for _, rep := range x.Reports {
		out.Reports = append(out.Reports, Report(rep))
	}
	return out
}

// signature is the places of e's invalid values, its parts walked as a tree; nil for none.
func (t *traversal) signature(e value.Value) []int {
	if t.memo.ev.InvalidCount() == 0 {
		return nil
	}
	s := &signing{ev: t.ev}
	s.walk(e)
	return s.marks
}

// signing walks a value's parts as a tree, noting the place of each invalid one.
type signing struct {
	ev    Evaluator
	n     int
	marks []int
}

func (s *signing) walk(v value.Value) {
	if v != nil && s.ev.Invalid(v) {
		s.marks = append(s.marks, s.n)
	}
	s.n++
	eachPart(v, func(p value.Value) bool {
		s.walk(p)
		return true
	})
}

// instancesOf is the instances of e in the order stage C visits them.
func instancesOf(e value.Value) []*value.Record {
	c := &collecting{seen: map[*value.Record]bool{}}
	c.walk(e)
	return c.out
}

// collecting walks a value's parts as stage C does, keeping each instance at its first reach.
type collecting struct {
	seen map[*value.Record]bool
	out  []*value.Record
}

func (c *collecting) walk(v value.Value) {
	if rec, ok := v.(*value.Record); ok {
		if c.seen[rec] {
			return
		}
		c.seen[rec] = true
		c.out = append(c.out, rec)
	}
	eachPart(v, func(p value.Value) bool {
		c.walk(p)
		return true
	})
}
