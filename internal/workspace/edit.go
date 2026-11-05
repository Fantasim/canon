package workspace

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
)

// EditRequest is an edit (API.md E17-E21): its changes, the revision its client read, whether it
// may write errors, writes nothing, or may normalize files not in canonical layout, and the
// language of its re-check's named-check messages, "" for Options.Lang (DECISIONS 345).
type EditRequest struct {
	Changes
	Base        string
	AllowErrors bool
	DryRun      bool
	Normalize   bool
	Lang        string
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

	wroteLocks []build.Lock // the canon.lock files it writes (E20), whose packages it locks (J7)

	checked    *build.Analysis // the re-check's analysis, and the packages it re-checked (E18, E35)
	rechecked  []string
	layerReads []build.Read      // what its inactive layers' analyses consulted (S5; DECISIONS 463)
	layers     []*build.Analysis // those analyses, which its edit event reuses (W13; DECISIONS 469)
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
	a, scope, err := s.scoped(ctx, req.Ops, req.Base)
	if err != nil {
		return nil, err
	}
	plan, err := s.apply(ctx, a, scope, req.Changes, req.AllowErrors)
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
	if err := out.recheck(ctx, a, owners, req.Lang); err != nil {
		return nil, err
	}
	if err := s.staleConsulted(ctx, req.Base, out); err != nil {
		return nil, err
	}
	if err := s.undoChecked(ctx, req, out); err != nil {
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
	if err := s.commit(ctx, out, out.writers(a, owners)); err != nil {
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
// overlay (S12), a stale base (S5), then a file that does not parse, then files not in canonical
// layout without Normalize (M9).
func (s *Snapshot) refuse(ctx context.Context, req EditRequest, a *build.Analysis, plan *edit.Plan, owners []string) error {
	if err := s.noOverlay(a, written(plan)); err != nil {
		return err
	}
	if err := s.staleFor(ctx, req.Base, a, owners); err != nil {
		return err
	}
	if err := unparsed(plan); err != nil {
		return err
	}
	if len(plan.NotCanonical) > 0 && !req.Normalize {
		return &NotCanonicalError{Files: slices.Clone(plan.NotCanonical)}
	}
	return nil
}

// written are the changes plan makes and the files it would write but cannot print into, which
// the overlay check (S12) judges like the others (API.md E21).
func written(plan *edit.Plan) []edit.Change {
	out := slices.Clone(plan.Changes)
	for _, bad := range plan.Unparsed {
		out = append(out, edit.Change{Kind: edit.ChangeModified, Path: bad.File})
	}
	return out
}

// unparsed is the first file of plan, in path order, that it must write and that does not parse,
// refused whatever Normalize and AllowErrors say: it cannot be laid out nor printed into (API.md M9).
func unparsed(plan *edit.Plan) error {
	if len(plan.Unparsed) == 0 {
		return nil
	}
	return plan.Unparsed[0]
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

// recheck re-checks on After the owners and their importers in lang, adds the lock lines, then
// the inactive layers' findings (API.md E17a, E18, E20, R6a; DECISIONS 463); Covers needs
// an every-package analysis kept on Before, the edit's own base being its scope's.
func (o *EditOutcome) recheck(ctx context.Context, a *build.Analysis, owners []string, lang string) error {
	pkgs := importing(a.Units(), owners...)
	switch {
	case len(pkgs) == 0 && len(o.Changes) > 0: // a file no package owns is never written unchecked
		return fmt.Errorf(fmtWrap, edit.ErrInternal, errUnowned)
	case len(pkgs) == 0:
		o.Checked = &build.Result{Findings: build.Findings{Files: a.Files()}}
		return nil
	}
	at := o.After // where the re-check is kept, which a lock line's plan below may replace
	checked, err := AnalyzeOnly(ctx, at, pkgs)
	if err != nil {
		return err
	}
	if o.Checked, err = checked.OwnIn(ctx, lang); err != nil {
		return err
	}
	o.checked, o.rechecked = checked, pkgs
	if all := o.Before.keptAnalysis(Key(OpAnalyze, nil), true); all != nil && build.Covers(all, checked) {
		o.After.cover(pkgs) // the packages outside pkgs read none of the files written (owners)
	}
	locked, err := o.ownLocks(checked)
	if locked {
		o.After = o.Before.planned(o.Changes, o.aliases) // the sources after the edit hold its lock lines
	}
	if err != nil {
		return err
	}
	if err := o.layered(ctx, checked, pkgs, lang); err != nil {
		return err
	}
	return at.keepEvent(ctx, checked, o.layers)
}

// undoChecked refuses a base edit made with AllowErrors whose Undo an operation would refuse,
// judged on the re-check's analysis of its result, which a clean edit adds no analysis to (API.md
// E22, E17a; DECISIONS 273, 466).
func (s *Snapshot) undoChecked(ctx context.Context, req EditRequest, o *EditOutcome) error {
	if !req.AllowErrors || req.EditLayer != "" || o.checked == nil {
		return nil
	}
	env := edit.Env{Project: o.After.b, Host: req.Host, Verdicts: &s.p.verdicts}
	return edit.CheckUndo(ctx, env, edit.NewSnapshot(o.checked), o.Plan)
}

// commit writes o's changes through s, journaled (API.md N10), named by the revision they give,
// under the writer locks of pkgs, after rolling back the journals they meet (N9, N11; DECISIONS
// 352, 375); a panic on the way rolls back what it changed before it goes on (X2).
func (s *Snapshot) commit(ctx context.Context, o *EditOutcome, pkgs []string) (err error) {
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
	release, reqs, err := s.lockWriters(ctx, &site, pkgs, len(o.Plan.Unowned) > 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, release()) }()
	if err := s.settleFirst(site, reqs); err != nil {
		return err
	}
	return commitOwn(ctx, site, rev, o.Changes)
}

// commitOwn commits changes on site as one of this process's commits in its directory; a panic
// rolls back what it changed before it goes on (API.md X2).
func commitOwn(ctx context.Context, site edit.Site, rev string, changes []edit.Change) error {
	done := committing(site.Layout.Dir())
	defer done()
	defer func() {
		if r := recover(); r != nil {
			done()
			ownBack(site)
			panic(r)
		}
	}()
	return edit.Commit(ctx, site, rev, changes)
}

// ownBack rolls back a commit that panicked, still under its locks: its own journal only, through
// Settle (DECISIONS 375: J8); a commit recording no lock, by pid as before. What it cannot roll
// back waits for the next Open (O5).
func ownBack(site edit.Site) {
	if site.Locks != nil {
		_, _ = edit.Settle(site, nil)
		return
	}
	_ = edit.Recover(site, nil)
}
