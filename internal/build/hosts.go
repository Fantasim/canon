package build

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// evalHost is the eval.Host of a build (DECISIONS 186); it loads nothing yet.
type evalHost struct {
	ev       *eval.Evaluator
	verifier *verify.Verifier
	loads    []*syntax.LoadExpr
	errs     []error
}

// Load refuses every load expression; the build then fails with ErrLoad.
func (h *evalHost) Load(_ context.Context, e *syntax.LoadExpr, _ types.Type) (value.Value, bool) {
	h.loads = append(h.loads, e)
	return nil, false
}

// Verify runs stage B on one top-level value, then poisons it or reports E3505 (EVALUATION.md §5).
func (h *evalHost) Verify(ctx context.Context, root eval.Root, v value.Value) bool {
	res, err := h.verifier.Check(ctx, root, v)
	if err != nil {
		h.errs = append(h.errs, fmt.Errorf(fmtWrapInternal, ErrInternal, err))
		return false
	}
	if res.Poisoned {
		h.ev.Poison(root)
	}
	for _, u := range res.Unbound {
		h.ev.ReportUnbound(root, u.Ref, u.Path)
	}
	return res.Valid
}

// failure is the Go error of a run: a forced load, then the internal errors of stage B and of
// the evaluator; nil when there is none.
func (h *evalHost) failure(set *source.FileSet, prog *check.Program) error {
	if len(h.loads) > 0 {
		span, ok := loadSpan(prog, h.loads[0])
		if !ok {
			return fmt.Errorf(fmtNoLoadSite, ErrInternal)
		}
		return &LoadError{Span: span, Site: set.Locate(span)}
	}
	errs := slices.Clone(h.errs)
	if err := h.ev.Err(); err != nil {
		errs = append(errs, fmt.Errorf(fmtWrapInternal, ErrInternal, err))
	}
	return errors.Join(errs...)
}

// loadSpan is the span of a load expression, found in the program's files.
func loadSpan(prog *check.Program, e *syntax.LoadExpr) (source.Span, bool) {
	for _, cp := range prog.Packages {
		for _, f := range cp.Files {
			found := false
			syntax.Inspect(f, func(n syntax.Node) bool {
				found = found || n == syntax.Node(e)
				return !found
			})
			if found {
				return f.Span(e), true
			}
		}
	}
	return source.Span{}, false
}

// LoadError is a load expression a build forced before the load package exists (ErrLoad).
type LoadError struct {
	Span source.Span
	Site source.Location // the span resolved: display path, line and column
}

func (e *LoadError) Error() string {
	return fmt.Sprintf(fmtLoad, ErrLoad, e.Site.Path, e.Site.Line, e.Site.Col)
}

func (e *LoadError) Unwrap() error { return ErrLoad }

// checks is the evaluator as rules.Evaluator: eval.CheckRun copied into rules.Run (DECISIONS 186).
type checks struct {
	*eval.Evaluator
}

func (c checks) Run(ctx context.Context, d *syntax.CheckDecl, self value.Value) rules.Run {
	x := c.Evaluator.Run(ctx, d, self)
	out := rules.Run{Aborted: x.Aborted, Failed: x.Failed, Message: x.Message}
	for _, r := range x.Reports {
		out.Reports = append(out.Reports, rules.Report{Warn: r.Warn, At: r.At, Message: r.Message})
	}
	return out
}

// irHost is the evaluator as stage E's ir.Host; stage B has begun, so a first Force verifies.
type irHost struct {
	ev *eval.Evaluator
}

func (h irHost) Value(ctx context.Context, pkg, name string) (value.Value, bool) {
	return h.ev.Force(ctx, eval.Root{Pkg: pkg, Name: name})
}

func (h irHost) Call(ctx context.Context, fn check.Object, recv value.Value, args []value.Value) (value.Value, bool) {
	return h.ev.Call(ctx, fn, recv, args)
}

// assets finds asset files under the placed roots, names matched byte for byte, each directory
// listed once per run; a listing that fails is an error of the run.
type assets struct {
	fs     project.FS
	layout *project.Layout
	host   *evalHost
	dirs   map[string][]string
}

func (a *assets) Exists(root, name string) bool {
	p, ok := a.layout.Resolve(rootMark+root+pathSep+name, "", source.Span{}, diag.NewBag(nil, ""))
	if !ok || p.Dir {
		return false
	}
	return slices.Contains(a.files(path.Dir(p.Abs)), path.Base(p.Abs))
}

// files is the names of the files of dir, none when it does not exist.
func (a *assets) files(dir string) []string {
	if names, ok := a.dirs[dir]; ok {
		return names
	}
	entries, err := a.fs.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		a.host.errs = append(a.host.errs, fmt.Errorf(fmtWrap, err))
	}
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	a.dirs[dir] = names
	return names
}
