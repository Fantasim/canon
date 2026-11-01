package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/value"
)

// literalBuilder is a Builder that also verifies, into a capture, a kept literal a test built (TYPES.md §11.5).
type literalBuilder interface {
	// BuildLiteral verifies lit into capture, its steps the test's; false: a hard error stopped it.
	BuildLiteral(ctx context.Context, lit KeptLiteral, capture *diag.Bag) bool
}

// litScope is the kept literals a run evaluating no top-level value recorded, in order, with their
// entries no walk judged yet; a verification judging it also judges for up, the scope it is in.
type litScope struct {
	recs    []keptRec
	pending pendingSet
	up      *litScope
}

// newScope is an empty scope inside up.
func newScope(up *litScope) *litScope {
	return &litScope{pending: pendingSet{}, up: up}
}

// judgeScope hands each literal s recorded to judge, its entries still pending only, in recording
// order, s judging meanwhile; false when judge stopped at a hard error.
func (e *Evaluator) judgeScope(s *litScope, judge func(KeptLiteral) bool) bool {
	if s == nil || len(s.recs) == 0 {
		return true
	}
	defer e.judgingIn(s)()
	for len(s.recs) > 0 { // a judgement's defaults and where re-runs may record more (stageSink)
		recs := s.recs
		s.recs = nil
		for _, rec := range recs {
			if left := s.pending.residue(rec); left != nil && !e.judgeOne(rec, left, judge) {
				return false
			}
		}
	}
	return true
}

// checkLiterals judges check run r's kept literals: the judged subject's, else the check's (TYPES.md §11.5).
func (e *Evaluator) checkLiterals(ctx context.Context, r *run) {
	s := r.lits
	if s == nil || len(s.recs) == 0 {
		return
	}
	if j := e.lits.judging; j != nil {
		j.recs = append(j.recs, s.recs...)
		for v, es := range s.pending { //canon:unordered each value's entries kept in order
			j.pending[v] = append(j.pending[v], es...)
		}
		return
	}
	e.judgeScope(s, e.hostJudge(ctx, Root{Pkg: r.charge.pkg, Name: r.charge.name}))
}

// judgingIn makes a verification judge s's entries, and its outer scopes', until restore; nil: the roots'.
func (e *Evaluator) judgingIn(s *litScope) (restore func()) {
	saved := e.lits.judging
	e.lits.judging = s
	return func() { e.lits.judging = saved }
}

// hostJudge judges a literal through the host as root's, as a root's are, until the evaluator stops (EVALUATION.md §5).
func (e *Evaluator) hostJudge(ctx context.Context, root Root) func(KeptLiteral) bool {
	return func(l KeptLiteral) bool {
		e.lits.v.VerifyLiterals(ctx, root, []KeptLiteral{l})
		return ctx.Err() == nil && !e.exhausted
	}
}

// callScope is the scope of a precomputation's or a vector's call: one when the host verifies kept literals.
func (e *Evaluator) callScope() *litScope {
	if e.lits.v == nil {
		return nil
	}
	return newScope(nil)
}

// testScope is the scope of a test's statements, or of an expect's subject or substitutions inside
// it: one when the test's builder verifies kept literals.
func (r *run) testScope() *litScope {
	if r.test == nil || r.test.lits == nil {
		return nil
	}
	return newScope(r.lits)
}

// judgeInto judges s's literals through the test's builder into capture (EVALUATION.md §10.2).
func (r *run) judgeInto(s *litScope, capture *diag.Bag) bool {
	if s == nil || r.test == nil || r.test.lits == nil {
		return true
	}
	return r.ev.judgeScope(s, func(l KeptLiteral) bool { return r.test.lits.BuildLiteral(r.ctx, l, capture) })
}

// judgeStatements judges the literals the test's statements built where its other soft findings go;
// false when the package's budget ran out meanwhile.
func (r *run) judgeStatements() bool {
	e, pkg := r.ev, r.fr.pkg
	bag := e.bags[pkg]
	if bag == nil {
		bag = diag.NewBag(e.files(), pkg)
	}
	r.judgeInto(r.lits, bag)
	return !e.halted(pkg)
}

// adoptLiterals makes the copy of each literal of s a substituted value holds its let's, judged where stored (EVALUATION.md §10.5).
func (w *World) adoptLiterals(s *litScope, copies map[value.Value]value.Value) {
	k := &w.ev.lits
	if s == nil || k.v == nil {
		return
	}
	left := s.recs[:0]
	for _, rec := range s.recs {
		cm, held := copies[rec.m].(*value.Map)
		if !held || rec.sub == nil {
			left = append(left, rec)
			continue
		}
		wr := keptRec{st: w.ev.state(rec.sub), m: cm, t: rec.t, id: k.next}
		k.next++
		for i, v := range rec.m.Vals {
			if s.pending.take(v, func(x keptEntry) bool { return x.rec == rec.id && x.key == rec.m.Keys[i] }) {
				cv := copies[v]
				k.roots.pending[cv] = append(k.roots.pending[cv], keptEntry{rec: wr.id, t: rec.t, key: cm.Keys[i]})
			}
		}
		k.roots.recs = append(k.roots.recs, wr)
	}
	s.recs = left
}
