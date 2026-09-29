package build

import (
	"context"
	"errors"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"github.com/fantasim/canonlang/internal/views"
	"github.com/fantasim/canonlang/internal/views/render"
)

// viewFiles is an `emit view`'s one file, the package's view model written in phase 8, errors
// or not (VIEWMODEL.md J1, J4; log-2026-09-28 item 4: gen/view is no ir.Generator).
func (r *run) viewFiles(ctx context.Context, p *ir.Package, e *ir.Emit) ([]ir.File, error) {
	m, err := r.viewModel(ctx, p.Name)
	if err != nil {
		return nil, err
	}
	b, err := viewgen.Write(m)
	if err != nil {
		return nil, internal(err)
	}
	return []ir.File{{Path: e.FileName, Content: b}}, nil
}

// viewModel is pkg's view model, what phases 3-7 left alone evaluated on demand, aside (EVALUATION.md §1 phase 8).
func (r *run) viewModel(ctx context.Context, pkg string) (m *vm.ViewModel, err error) {
	scratch := r.throwawayBags()
	fold := eval.NewFolder(scratch, r.opt)
	restore := r.hostAside(scratch)
	defer func() {
		if failed := errors.Join(restore(), eval.FoldErr(fold)); err == nil && failed != nil {
			m, err = nil, failed
		}
	}()
	m, err = views.Build(ctx, r.viewInput(ctx, pkg, scratch, fold))
	if cerr := ctx.Err(); cerr != nil {
		return nil, cerr
	}
	if err != nil {
		return nil, internal(err)
	}
	return m, nil
}

// viewInput is what pkg's model is built from: phase 2's program and catalogues, a folder into
// throwaway bags, the values it reads, the package's findings of phases 1-7 (J15) and every
// loaded package's errors (J4).
func (r *run) viewInput(ctx context.Context, pkg string, scratch check.Bags, fold check.Folder) views.Input {
	proj := r.s.proj
	return views.Input{
		Program: r.prog, Package: pkg, Language: proj.Canon.String(), Studio: proj.Studio.Path,
		Languages: proj.Languages, Force: r.forceAside(ctx, scratch), Fold: fold,
		I18N: r.texts, Layout: r.s.layout, Layers: r.p.opt.Layers, Findings: r.bags[pkg].Findings(),
		Files: r.s.set, Errors: r.loadedErrors(), Eval: viewEval{r.ev}, CheckRun: r.failed.find,
	}
}

// forceAside is a root's value as phases 3-7 settled it, else evaluated now into scratch; false
// when it fails (VIEWMODEL.md C3, J12).
func (r *run) forceAside(ctx context.Context, scratch check.Bags) func(eval.Root) (value.Value, bool) {
	return func(root eval.Root) (value.Value, bool) { return r.ev.ForceAside(ctx, root, scratch) }
}

// loadedErrors is the error findings of every loaded package, in package order (VIEWMODEL.md J4).
func (r *run) loadedErrors() []diag.Finding {
	var out []diag.Finding
	for _, cp := range r.prog.Packages {
		out = append(out, errorsOf(r.bags[cp.Path])...)
	}
	return out
}

// hostAside points the host's loads and verification at scratch, keeping nothing a load reads,
// until the returned function restores it; that function is the run's failure: an unsupported
// load (ErrLoad, DECISIONS 196) or an internal error (195), phases 1-7 having met none.
func (r *run) hostAside(scratch check.Bags) func() error {
	h := r.host
	bags, verifier, throwaway := h.bags, h.verifier, h.scratch
	h.bags, h.verifier, h.scratch = scratch, verify.NewShared(r.vix, r.ev, scratch, r.assets), true
	return func() error {
		h.bags, h.verifier, h.scratch = bags, verifier, throwaway
		return h.failure(r.s.set, r.prog)
	}
}

// viewEval is the evaluator as the view model's render.Evaluator (VIEWMODEL.md §3.4, X7).
type viewEval struct {
	ev *eval.Evaluator
}

func (v viewEval) Eval(ctx context.Context, e syntax.Expr, self value.Value, m render.Magic) (value.Value, bool) {
	return v.ev.View(ctx, e, self, eval.Magic{ID: m.ID, Key: m.Key, Index: m.Index})
}
