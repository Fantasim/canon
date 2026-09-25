package eval

import "github.com/fantasim/canonlang/internal/value"

// InstanceOf is the instance rec is, for the tests of eval_test.
func (e *Evaluator) InstanceOf(rec *value.Record) *value.Record {
	return e.instance(rec)
}

// StepsSpent is the steps charged so far, for the tests of eval_test.
func (e *Evaluator) StepsSpent() int64 {
	return e.steps
}

// BoundCount is the number of records holding bound arguments, for the tests of eval_test.
func (e *Evaluator) BoundCount() int {
	return len(e.bound)
}
