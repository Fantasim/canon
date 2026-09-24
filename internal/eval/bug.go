package eval

import (
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/syntax"
)

// bug aborts the root on a state the checker excludes: every abort has a cause, a finding or,
// here, an internal error the caller turns into ErrInternal (DECISIONS 188).
func (r *run) bug(n syntax.Node) {
	if r.failed {
		return
	}
	r.failed = true
	where := ""
	if r.fr.file != nil && n != nil {
		sp := r.span(n)
		line, col := r.fr.file.Src.Position(sp.Start)
		where = fmt.Sprintf(fmtWhere, n.Kind(), r.fr.file.Src.Path, line, col)
	}
	r.ev.bugs = append(r.ev.bugs, fmt.Errorf(fmtBug, errInconsistent, where))
}

// Err is every internal error met so far, in the order met; nil when there is none.
func (e *Evaluator) Err() error {
	return errors.Join(e.bugs...)
}
