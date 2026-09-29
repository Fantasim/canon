package canon

import (
	"context"
	"errors"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/workspace"
)

// Event reports a new snapshot after a change and its automatic re-check (API.md W12-W16).
type Event struct {
	Revision Revision
	Cause    EventCause
	Files    []string
	Packages []string
	Findings []Finding // all current findings of Packages: replace, do not merge
	Summary  Summary
	Err      error
}

// Watch calls fn after each coalesced change until ctx is done or the project closes (rules
// W12-W16): it learns the read set of every package first, then returns. Without Options.FS
// it follows the OS's notifications, as it does for an FS with a method OSBacked() bool true.
func (p *Project) Watch(ctx context.Context, fn func(Event)) (err error) {
	defer recoverInternal(&err)
	s, err := p.read(ctx)
	if err != nil {
		return err
	}
	w := &watching{fn: fn, known: map[string]bool{}, reads: map[string]*pkgReads{}, roots: p.roots}
	if err := w.seed(ctx, s); err != nil {
		return err
	}
	if w.rev, err = p.revision(ctx, s); err != nil {
		return err
	}
	opts := workspace.WatchOptions{OS: p.osFiles, Logger: p.logger}
	if _, err := p.workspace().Watch(ctx, w.deliver, opts); err != nil {
		return apiError(err)
	}
	return nil
}

// watching is one Watch, on its delivery goroutine: packages seen (kept on failures, so a removal
// is reported), each one's reads (S2), the last read set and sources (S3), the last revision
// delivered, and a panic of fn (X2).
type watching struct {
	fn      func(Event)
	rev     Revision
	known   map[string]bool
	reads   map[string]*pkgReads
	listed  map[string]string // absolute name to display path
	sources map[string]string
	roots   map[string]string // Options.Roots, which load calls resolve through
	matched []string          // every file the loads' globs read at the last event, absolute
	fault   error
}

// seed learns the packages of s and what each reads, a check of all of them; a failure to
// check leaves them to be learnt at the first event (W12).
func (w *watching) seed(ctx context.Context, s *workspace.Snapshot) error {
	units, err := share(ctx, s, workspace.Key(workspace.OpPackages, nil), s.Build().Packages)
	if ending(ctx, err) {
		return errEnding(ctx, err)
	}
	if err != nil {
		units = nil
	}
	w.matched = w.globs(s, units)
	if units != nil && len(units.Units) > 0 {
		if err := w.learnAll(ctx, s, w.saw(units)); err != nil {
			return err
		}
	}
	w.listed, w.sources = readSet(s)
	return nil
}

// learnAll checks names together and learns what each reads; when that fails, it checks each on
// its own, so one package that cannot be checked leaves the others' reads known.
func (w *watching) learnAll(ctx context.Context, s *workspace.Snapshot, names []string) error {
	a, err := analyze(ctx, s, names)
	if ending(ctx, err) {
		return errEnding(ctx, err)
	}
	if err == nil {
		w.learn(a, names)
		return nil
	}
	for _, pkg := range names {
		one := []string{pkg}
		a, err := analyze(ctx, s, one)
		if ending(ctx, err) {
			return errEnding(ctx, err)
		}
		if err == nil {
			w.learn(a, one)
		}
	}
	return nil
}

// deliver re-checks what c affects and calls fn with the event (W13, W14); nothing is delivered
// once ctx is done or the project closed, nor for an outside change to nothing read (W12).
func (w *watching) deliver(ctx context.Context, c workspace.Change) {
	ev, ok := w.event(ctx, c)
	if !ok {
		return
	}
	ev.Err = joinErrors(ev.Err, w.fault)
	w.fault = w.call(ev)
}

// call runs fn, a panic in it returned as an *InternalError (X2).
func (w *watching) call(ev Event) (err error) {
	defer recoverInternal(&err)
	w.fn(ev)
	return nil
}

// event is c re-checked, false when there is nothing to deliver or the watch is ending.
func (w *watching) event(ctx context.Context, c workspace.Change) (Event, bool) {
	ev := Event{Cause: EventCause(c.Cause)}
	deliver, err := w.fill(ctx, c, &ev)
	if ending(ctx, err) {
		return ev, false
	}
	if c.Err != nil {
		err = joinErrors(err, panicError(c.Err))
	}
	ev.Err = err
	return ev, deliver || err != nil
}

// fill re-checks c and sets ev, the revision last, once the re-check read what it needed (S3);
// a panic is returned as an *InternalError (X2).
func (w *watching) fill(ctx context.Context, c workspace.Change, ev *Event) (deliver bool, err error) {
	defer recoverInternal(&err)
	s := c.Snapshot
	units, uerr := share(ctx, s, workspace.Key(workspace.OpPackages, nil), s.Build().Packages)
	if uerr != nil {
		units = nil
	}
	listed, sources := readSet(s)
	g := w.relevance(s, units)
	names := g.names(c, listed, sources)
	outside := c.Cause == workspace.CauseExternal
	if outside && len(names) == 0 {
		return false, nil
	}
	if uerr != nil {
		clear(w.reads)
		err = failed(uerr, ev)
	} else {
		err = w.recheck(ctx, s, units, names, ev)
	}
	w.matched = g.globs()
	recorded(listed, s)
	ev.Files = w.files(c, listed, sources, g.matched)
	rev, rerr := s.Revision(ctx)
	ev.Revision = Revision(rev)
	deliver = !outside || len(ev.Packages) > 0 || ev.Revision != w.rev || err != nil
	if deliver {
		w.rev = ev.Revision
	}
	return deliver, joinErrors(err, rerr)
}

// recheck re-checks the packages names affect and fills ev with their findings; a package
// removed is listed with none, so a client drops what it held (S2, W14).
func (w *watching) recheck(ctx context.Context, s *workspace.Snapshot, units *build.Units, names []string, ev *Event) error {
	live := w.affected(units, names)
	ev.Packages = w.removed(units, live)
	if len(live) == 0 {
		return nil
	}
	a, err := analyze(ctx, s, live)
	if err != nil {
		for _, pkg := range live {
			delete(w.reads, pkg)
		}
		return failed(err, ev)
	}
	res := a.Result()
	ev.Findings, ev.Summary = fromDiag(res.Files, res.List), summaryOf(res.Summary)
	w.learn(a, live)
	return nil
}

// removed is live and every package seen before that no longer exists, sorted; the packages of
// units are the ones seen from now on.
func (w *watching) removed(units *build.Units, live []string) []string {
	out := slices.Clone(live)
	now := w.saw(units)
	for _, pkg := range slices.Sorted(maps.Keys(w.known)) {
		if !slices.Contains(now, pkg) {
			out = append(out, pkg)
			delete(w.known, pkg)
			delete(w.reads, pkg)
		}
	}
	slices.Sort(out)
	return out
}

// saw records the packages of units as seen and returns their names, in order.
func (w *watching) saw(units *build.Units) []string {
	names := make([]string, 0, len(units.Units))
	for _, u := range units.Units {
		names = append(names, u.Name)
	}
	for _, name := range names {
		w.known[name] = true
	}
	return names
}

// failed is a re-check's error, its findings in ev when project.canon or a layer stopped it (O4).
func failed(err error, ev *Event) error {
	var pe *ProjectError
	if errors.As(err, &pe) {
		ev.Findings = pe.Findings
	}
	return err
}

// ending reports the watch ending: ctx done, or the project closed (O6).
func ending(ctx context.Context, err error) bool {
	return ctx.Err() != nil || errors.Is(err, ErrClosed)
}

func errEnding(ctx context.Context, err error) error {
	if cerr := ctx.Err(); cerr != nil {
		return cerr
	}
	return err
}

// joinErrors is a and b, either alone as it is when the other is nil.
func joinErrors(a, b error) error {
	switch {
	case b == nil:
		return a
	case a == nil:
		return b
	}
	return errors.Join(a, b)
}
