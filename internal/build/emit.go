package build

import (
	"cmp"
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// BuildOptions select what a build emits (API.md §13.1).
type BuildOptions struct {
	Packages []string    // selectors (API.md R1); none selects every package
	Targets  []ir.Target // emit only these (CLI.md §3.4 --target); none emits every target
	Adopt    []string    // display paths of unmarked C++ headers the build may take over (API.md B2)
	Check    bool        // write nothing; report what would change as stale (CLI.md §3.4 --check)
}

// Status is what a build does with one output file (API.md §13.1).
type Status uint8

// Output is one file an emit produces (CLI.md §3.4).
type Output struct {
	Path    string // display path (WIRE.md §2.3)
	Abs     string // the file on disk, --root overrides applied
	Target  ir.Target
	Package string
	Status  Status
	Content []byte
}

// Lock is the canon.lock of one selected package after the build (LOCK.md §5).
type Lock struct {
	Package   string
	Path, Abs string
	Status    Status
	Lines     []string // the lines added or rewritten, in canonical order
	Content   []byte
}

// BuildResult is a build's findings, outputs by Path, locks, and Check mode's verdict (API.md §13.1).
type BuildResult struct {
	Result
	Outputs []Output
	Locks   []Lock
	Stale   bool
}

// Build runs every phase, then writes the outputs and locks: all or nothing on a write error; a
// crash between renames can leave a mix.
func (p *Project) Build(ctx context.Context, opt BuildOptions) (*BuildResult, error) {
	r, err := p.prepare(ctx, opt.Packages)
	if err != nil {
		return nil, err
	}
	if err := r.refuseTargets(opt.Targets); err != nil {
		return nil, err
	}
	if err := r.analyze(ctx); err != nil {
		return nil, err
	}
	out := &BuildResult{Result: *r.result()}
	failed := out.Summary.Errors
	outputs, err := r.emit(ctx, opt, failed > 0)
	if err = interrupted(ctx, err); err != nil {
		return nil, err
	}
	if outputs, err = r.place(outputs, opt.Adopt); err != nil {
		return nil, err
	}
	if out.Result = *r.result(); out.Summary.Errors > failed {
		return out, nil
	}
	var locks []*lockOut
	if failed == 0 && len(p.opt.Layers) == 0 {
		if locks, err = r.updateLocks(); err != nil {
			return nil, err
		}
	}
	if err := r.commit(opt.Check, out, outputs, locks); err != nil {
		return nil, err
	}
	return out, nil
}

// refuseTargets refuses, before any analysis, an emit of the selection whose target has no
// generator yet, naming it (DECISIONS 196).
func (r *run) refuseTargets(targets []ir.Target) error {
	for _, u := range r.selected {
		for _, ed := range emitDecls(u) {
			t, known := targetOf(ed.Target.Name)
			if known && !written(t) && (len(targets) == 0 || slices.Contains(targets, t)) {
				return fmt.Errorf(fmtNoGenerator, u.Name, ed.Target.Name, ErrNoGenerator)
			}
		}
	}
	return nil
}

// emitDecls is every emit declaration of a package, in file then source order.
func emitDecls(u *project.Unit) []*syntax.EmitDecl {
	var out []*syntax.EmitDecl
	for _, f := range u.Files {
		for _, d := range f.Decls {
			if ed, ok := d.(*syntax.EmitDecl); ok && ed.Target != nil {
				out = append(out, ed)
			}
		}
	}
	return out
}

// written reports a target the build writes: a generator's, or the view model (log-2026-09-28 item 4).
func written(t ir.Target) bool {
	return t == ir.TargetView || generators[t] != nil
}

// targetOf is the target an emit's word names.
func targetOf(word string) (ir.Target, bool) {
	for t := ir.TargetGo; int(t) < len(targetWords); t++ {
		if targetWords[t] == word {
			return t, true
		}
	}
	return 0, false
}

// output is an Output with the location of the emit that writes it, for its findings.
type output struct {
	Output
	at      source.Span
	old     []byte // the file's content before the build
	existed bool
}

// emit runs the generator of every emit of the selected packages whose target opt selects, in
// package then source order; after an error, only the views run.
func (r *run) emit(ctx context.Context, opt BuildOptions, failed bool) ([]*output, error) {
	var out []*output
	for _, p := range r.ir {
		for _, e := range p.Emits {
			if skipped(e, opt.Targets, failed) {
				continue
			}
			placed, err := r.emitOne(ctx, p, e)
			if err != nil {
				return nil, err
			}
			out = append(out, placed...)
		}
	}
	return out, nil
}

// skipped reports an emit the build does not run: a target not selected, a code or data
// target after an error, or an output stage E could not place (its finding is reported).
func skipped(e *ir.Emit, targets []ir.Target, failed bool) bool {
	return len(targets) > 0 && !slices.Contains(targets, e.Target) || failed && e.Target != ir.TargetView || e.Dir == ""
}

// emitOne generates one emit and places its files; a view runs even after errors (API.md B1).
func (r *run) emitOne(ctx context.Context, p *ir.Package, e *ir.Emit) ([]*output, error) {
	var files []ir.File
	var err error
	if e.Target == ir.TargetView {
		files, err = r.viewFiles(ctx, p, e)
	} else if err = complete(p); err == nil {
		files, err = generate(p, e)
	}
	if err != nil {
		return nil, err
	}
	return r.outputs(p, e, files)
}

// complete refuses a package whose values or constants stage E left without a value: a
// generator never sees one, since a missing value follows an error that stops emission.
func complete(p *ir.Package) error {
	for _, v := range p.Values {
		if v.V == nil {
			return internal(fmt.Errorf(fmtNoValue, p.Name, v.Name))
		}
	}
	for _, c := range p.Consts {
		if c.V == nil {
			return internal(fmt.Errorf(fmtNoValue, p.Name, c.Name))
		}
	}
	return nil
}

func generate(p *ir.Package, e *ir.Emit) ([]ir.File, error) {
	if int(e.Target) >= len(generators) || generators[e.Target] == nil {
		return nil, fmt.Errorf(fmtEmit, p.Name, e.Out, ErrNoGenerator)
	}
	files, err := generators[e.Target](p, e)
	if err != nil {
		return nil, fmt.Errorf(fmtEmit, p.Name, e.Out, err)
	}
	return files, nil
}

// outputs places an emit's files: its out resolved from the package directory (WIRE.md §2).
func (r *run) outputs(p *ir.Package, e *ir.Emit, files []ir.File) ([]*output, error) {
	at, ok := r.s.layout.Resolve(e.Out, p.Dir, source.Span{}, diag.NewBag(nil, p.Name))
	if !ok {
		return nil, internal(fmt.Errorf(fmtUnplaced, p.Name, e.Out))
	}
	display, abs := strings.TrimSuffix(at.Display, pathSep), at.Abs
	if e.FileName != "" {
		display, abs = path.Dir(display), path.Dir(abs)
	}
	span := r.emitSpan(p.Name, e.Target)
	out := make([]*output, len(files))
	for i, f := range files {
		out[i] = &output{at: span, Output: Output{
			Path: path.Join(display, f.Path), Abs: path.Join(abs, f.Path), Target: e.Target, Package: p.Name, Content: f.Content,
		}}
	}
	return out, nil
}

// emitSpan locates the out option of a package's first emit of target t, the one stage E
// reads; no location when there is none.
func (r *run) emitSpan(pkg string, t ir.Target) source.Span {
	i := slices.IndexFunc(r.cps, func(cp *check.Package) bool { return cp.Path == pkg })
	if i < 0 {
		return source.Span{}
	}
	for _, f := range r.cps[i].Files {
		for _, d := range f.Decls {
			if ed, ok := d.(*syntax.EmitDecl); ok && ed.Target != nil && ed.Target.Name == targetWords[t] && ed.Options != nil {
				return f.Span(outOption(ed))
			}
		}
	}
	return source.Span{}
}

// outOption is the value of an emit's out option, or the emit itself when it has none.
func outOption(ed *syntax.EmitDecl) syntax.Node {
	for _, it := range ed.Options.Items {
		if fi, ok := it.(*syntax.FieldItem); ok && fi.Name != nil && fi.Name.Name == check.OptOut {
			return fi.Value
		}
	}
	return ed
}

// updateLocks is the lock of every selected package after a build without error (LOCK.md §5).
func (r *run) updateLocks() ([]*lockOut, error) {
	var out []*lockOut
	for _, st := range r.locks {
		l, err := st.update()
		if err != nil {
			return nil, err
		}
		if l != nil {
			out = append(out, &lockOut{Lock: *l, old: st.raw})
		}
	}
	return out, nil
}

// sortOutputs orders outputs by the bytes of their display path (API.md §13.1).
func sortOutputs(outputs []Output) {
	slices.SortFunc(outputs, func(a, b Output) int { return cmp.Compare(a.Path, b.Path) })
}
