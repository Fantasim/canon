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
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// evalHost is the eval.Host of a build, load.dir of JSON included (DECISIONS 186, 196).
type evalHost struct {
	ev        *eval.Evaluator
	verifier  *verify.Verifier
	prog      *check.Program
	bags      check.Bags
	loader    *load.Loader
	loads     []*syntax.LoadExpr
	loadCause string
	errs      []error
}

// Load runs e's form against expected; an unsupported form, option or default is ErrLoad,
// naming the cause load reported (DECISIONS 196).
func (h *evalHost) Load(ctx context.Context, e *syntax.LoadExpr, expected types.Type) (value.Value, bool) {
	site, ok := findLoad(h.prog, e)
	if !ok {
		h.loads = append(h.loads, e)
		return nil, false
	}
	req := load.Request{Pkg: site.pkg, From: path.Dir(site.file.Src.Path), Span: site.span, Bag: h.bags[site.pkg]}
	v, ok, err := h.loader.Load(ctx, req, e, expected)
	switch {
	case errors.Is(err, load.ErrUnsupported):
		if len(h.loads) == 0 {
			h.loadCause = unsupportedCause(err)
		}
		h.loads = append(h.loads, e)
		return nil, false
	case err != nil:
		h.errs = append(h.errs, fmt.Errorf(fmtWrapInternal, ErrInternal, err))
		return nil, false
	default:
		return v, ok
	}
}

// EvalSymlinks lets load.dir follow links through the OS file system a build reads (WIRE.md §6.5).
func (f osFS) EvalSymlinks(name string) (string, error) { return project.EvalSymlinks(f.FS, name) }

// unsupportedCause is a load.UnsupportedError's own cause, "" for a bare ErrUnsupported.
func unsupportedCause(err error) string {
	var ue *load.UnsupportedError
	if errors.As(err, &ue) {
		return ue.Cause
	}
	return ""
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
		return &LoadError{Span: span, Site: set.Locate(span), Cause: h.loadCause}
	}
	errs := slices.Clone(h.errs)
	if err := h.ev.Err(); err != nil {
		errs = append(errs, fmt.Errorf(fmtWrapInternal, ErrInternal, err))
	}
	return errors.Join(errs...)
}

// loadSpan is the span of a load expression, found in the program's files.
func loadSpan(prog *check.Program, e *syntax.LoadExpr) (source.Span, bool) {
	site, ok := findLoad(prog, e)
	return site.span, ok
}

// loadSite is where a load expression was written: its package, file and span.
type loadSite struct {
	pkg  string
	file *syntax.File
	span source.Span
}

// findLoad is e's site, found by walking the program's files.
func findLoad(prog *check.Program, e *syntax.LoadExpr) (loadSite, bool) {
	for _, cp := range prog.Packages {
		for _, f := range cp.Files {
			found := false
			syntax.Inspect(f, func(n syntax.Node) bool {
				found = found || n == syntax.Node(e)
				return !found
			})
			if found {
				return loadSite{pkg: cp.Path, file: f, span: f.Span(e)}, true
			}
		}
	}
	return loadSite{}, false
}

// LoadError is a load form or option the load package does not read yet (ErrLoad), the cause
// it named (DECISIONS 196: every refusal names its cause; "" before any load reports one).
type LoadError struct {
	Span  source.Span
	Site  source.Location // the span resolved: display path, line and column
	Cause string
}

func (e *LoadError) Error() string {
	if e.Cause == "" {
		return fmt.Sprintf(fmtLoad, ErrLoad, e.Site.Path, e.Site.Line, e.Site.Col)
	}
	return fmt.Sprintf(fmtLoadCause, ErrLoad, e.Cause, e.Site.Path, e.Site.Line, e.Site.Col)
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
		a.host.errs = append(a.host.errs, displayErrorIn(a.layout.Dir, err))
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
