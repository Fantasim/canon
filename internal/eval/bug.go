package eval

import (
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// bug aborts the root on a state the checker excludes, an internal error the caller turns into
// ErrInternal (DECISIONS 195) whose detail names n's place, the value and the function (API.md X1).
func (r *run) bug(n syntax.Node) {
	if r.failed {
		return
	}
	r.failed = true
	parts := []string{r.bugSite(n)}
	if r.charge.name != "" {
		parts = append(parts, fmt.Sprintf(fmtEvaluating, r.charge.pkg, r.charge.name))
	}
	if pc, _, _, ok := runtime.Caller(1); ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			name := fn.Name()
			parts = append(parts, fmt.Sprintf(fmtMetIn, name[strings.LastIndex(name, pathSep)+1:]))
		}
	}
	where := strings.Join(slices.DeleteFunc(parts, func(p string) bool { return p == "" }), argSep)
	r.ev.bugs = append(r.ev.bugs, fmt.Errorf(fmtWrap, errInconsistent, where))
}

// bugSite is n's kind and position, "" without a node or a file to place it in.
func (r *run) bugSite(n syntax.Node) string {
	if r.fr == nil || r.fr.file == nil || n == nil {
		return ""
	}
	sp := r.span(n)
	line, col := r.fr.file.Src.Position(sp.Start)
	return fmt.Sprintf(fmtWhere, n.Kind(), r.fr.file.Src.Path, line, col)
}

// Err is every internal error met so far, in the order met; nil when there is none.
func (e *Evaluator) Err() error {
	return errors.Join(e.bugs...)
}
