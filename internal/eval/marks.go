package eval

import "github.com/fantasim/canonlang/internal/value"

// InvalidCount is how many values are marked invalid, a vector's parent's included: no mark is
// ever removed, so an unchanged count means nothing was marked since.
func (e *Evaluator) InvalidCount() int {
	n := len(e.invalid)
	if e.parent != nil {
		n += e.parent.InvalidCount()
	}
	return n
}

// InvalidValues is every value Invalid reports, in no order.
func (e *Evaluator) InvalidValues() []value.Value {
	var out []value.Value
	for ; e != nil; e = e.parent {
		for v := range e.invalid { //canon:unordered a set, read as one
			out = append(out, v)
		}
	}
	return out
}
