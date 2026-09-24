package load

import (
	"context"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Loader reads load forms against a project's files: `load.dir` of JSON for now (WIRE.md §6).
type Loader struct {
	FS     project.FS
	Layout *project.Layout
	Set    *source.FileSet
}

// Request is a load expression forced from one file: its package, directory, call site and bag (WIRE.md §2.2).
type Request struct {
	Pkg  string
	From string
	Span source.Span
	Bag  *diag.Bag
}

// Load reads e against t; false after a finding, poisoning the value (EVALUATION.md §7).
func (l *Loader) Load(ctx context.Context, req Request, e *syntax.LoadExpr, t types.Type) (value.Value, bool, error) {
	if e.Method == nil || e.Method.Name != methodDir {
		return nil, false, unsupported("a load form other than load.dir")
	}
	return l.dir(ctx, req, e, t)
}
