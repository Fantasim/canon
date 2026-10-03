package build

import (
	"github.com/fantasim/canonlang/internal/eval"
)

// alone is the values of r.order no other one of it can hold a part of. A value holds a part of
// another only if its evaluation read it, directly or through values read, as the evaluator noted
// (log-2026-09-29 M4 P12-r, amended by P18); one never completed is traversed by no stage C.
func (r *run) alone() func(eval.Root) bool {
	done := map[eval.Root]bool{}
	for _, root := range r.ev.Completed() {
		done[root] = true
	}
	order := make(map[eval.Root]bool, len(r.order))
	for _, root := range r.order {
		order[root] = true
	}
	out := map[eval.Root]bool{}
	for _, root := range r.order {
		out[root] = done[root] && !r.heldBy(root, order)
	}
	return func(root eval.Root) bool { return out[root] }
}

// heldBy reports another value of order whose evaluation read root, directly or through the
// values read: each value reading root, then each reading one of those, and so on; true when the
// evaluator noted no read.
func (r *run) heldBy(root eval.Root, order map[eval.Root]bool) bool {
	seen := map[eval.Root]bool{root: true}
	for queue := []eval.Root{root}; len(queue) > 0; queue = queue[1:] {
		readers, noted := r.ev.ReadBy(queue[0])
		if !noted {
			return true
		}
		for _, by := range readers {
			if seen[by] {
				continue
			}
			if order[by] {
				return true
			}
			seen[by] = true
			queue = append(queue, by)
		}
	}
	return false
}
