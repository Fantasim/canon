package eval

import "github.com/fantasim/canonlang/internal/value"

// retagLog is the records given an identity in place (withIdentity), in order, once asked for.
type retagLog struct {
	on  bool
	log []*value.Record
}

func (l *retagLog) note(rec *value.Record) {
	if l.on {
		l.log = append(l.log, rec)
	}
}

// RetagMark starts noting the records given an identity in place, when a keyed list or a table
// takes them, and is how many were noted so far: a mark for RetaggedSince.
func (e *Evaluator) RetagMark() int {
	e.retags.on = true
	return len(e.retags.log)
}

// RetaggedSince is the records given an identity in place since mark, in order.
func (e *Evaluator) RetaggedSince(mark int) []*value.Record {
	return e.retags.log[min(mark, len(e.retags.log)):]
}
