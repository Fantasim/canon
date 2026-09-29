package rules

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// retags is what an Evaluator giving records an identity in place also tells: a mark in its log
// of them, and the records given one since a mark.
type retags interface {
	RetagMark() int
	RetaggedSince(mark int) []*value.Record
}

// rootAt is a top-level value traversed and not yet indexed: its value, declared type, name, and
// the retag mark when its traversal began.
type rootAt struct {
	v    value.Value
	dt   types.Type
	name string
	mark int
}

// later queues v, a top-level value whose traversal begins, for the path index; without a retag
// log, v is indexed at once.
func (r *Runner) later(v value.Value, dt types.Type, name string) {
	if r.retags == nil {
		r.index(v, dt, verify.Root(name), nil)
		return
	}
	r.pending = append(r.pending, rootAt{v: v, dt: dt, name: name, mark: r.retags.RetagMark()})
}

// pathOf is the path of v where its top-level value first reached it, nil for none. The index is
// built at the first asking, each value's records given an identity since its traversal began
// named as they were then (API.md P8, log-2026-09-29 M4 P12-r).
func (r *Runner) pathOf(v value.Value) *verify.Path {
	if v == nil {
		return nil
	}
	for _, p := range r.pending {
		r.index(p.v, p.dt, verify.Root(p.name), lateIn(r.retags.RetaggedSince(p.mark)))
	}
	r.pending = nil
	return r.paths[v]
}

// index records where each value of the subtree is first reached.
func (r *Runner) index(v value.Value, dt types.Type, at *verify.Path, late func(*value.Record) bool) {
	if _, known := r.paths[v]; known || v == nil {
		return
	}
	r.paths[v] = at
	namedParts(v, dt, late, func(p part) bool {
		r.index(p.v, p.t, p.s.on(at), late)
		return true
	})
}

// lateIn is membership in retagged; nil for none.
func lateIn(retagged []*value.Record) func(*value.Record) bool {
	if len(retagged) == 0 {
		return nil
	}
	set := make(map[*value.Record]bool, len(retagged))
	for _, rec := range retagged {
		set[rec] = true
	}
	return func(rec *value.Record) bool { return set[rec] }
}

// lateFrom is, for a list met now, membership in the records given an identity from now on;
// nil without a retag log.
func (t *traversal) lateFrom() func(*value.Record) bool {
	if t.retags == nil {
		return nil
	}
	mark := t.retags.RetagMark()
	return func(rec *value.Record) bool { return slices.Contains(t.retags.RetaggedSince(mark), rec) }
}
