package build

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

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
	sites     *loadSites                   // where the program's load expressions are
}

// Load runs e's form against expected; an unsupported form, option or default is ErrLoad,
// naming the cause load reported (DECISIONS 196).
func (h *evalHost) Load(ctx context.Context, e *syntax.LoadExpr, expected types.Type) (value.Value, bool) {
	return h.loadInto(ctx, e, expected, loadTarget{ev: h.ev, bags: h.bags, scratch: h.scratch})
}

// LoadInto is Load into a vector's throwaway bags, decoded through its evaluator ev (DECISIONS 204).
func (h *evalHost) LoadInto(ctx context.Context, ev *eval.Evaluator, e *syntax.LoadExpr, expected types.Type, bags check.Bags) (value.Value, bool) {
	return h.loadInto(ctx, e, expected, loadTarget{ev: ev, bags: bags, scratch: true})
}

// the memo replays the build's loads (IMPLEMENTATION-PLAN §7.6)
var _ eval.LoadMemo = (*evalHost)(nil)

// LoadRecorded is Load, and what the memo needs to replay it (IMPLEMENTATION-PLAN §7.6 NFR-02).
func (h *evalHost) LoadRecorded(ctx context.Context, e *syntax.LoadExpr, expected types.Type) (value.Value, bool, eval.LoadInputs) {
	var in *load.Inputs
	v, ok := h.loadInto(ctx, e, expected, loadTarget{ev: h.ev, bags: h.bags, scratch: h.scratch, inputs: &in})
	if !ok || in == nil {
		return v, ok, nil
	}
	return v, ok, in
}

// LoadReplay reads what a recorded load e read again, charged to its package as its reads were
// (API.md S3, S5); done reports the findings the load reported itself into the bag it used.
func (h *evalHost) LoadReplay(ctx context.Context, e *syntax.LoadExpr, in eval.LoadInputs) (func(), bool) {
	li, ok := in.(*load.Inputs)
	if !ok || h.scratch || h.loader == nil {
		return nil, false
	}
	site, ok := h.siteIn(h.prog, e)
	if !ok {
		return nil, false
	}
	defer h.track(site.pkg)()
	if ctx.Err() != nil || !h.loader.Replay(li) {
		return nil, false
	}
	bag := h.bags[site.pkg]
	return func() { li.Report(bag) }, true
}

// loadTarget is where a load goes: the evaluator decoding it (defaults, discriminants), the
// bags its findings go to, whether they are throwaway (load.Request.Scratch), and where a load
// recorded for the memo keeps its inputs (nil: not recorded).
type loadTarget struct {
	ev      *eval.Evaluator
	bags    check.Bags
	scratch bool
	inputs  **load.Inputs
}

// loadInto is Load into to's bags, decoded through to's evaluator.
func (h *evalHost) loadInto(ctx context.Context, e *syntax.LoadExpr, expected types.Type, to loadTarget) (value.Value, bool) {
	site, ok := h.siteIn(h.prog, e)
	if !ok {
		h.loads = append(h.loads, e)
		return nil, false
	}
	defer h.track(site.pkg)()
	req := load.Request{
		Pkg: site.pkg, From: path.Dir(site.file.Src.Path), Span: site.span, Bag: to.bags[site.pkg], Scratch: to.scratch,
	}.Through(to.ev)
	v, ok, err := h.run(ctx, req, e, expected, to.inputs)
	switch {
	case ctx.Err() != nil:
		return nil, false // the run returns ctx.Err()
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

// run is the loader's Load of e, or with inputs set Recorded, keeping its inputs there.
func (h *evalHost) run(ctx context.Context, req load.Request, e *syntax.LoadExpr, t types.Type, inputs **load.Inputs) (value.Value, bool, error) {
	if inputs == nil {
		return h.loader.Load(ctx, req, e, t)
	}
	out, err := h.loader.Recorded(ctx, req, e, t)
	*inputs = out.Inputs
	return out.Value, out.OK, err
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
	defer h.track(root.Pkg)()
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
	defer h.track(root.Pkg)()
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
		site, ok := h.siteIn(prog, h.loads[0])
		if !ok {
			return internal(errNoLoadSite)
		}
		return &LoadError{Span: site.span, Site: set.Locate(site.span), Cause: h.loadCause}
	}
	return h.internalErrs()
}

// internalErrs is the internal errors of stage B, of the fold and of the evaluator (DECISIONS
// 195); nil when there is none.
func (h *evalHost) internalErrs() error {
	errs := slices.Clone(h.errs)
	if err := eval.FoldErr(h.fold); err != nil {
		errs = append(errs, internal(err))
	}
	if err := h.ev.Err(); err != nil {
		errs = append(errs, internal(err))
	}
	return errors.Join(errs...)
}

// loadSite is where a load expression was written: its package, file and span.
type loadSite struct {
	pkg  string
	file *syntax.File
	span source.Span
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

// checks is the evaluator as rules.Evaluator: eval.CheckRun copied into rules.Run (DECISIONS
// 186); stage C and D's runner keeps the failed runs a view model's J15 messages need.
type checks struct {
	*eval.Evaluator
	failed *checkRuns
}

func (c checks) Run(ctx context.Context, d *syntax.CheckDecl, self value.Value, path string) rules.Run {
	x := c.Evaluator.Run(ctx, d, self, path)
	if x.Failed {
		c.failed.note(d, self, x.Message)
	}
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

// assets finds asset files, each segment matched exactly against a listing (TYPES.md §13.4).
type assets struct {
	fs     project.FS
	layout *project.Layout
	host   *evalHost
	dirs   map[string]dirListing
}

// dirListing is one directory's own file and subdirectory names, listed once.
type dirListing struct {
	files map[string]bool
	subs  map[string]bool
}

// Exists resolves root, then walks name under it one folder at a time (WIRE.md §2.2, TYPES.md §13.4).
func (a *assets) Exists(root, from, name string) (string, bool) {
	dir, ok := a.layout.Resolve(root, from, source.Span{}, diag.NewBag(nil, ""))
	if !ok {
		return root, false
	}
	segs := strings.Split(name, pathSep)
	abs := dir.Abs
	for _, seg := range segs[:len(segs)-1] {
		if !a.list(abs).subs[seg] {
			return dir.Display, false
		}
		abs = path.Join(abs, seg)
	}
	return dir.Display, a.list(abs).files[segs[len(segs)-1]]
}

// list is the file and subdirectory names of dir, listed once; neither when it does not exist.
func (a *assets) list(dir string) dirListing {
	if l, ok := a.dirs[dir]; ok {
		if log, ok := a.fs.(*readLog); ok { // a listing read once still counts for each package (API.md S5)
			log.note(touch{abs: dir, dir: true}, false)
		}
		return l
	}
	entries, err := a.fs.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		a.host.errs = append(a.host.errs, displayErrorIn(a.layout.Dir, err))
	}
	l := dirListing{files: map[string]bool{}, subs: map[string]bool{}}
	for _, e := range entries {
		if isDir, ok := a.kind(dir, e); ok {
			if isDir {
				l.subs[e.Name()] = true
			} else {
				l.files[e.Name()] = true
			}
		}
	}
	a.dirs[dir] = l
	return l
}

// kind is whether e is a directory or a file: its own type for anything but a symbolic link,
// which is followed to its target; a broken or looping link is neither, so a name under it is
// silently not found, as a listing of it was before this walk existed.
func (a *assets) kind(dir string, e fs.DirEntry) (isDir, ok bool) {
	if e.Type()&fs.ModeSymlink == 0 {
		return e.IsDir(), true
	}
	info, err := a.fs.Stat(path.Join(dir, e.Name()))
	if err != nil {
		return false, false
	}
	return info.IsDir(), true
}
