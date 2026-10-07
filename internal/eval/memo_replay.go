package eval

import (
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

// replayOutcome is where a replay stands after one of its steps.
type replayOutcome uint8

// replayEntry replays the entry kept for k: its steps around a new forcing of each value it
// read, its findings, marks and record. Up to a read that differs it did what evaluating does;
// there hit is false and the entry's steps are taken back, for the caller to evaluate it.
func (r *run) replayEntry(k memoKey) (rec *value.Record, hit bool) {
	e := r.ev
	en := e.memo.lookup(k)
	if en == nil || !e.canReplay(en) {
		return nil, false
	}
	start := e.spent[r.charge]
	infos, out, said := r.replayReads(en)
	var tab []value.Value
	if out == replayOn {
		if tab, hit = en.kept.seed(infos); !hit {
			out = replayMiss
		}
	}
	switch out {
	case replayMiss:
		r.rollback(start)
		return nil, false
	case replayStop:
		r.reportKept(en.found, said)
		return nil, true
	case replayOn:
	}
	e.memo.stats.hits++
	r.reportKept(en.found, said)
	if en.kept.root == nilSlot {
		r.failed = true
		return nil, true
	}
	rec, _ = e.thaw(&en.kept, tab).(*value.Record)
	e.memo.noteToken(rec, en)
	return rec, true
}

// replayReads charges each step segment and forces each value read, then the last segment:
// the values read, how the replay stands, and how many reads the findings to report precede.
func (r *run) replayReads(en *memoEntry) ([]*readInfo, replayOutcome, int) {
	infos := make([]*readInfo, 0, len(en.reads))
	for i := range en.reads {
		rd := &en.reads[i]
		if out := r.replaySteps(rd.steps); out != replayOn {
			return infos, out, i
		}
		in, out := r.replayForce(rd)
		if out != replayOn {
			return infos, out, i + 1
		}
		infos = append(infos, in)
	}
	return infos, r.replaySteps(en.tail), len(en.reads) + 1
}

// reportKept reports, in the order the entry did, the findings it reported before its read said.
func (r *run) reportKept(found []memoFinding, said int) {
	for _, f := range found {
		if f.read < said {
			r.ev.report(f.pkg, f.b)
		}
	}
}

// canReplay reports that en's calls stay under the depth limit here and its code is the program's.
func (e *Evaluator) canReplay(en *memoEntry) bool {
	if e.depth+en.need > maxDepth {
		return false
	}
	for _, f := range en.files {
		if _, live := e.index.pkg[f]; !live {
			return false
		}
	}
	return true
}

// replayForce forces the value read, as deep in frames as the entry read it, and compares it;
// a poisoned one stops the entry silently, as the read does.
func (r *run) replayForce(rd *memoRead) (*readInfo, replayOutcome) {
	e := r.ev
	st := e.rootState(rd.root)
	if st == nil || st.status == forcing {
		return nil, replayMiss
	}
	depth, implicit := e.depth, e.implicit
	e.depth, e.implicit = depth+rd.depth, implicit+rd.implicit
	v, ok := e.force(r.ctx, st, r, nil)
	e.depth, e.implicit = depth, implicit
	if !ok {
		r.stop()
		return nil, replayStop
	}
	in := e.memo.info(e, v)
	if !in.ok || in.fp != rd.fp || !in.cleanIn(e) {
		return nil, replayMiss
	}
	return in, replayOn
}

// replaySteps charges n steps at once, unless they would reach the package's budget: evaluating the
// entry reports E4401 at its own expression.
func (r *run) replaySteps(n int64) replayOutcome {
	e := r.ev
	if !e.fits(r.charge.pkg, n) || int64(int(n)) != n {
		return replayMiss
	}
	if !r.spend(int(n), noSpan) {
		return replayStop // cancelled
	}
	return replayOn
}

// noSpan is where a replayed charge would report E4401: never, the budget checked first.
func noSpan() source.Span {
	return source.Span{}
}

// rollback takes back the steps the entry was charged since start; those of the values forced
// meanwhile stay theirs.
func (r *run) rollback(start int64) {
	r.ev.takeBack(r.charge, start)
}
