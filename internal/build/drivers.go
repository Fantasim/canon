package build

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views"
)

// driverReads is a program typeFunction `drivers` are read over and the fresh host that
// evaluates its values aside (VIEWMODEL.md 12.3): an unsupported load met there contributes no
// entries, on either program, whatever the selection; an internal error fails the model (errs).
type driverReads struct {
	prog    *check.Program
	host    *evalHost
	settled func(eval.Root) (value.Value, bool) // the run's own settled values; nil for the drivers-only program
}

// world is d as views reads it: a value phases 3-7 settled, else evaluated by d's host.
func (d *driverReads) world(ctx context.Context) *views.World {
	force := func(root eval.Root) (value.Value, bool) {
		if d.settled != nil {
			if v, ok := d.settled(root); ok {
				return v, true
			}
		}
		return d.host.ev.ForceAside(ctx, root, d.host.bags)
	}
	return &views.World{Program: d.prog, Force: force}
}

// errs is the internal errors d's host met (DECISIONS 195); its unsupported loads are ignored.
func (d *driverReads) errs() error {
	if d == nil {
		return nil
	}
	return d.host.internalErrs()
}

// driverReads are pkg's drivers' reads, each kind built once per run: over the run's own program
// when it loads every package importing pkg, directly or not (only they can apply its type
// functions); else over the drivers-only program.
func (r *run) driverReads(ctx context.Context, pkg string) (*driverReads, error) {
	missing := func(u *project.Unit) bool { return !slices.Contains(r.loaded, u) }
	if !slices.ContainsFunc(dependents(r.s.units, []string{pkg}), missing) {
		if r.ownReads == nil {
			h := r.throwawayHost()
			h.ev.BeginVerification(ctx)
			r.ownReads = &driverReads{prog: r.prog, host: h, settled: r.ev.Settled}
		}
		return r.ownReads, nil
	}
	if r.wideReads == nil {
		w, err := r.newWide(ctx)
		if err != nil {
			return nil, err
		}
		r.wideReads = &driverReads{prog: w.prog, host: w.host}
	}
	return r.wideReads, nil
}

// newWide checks and prepares, for drivers only, every package importing a selected one, with
// their imports: packages read only for this report nothing and block nothing (driverReads).
func (r *run) newWide(ctx context.Context) (*run, error) {
	units := withStudio(r.s.units, imported(r.s.units, dependents(r.s.units, r.selectedNames())), r.s.proj.Studio.Path)
	w := &run{p: r.p, s: r.s, loaded: units, bags: check.Bags{}, opt: r.opt}
	for _, u := range units {
		w.bags[u.Name] = diag.NewBag(r.s.set, u.Name)
	}
	if err := w.phase2(ctx); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	w.newHost(w.bags)
	w.host.scratch = true
	w.ev.BeginVerification(ctx)
	return w, nil
}

// dependents are the units importing one of names, directly or not, and those named, in name order.
func dependents(all []*project.Unit, names []string) []*project.Unit {
	in := map[string]bool{}
	for _, n := range names {
		in[n] = true
	}
	imports := func(u *project.Unit) bool {
		return slices.ContainsFunc(u.Imports, func(n string) bool { return in[n] })
	}
	for grown := true; grown; {
		grown = false
		for _, u := range all {
			if !in[u.Name] && imports(u) {
				in[u.Name], grown = true, true
			}
		}
	}
	return slices.DeleteFunc(slices.Clone(all), func(u *project.Unit) bool { return !in[u.Name] })
}
