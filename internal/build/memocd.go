package build

import (
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// memoChecks is checks serving the rules memo: a traced or replayed run is told with its
// instance as Run tells its own (VIEWMODEL.md J15).
type memoChecks struct {
	checks
}

// Ran notes a failed run of d on self, which rules made through the memo.
func (c memoChecks) Ran(d *syntax.CheckDecl, self value.Value, run rules.Run) {
	if run.Failed {
		c.failed.note(d, self, run.Message)
	}
}

// alone is the top-level values of r.order that no other value of r.order can hold a part of:
// none that completed after it is of its package or of one importing it. Stage C replays their
// tables' entries through the rules memo.
func (r *run) alone() func(eval.Root) bool {
	at := map[eval.Root]int{}
	for i, root := range r.ev.Completed() {
		at[root] = i
	}
	readers := map[string]map[string]bool{}
	out := map[eval.Root]bool{}
	for _, root := range r.order {
		i, done := at[root]
		if !done {
			continue
		}
		if readers[root.Pkg] == nil {
			readers[root.Pkg] = r.readersOf(root.Pkg)
		}
		out[root] = !r.heldBy(root, i, at, readers[root.Pkg])
	}
	return func(root eval.Root) bool { return out[root] }
}

// readersOf is pkg and every loaded package importing it, directly or not.
func (r *run) readersOf(pkg string) map[string]bool {
	out := map[string]bool{pkg: true}
	for _, u := range dependents(r.loaded, []string{pkg}) {
		out[u.Name] = true
	}
	return out
}

// heldBy reports a value of r.order, of a package in readers, that completed after root, the i-th.
func (r *run) heldBy(root eval.Root, i int, at map[eval.Root]int, readers map[string]bool) bool {
	for _, other := range r.order {
		if j, done := at[other]; done && j > i && other != root && readers[other.Pkg] {
			return true
		}
	}
	return false
}
