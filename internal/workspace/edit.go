package workspace

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
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
	aliases map[string]string // the other names packages read a written file by (owners)

	checked   *build.Analysis // the re-check's analysis, and the packages it re-checked (E18, E35)
	rechecked []string
}

// Edit is the edit transaction, the one writer (API.md E17-E21, S9-S12, N10, W15): the
// operations applied in memory, the refusals in E21's order, then, unless DryRun, the commit and
// one edit event. An outcome refused for its errors comes with ErrRejected (E19).
func (p *Project) Edit(ctx context.Context, req EditRequest) (*EditOutcome, error) {
	var out *EditOutcome
	after := func() wrote {
		if out == nil || !out.Applied {
			return wrote{cause: CauseExternal}
		}
		return wrote{cause: CauseEdit, files: out.written(), planned: out.After}
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

// edit is the edit computed on s: its ops applied against the analysis of their scope alone,
// then its affected packages re-checked (API.md E17, E17a, E18; DECISIONS 330), the refusals in
// E21's order; an edit without ops analyses nothing.
func (s *Snapshot) edit(ctx context.Context, req EditRequest) (*EditOutcome, error) {
	if err := edit.RenameRequest(req.Ops, req.AllowErrors, req.EditLayer); err != nil {
		return nil, err
	}
	if len(req.Ops) == 0 {
		return s.noOps(ctx, req.Base)
	}
	a, err := s.scoped(ctx, req.Ops)
	if err != nil {
		return nil, err
	}
	plan, err := s.apply(ctx, a, req.Changes, req.AllowErrors)
	if err != nil {
		return nil, err
	}
	owners, aliases, err := s.owners(ctx, plan)
	if err != nil {
		return nil, err
	}
	if err := s.refuse(ctx, req, a, plan, owners); err != nil {
		return nil, err
	}
	out := &EditOutcome{Plan: plan, Changes: slices.Clone(plan.Changes), Before: s, After: s.planned(plan.Changes, aliases), aliases: aliases}
	if err := out.recheck(ctx, a, owners); err != nil {
		return nil, err
	}
	if err := s.staleConsulted(ctx, req.Base, out); err != nil {
		return nil, err
	}
	if out.Checked.Summary.Errors > 0 && !req.AllowErrors {
		return out, ErrRejected
	}
	if err := out.preserved(req, a); err != nil {
		return nil, err
	}
	if req.DryRun || len(out.Changes) == 0 {
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

// owners are the packages its plan touches and those whose static read sets or asset roots hold
// what the edit writes, removes or creates, with the other names they read a file it writes by
// (API.md E17, DECISIONS 252, 330).
func (s *Snapshot) owners(ctx context.Context, plan *edit.Plan) ([]string, map[string]string, error) {
	var files, dirs, names []string
	for _, c := range plan.Changes {
		for _, display := range []string{c.Path, c.OldPath} {
			abs, ok := s.b.Abs(display)
			if !ok || display == "" {
				continue
			}
			files = append(files, abs)
			if c.Kind != edit.ChangeModified {
				dirs, names = append(dirs, s.listings(abs)...), append(names, abs)
			}
		}
	}
	st, err := s.static(ctx)
	if err != nil {
		return nil, nil, err
	}
	readers, aliases := st.Readers(files, dirs, names)
	return append(slices.Clone(plan.Touched), readers...), aliases, nil
}

// listings are the directories whose listing gains or loses abs, a name created or removed: abs
// itself, then each directory above it up to the first that existed before the edit, as pinDirs
// finds them (log-2026-09-29 M4 P14-r3).
func (s *Snapshot) listings(abs string) []string {
	out := []string{abs}
	for d := project.DirOf(abs); ; d = project.DirOf(d) {
		out = append(out, d)
		if _, err := s.fs.Stat(d); err == nil || project.DirOf(d) == d {
			return out
		}
	}
}

// refuse is the first refusal of a plan after its operations (API.md E21): a file with an
// overlay (S12), a stale base (S5), then files not in canonical layout without Normalize (M9).
func (s *Snapshot) refuse(ctx context.Context, req EditRequest, a *build.Analysis, plan *edit.Plan, owners []string) error {
	if err := s.noOverlay(a, plan.Changes); err != nil {
		return err
	}
	if err := s.staleFor(ctx, req.Base, a, owners); err != nil {
		return err
	}
	if len(plan.NotCanonical) > 0 && !req.Normalize {
		return &NotCanonicalError{Files: slices.Clone(plan.NotCanonical)}
	}
	return nil
}

// noOverlay is an *OverlayError for the first file changes write whose real path an overlay
// covers, under its own name or another (API.md S12; log-2026-09-29 M4 P14-r4).
func (s *Snapshot) noOverlay(a *build.Analysis, changes []edit.Change) error {
	var displays, names []string
	for _, c := range changes {
		for _, display := range []string{c.Path, c.OldPath} {
			if abs, ok := s.b.Abs(display); ok && display != "" {
				displays, names = append(displays, display), append(names, abs)
			}
		}
	}
	over := slices.Sorted(maps.Keys(s.fs.over))
	if len(names) == 0 || len(over) == 0 {
		return nil
	}
	covered := map[string]bool{}
	for _, real := range a.RealPaths(over...) {
		covered[real] = true
	}
	for i, real := range a.RealPaths(names...) {
		if s.fs.over[names[i]] != nil || covered[real] {
			return &OverlayError{File: displays[i]}
		}
	}
	return nil
}

// recheck re-checks on After the owners and their importers, with their imports alone, and adds
// the lock lines the edit requires (API.md E17, E17a, E18, E20); Covers needs an every-package
// analysis a call kept on Before, the edit's own base being its scope's.
func (o *EditOutcome) recheck(ctx context.Context, a *build.Analysis, owners []string) error {
	pkgs := importing(a.Units(), owners...)
	switch {
	case len(pkgs) == 0 && len(o.Changes) > 0: // a file no package owns is never written unchecked
		return fmt.Errorf(fmtWrap, edit.ErrInternal, errUnowned)
	case len(pkgs) == 0:
		o.Checked = &build.Result{Findings: build.Findings{Files: a.Files()}}
		return nil
	}
	checked, err := AnalyzeOnly(ctx, o.After, pkgs)
	if err != nil {
		return err
	}
	o.Checked, o.checked, o.rechecked = checked.Own(), checked, pkgs
	if all := o.Before.keptAnalysis(Key(OpAnalyze, nil), true); all != nil && build.Covers(all, checked) {
		o.After.cover(pkgs) // the packages outside pkgs read none of the files written (owners)
	}
	locked, err := o.ownLocks(checked)
	if locked {
		o.After = o.Before.planned(o.Changes, o.aliases) // the sources after the edit hold its lock lines
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
