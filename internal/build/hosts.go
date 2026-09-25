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
	assets    *assets
	index     *verify.Index
	loads     []*syntax.LoadExpr
	loadCause string
	errs      []error
	logBags   func() check.Bags            // set by canon test: each verification gets fresh bags...
	causes    map[eval.Root][]diag.Finding // ...and a value it poisons keeps their errors
	fold      check.Folder                 // phase 2's folder, whose internal errors failure reports (DECISIONS 195)
	scratch   bool                         // bags is throwaway: load keeps nothing it reads (a test call's host)
}

// Load runs e's form against expected; an unsupported form, option or default is ErrLoad,
// naming the cause load reported (DECISIONS 196).
func (h *evalHost) Load(ctx context.Context, e *syntax.LoadExpr, expected types.Type) (value.Value, bool) {
	return h.loadInto(ctx, e, expected, h.bags, h.scratch)
}

// LoadInto is Load into a vector's throwaway bags, a scratch load (ADR-0003, DECISIONS 204).
func (h *evalHost) LoadInto(ctx context.Context, e *syntax.LoadExpr, expected types.Type, bags check.Bags) (value.Value, bool) {
	return h.loadInto(ctx, e, expected, bags, true)
}

// loadInto is Load into bags; scratch: bags is throwaway (load.Request.Scratch).
func (h *evalHost) loadInto(ctx context.Context, e *syntax.LoadExpr, expected types.Type, bags check.Bags, scratch bool) (value.Value, bool) {
	site, ok := findLoad(h.prog, e)
	if !ok {
		h.loads = append(h.loads, e)
		return nil, false
	}
	req := load.Request{Pkg: site.pkg, From: path.Dir(site.file.Src.Path), Span: site.span, Bag: bags[site.pkg], Scratch: scratch}
	v, ok, err := h.loader.Load(ctx, req, e, expected)
	switch {
	case errors.Is(err, load.ErrUnsupported):
		if len(h.loads) == 0 {
			h.loadCause = unsupportedCause(err)
		}
		h.loads = append(h.loads, e)
		return nil, false
	case err != nil:
		h.errs = append(h.errs, internal(err))
		return nil, false
	default:
		return v, ok
	}
}

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
	if h.logBags != nil {
		return h.verifyLogged(ctx, root, v)
	}
	res, err := h.verifier.Check(ctx, root, v)
	return h.settle(h.ev, root, res, err)
}

// verifyLogged verifies into fresh bags and keeps the errors of one that poisons the value, which
// canon test prints under a read of it (meta/decisions/log-2026-09-24.md "canon test review calls").
func (h *evalHost) verifyLogged(ctx context.Context, root eval.Root, v value.Value) bool {
	bags := h.logBags()
	restore := h.ev.ReportAside(bags) // a where predicate's hard error is the evaluator's (EVALUATION.md §7.1)
	res, err := verify.NewShared(h.index, h.ev, bags, h.assets).Check(ctx, root, v)
	restore()
	if err == nil && res.Poisoned {
		h.causes[root] = h.errorsIn(bags)
	}
	return h.settle(h.ev, root, res, err)
}

// errorsIn is the error findings of bags, in package order.
func (h *evalHost) errorsIn(bags check.Bags) []diag.Finding {
	var out []diag.Finding
	for _, cp := range h.prog.Packages {
		out = append(out, errorsOf(bags[cp.Path])...)
	}
	return out
}

// errorsOf is the error findings of b.
func errorsOf(b *diag.Bag) []diag.Finding {
	var out []diag.Finding
	for _, f := range b.Findings() {
		if f.Severity == diag.Error {
			out = append(out, f)
		}
	}
	return out
}

// VerifyInto verifies a value first forced in a vector through ev, the vector's evaluator, into its throwaway bags (ADR-0003, EVALUATION.md §1).
func (h *evalHost) VerifyInto(ctx context.Context, ev *eval.Evaluator, root eval.Root, v value.Value, bags check.Bags) bool {
	res, err := verify.NewShared(h.index, ev, bags, h.assets).Check(ctx, root, v)
	return h.settle(ev, root, res, err)
}

// settle applies a verification's result through ev: an error is internal; a poisoned value is
// poisoned, an unbound ref reported (E3505); true when the value is valid.
func (h *evalHost) settle(ev *eval.Evaluator, root eval.Root, res verify.Result, err error) bool {
	if err != nil {
		h.errs = append(h.errs, internal(err))
		return false
	}
	if res.Poisoned {
		ev.Poison(root)
	}
	for _, u := range res.Unbound {
		ev.ReportUnbound(root, u.Ref, u.Path)
	}
	return res.Valid
}

// failure is the Go error of a run: a forced load, then the internal errors of stage B and of
// the evaluator; nil when there is none.
func (h *evalHost) failure(set *source.FileSet, prog *check.Program) error {
	if len(h.loads) > 0 {
		span, ok := loadSpan(prog, h.loads[0])
		if !ok {
			return internal(errNoLoadSite)
		}
		return &LoadError{Span: span, Site: set.Locate(span), Cause: h.loadCause}
	}
	errs := slices.Clone(h.errs)
	if err := eval.FoldErr(h.fold); err != nil {
		errs = append(errs, internal(err))
	}
	if err := h.ev.Err(); err != nil {
		errs = append(errs, internal(err))
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

// Exists resolves root+"/"+name: root is an AssetSpec.Root, already carrying its own "@" (TYPES.md §13.4).
func (a *assets) Exists(root, name string) bool {
	p, ok := a.layout.Resolve(root+pathSep+name, "", source.Span{}, diag.NewBag(nil, ""))
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
