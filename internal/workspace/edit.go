package workspace

import (
	"context"
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// EditRequest is an edit (API.md E17-E21): its changes, the revision its client read, whether it
// may write errors, writes nothing, or may normalize files not in canonical layout.
type EditRequest struct {
	Changes
	Base        string
	AllowErrors bool
	DryRun      bool
	Normalize   bool
}

// EditOutcome is an edit computed on Before: its plan; every file it writes, lock lines included
// (E20); the re-check of the packages it affects (E17, E18); and After, the sources after it,
// the snapshot published when Applied, else Before with the edit made in memory, never published.
type EditOutcome struct {
	Plan    *edit.Plan
	Changes []edit.Change
	Checked *build.Result
	Applied bool
	Before  *Snapshot
	After   *Snapshot
}

// Edit is the edit transaction, the one writer (API.md E17-E21, S9-S12, N10, W15): the
// operations applied in memory, the refusals in E21's order, then, unless DryRun, the commit and
// one edit event. An outcome refused for its errors comes with ErrRejected (E19).
func (p *Project) Edit(ctx context.Context, req EditRequest) (*EditOutcome, error) {
	var out *EditOutcome
	after := func() (Cause, []string) {
		if out == nil || !out.Applied {
			return CauseExternal, nil
		}
		return CauseEdit, out.written()
	}
	next, err := p.writeAs(ctx, after, func(ctx context.Context, s *Snapshot) error {
		var err error
		out, err = s.edit(ctx, req)
		return err
	})
	if out != nil && out.Applied {
		out.After = next
	}
	return out, err
}

func (s *Snapshot) edit(ctx context.Context, req EditRequest) (*EditOutcome, error) {
	a, err := s.analyze(ctx, nil)
	if err != nil {
		return nil, err
	}
	plan, err := s.apply(ctx, a, req.Changes)
	if err != nil {
		return nil, err
	}
	if err := s.refuse(req, a, plan); err != nil {
		return nil, err
	}
	out := &EditOutcome{Plan: plan, Changes: slices.Clone(plan.Changes), Before: s, After: s.planned(plan.Changes)}
	if err := out.recheck(ctx, a); err != nil {
		return nil, err
	}
	switch {
	case out.Checked.Summary.Errors > 0 && !req.AllowErrors:
		return out, ErrRejected
	case req.DryRun || len(out.Changes) == 0:
		return out, nil
	}
	if err := s.commit(ctx, out); err != nil {
		return nil, err
	}
	out.Applied = true
	return out, nil
}

// written are the absolute names of the files the edit changed, a renamed one's two included.
func (o *EditOutcome) written() []string {
	var out []string
	for _, c := range o.Changes {
		for _, display := range []string{c.Path, c.OldPath} {
			if abs, ok := o.Before.b.Abs(display); ok && display != "" && c.Kind != edit.ChangeRemovedDir {
				out = append(out, abs)
			}
		}
	}
	return out
}

// refuse is the first refusal of a plan after its operations (API.md E21): a file with an
// overlay (S12), a stale base (S5), then files not in canonical layout without Normalize (M9).
func (s *Snapshot) refuse(req EditRequest, a *build.Analysis, plan *edit.Plan) error {
	if err := s.noOverlay(plan.Changes); err != nil {
		return err
	}
	if err := s.staleFor(req.Base, a, plan.Touched); err != nil {
		return err
	}
	if len(plan.NotCanonical) > 0 && !req.Normalize {
		return &NotCanonicalError{Files: slices.Clone(plan.NotCanonical)}
	}
	return nil
}

// noOverlay is an *OverlayError for the first file changes write that has an overlay (API.md S12).
func (s *Snapshot) noOverlay(changes []edit.Change) error {
	for _, c := range changes {
		for _, display := range []string{c.Path, c.OldPath} {
			if abs, ok := s.b.Abs(display); ok && display != "" && s.fs.over[abs] != nil {
				return &OverlayError{File: display}
			}
		}
	}
	return nil
}

// recheck re-checks, on After, the packages owning a file the edit writes and their importers
// (API.md E17, E18), and adds the lock lines the edit requires (E20).
func (o *EditOutcome) recheck(ctx context.Context, a *build.Analysis) error {
	pkgs := affected(a.Program(), o.Plan.Touched...)
	switch {
	case len(pkgs) == 0 && len(o.Changes) > 0: // a file no package owns is never written unchecked
		return fmt.Errorf(fmtWrap, edit.ErrInternal, errUnowned)
	case len(pkgs) == 0:
		o.Checked = &build.Result{Findings: build.Findings{Files: a.Files()}}
		return nil
	}
	checked, err := o.After.b.Analyze(ctx, pkgs)
	if err != nil {
		return err
	}
	o.Checked = checked.Result()
	locked, err := o.ownLocks(checked)
	if locked {
		o.After = o.Before.planned(o.Changes) // the sources after the edit hold its lock lines
	}
	return err
}

// commit writes o's changes through s, journaled (API.md N10), named by the revision they give;
// a panic on the way rolls back what it changed before it goes on (X2).
func (s *Snapshot) commit(ctx context.Context, o *EditOutcome) error {
	w, err := s.fs.writable()
	if err != nil {
		return err
	}
	rev, err := o.After.revision(ctx)
	if err != nil {
		return err
	}
	dir := s.b.Dir()
	site := edit.Site{FS: through(s.fs, w), Layout: s.b, Self: self(dir)}
	done := committing(dir)
	defer done()
	defer func() {
		if r := recover(); r != nil {
			done()
			_ = edit.Recover(site, nil) // what it cannot roll back waits for the next Open (O5)
			panic(r)
		}
	}()
	return edit.Commit(ctx, site, rev, o.Changes)
}
