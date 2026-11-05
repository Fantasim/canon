package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// InstanceOf is the instance rec is, for the tests of eval_test.
func (e *Evaluator) InstanceOf(rec *value.Record) *value.Record {
	return e.instance(rec)
}

// VersionOf reports old an earlier version of rec, an amendment's copy of it or of its copies,
// for the tests of eval_test.
func (e *Evaluator) VersionOf(rec, old *value.Record) bool {
	seen := map[value.Value]bool{}
	for v := value.Value(rec); v != nil && !seen[v]; v = e.rebuilt[v] {
		seen[v] = true
		if x, ok := v.(*value.Record); ok && e.instance(x) == e.instance(old) {
			return true
		}
	}
	return false
}

// StepsSpent is the steps charged so far, for the tests of eval_test.
func (e *Evaluator) StepsSpent() int64 {
	return e.steps
}

// FoldSteps is the steps charged to f's counter so far, for the tests of eval_test.
func FoldSteps(f check.Folder) int64 {
	return f.(*folder).steps.steps
}

// MemoCounts is the entries e replayed, kept in its memo and evaluated without keeping them.
func (e *Evaluator) MemoCounts() (hits, stored, unkept int) {
	if e.memo == nil {
		return 0, 0, 0
	}
	s := e.memo.stats
	return s.hits, s.stored, s.unkept
}

// WrittenMark reports v kept as written (TYPES.md §11.4), for the tests of eval_test.
func (e *Evaluator) WrittenMark(v value.Value) bool {
	return e.isWritten(v)
}

// BoundArgs is the arguments bound to rec (TYPES.md §11.1), for the tests of eval_test.
func (e *Evaluator) BoundArgs(rec *value.Record) map[*types.Param]value.Value {
	return e.boundParams(rec)
}

// BoundCount is the number of records holding bound arguments, for the tests of eval_test.
func (e *Evaluator) BoundCount() int {
	return len(e.bound)
}
