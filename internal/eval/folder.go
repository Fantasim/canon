package eval

import (
	"context"
	"errors"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// folder folds constants with the one evaluator (IMPLEMENTATION-PLAN §4.7, DECISIONS 150).
type folder struct {
	bags  check.Bags
	opt   Options
	steps *counter
	ev    *Evaluator
	calls []foldCall // every fold made, in order, for FoldsOf
	bugs  []error    // the internal errors of every fold so far, in the order met
	reads readLog    // the constants the folds read, which stage A forces again (EVALUATION.md §2.1)
}

// readLog is the constants a folder's folds read, once each, in the order first read (EVALUATION.md §2.1).
type readLog struct {
	roots []Root
	seen  map[Root]bool
}

// note logs st, read by the fold c, when it is a constant its folder's folds had not read; nil
// outside a fold, it does nothing.
func (c *foldCall) note(st *rootState) {
	if c == nil || c.log == nil || st.obj.Kind() != check.ObjConst || c.log.seen[st.root] {
		return
	}
	c.log.add(st.root)
}

// add logs root, read for the first time.
func (l *readLog) add(root Root) {
	if l.seen == nil {
		l.seen = map[Root]bool{}
	}
	l.seen[root] = true
	l.roots = append(l.roots, root)
}

// foldCall is one fold: its arguments, and what Broken answered it, recorded or replayed.
type foldCall struct {
	owner  check.Object
	e      syntax.Expr
	info   *check.Info
	broken map[check.Object]bool
	replay bool
	log    *readLog // its folder's, which notes each constant it reads
}

// NewFolder is the check.Folder a build gives check.Check: its findings go to the bag of the
// declaration that owns the folded expression.
func NewFolder(bags check.Bags, opt Options) check.Folder {
	return &folder{bags: bags, opt: opt, steps: newCounter(opt)}
}

// Fold evaluates e, a constant expression of owner that check has typed; a let, a user fn or a load in it fails the fold without a finding, which check reports (TYPES.md §15, DECISIONS 150).
func (f *folder) Fold(ctx context.Context, owner check.Object, e syntax.Expr, info *check.Info) (value.Value, bool) {
	return f.fold(ctx, foldCall{owner: owner, e: e, info: info})
}

// fold makes call, the answers of Broken kept with it, or given by it when it replays one.
func (f *folder) fold(ctx context.Context, call foldCall) (value.Value, bool) {
	if call.owner == nil || call.info == nil || f.bags == nil {
		return nil, false
	}
	if f.ev == nil || f.ev.info != call.info {
		f.ev = newEvaluator(f.bags, f.opt)
		f.ev.info, f.ev.constant, f.ev.counter = call.info, true, f.steps
	}
	ev := f.ev
	ev.index.pkg[call.owner.File()] = call.owner.Pkg()
	call.log = &f.reads
	ev.folding = &call
	r := ev.newRun(ctx, charge{pkg: call.owner.Pkg(), name: call.owner.Name()}, call.owner.File())
	v := r.eval(call.e)
	ev.folding = nil
	f.calls = append(f.calls, call)
	f.bugs, ev.bugs = append(f.bugs, ev.bugs...), nil
	return v, v != nil && !r.failed
}

// brokenAt is Broken[obj] as this fold sees it: the answer a replayed fold recorded, else info's.
func (c *foldCall) brokenAt(info *check.Info, obj check.Object) bool {
	if b, ok := c.broken[obj]; ok {
		return b
	}
	b := info != nil && info.Broken[obj]
	if !c.replay {
		if c.broken == nil {
			c.broken = map[check.Object]bool{}
		}
		c.broken[obj] = b
	}
	return b
}

// UseFolder makes e spend f's step counter, NewFolder's: one per invocation (DECISIONS 104).
// False, a no-op, once e has evaluated anything or for another Folder.
func (e *Evaluator) UseFolder(f check.Folder) bool {
	x, ok := f.(*folder)
	if !ok || !e.fresh() {
		return false
	}
	e.counter = x.steps
	return true
}

// FoldErr is every internal error the folds of f met, nil when none or when f is not
// NewFolder's: a build reports it as it does the evaluator's Err (DECISIONS 195).
func FoldErr(f check.Folder) error {
	if x, ok := f.(*folder); ok {
		return errors.Join(x.bugs...)
	}
	return nil
}
