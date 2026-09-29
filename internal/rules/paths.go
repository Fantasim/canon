package rules

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// rootAt is a top-level value traversed and not yet indexed: its value, declared type and name.
type rootAt struct {
	v    value.Value
	dt   types.Type
	name string
}

// later queues v, a top-level value whose traversal begins, for the path index.
func (r *Runner) later(v value.Value, dt types.Type, name string) {
	r.pending = append(r.pending, rootAt{v: v, dt: dt, name: name})
}

// pathOf is the path of v where its top-level value first reached it, nil for none. The index is
// built at the first asking, over the values traversed so far: a path reads only values' shapes
// and the keys of identities a keyed list or table gets when built, which no check run changes.
func (r *Runner) pathOf(v value.Value) *verify.Path {
	if v == nil {
		return nil
	}
	for _, p := range r.pending {
		r.index(p.v, p.dt, verify.Root(p.name))
	}
	r.pending = nil
	return r.paths[v]
}

// index records where each value of the subtree is first reached.
func (r *Runner) index(v value.Value, dt types.Type, at *verify.Path) {
	if _, known := r.paths[v]; known || v == nil {
		return
	}
	r.paths[v] = at
	eachPart(v, dt, func(p part) bool {
		r.index(p.v, p.t, p.s.on(at))
		return true
	})
}
