package eval

import "github.com/fantasim/canonlang/internal/value"

// InstanceOf is the instance rec is, for the tests of eval_test.
func (e *Evaluator) InstanceOf(rec *value.Record) *value.Record {
	return e.instance(rec)
}
