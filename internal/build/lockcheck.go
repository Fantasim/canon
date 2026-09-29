package build

import (
	"context"

	"github.com/fantasim/canonlang/internal/source"
)

// LockUpdates is each selected package's canon.lock the analysis adds facts or retirements
// to, with the lines added: what an edit writes beside its sources (API.md E20). None with an
// error or a layer, as for a build (API.md B1a); the analysis itself is left as it was.
func (a *Analysis) LockUpdates() ([]Lock, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.res.Summary.Errors > 0 || len(a.r.p.opt.Layers) > 0 {
		return nil, nil
	}
	var out []Lock
	for _, st := range a.r.locks {
		l, err := st.update()
		if err != nil {
			return nil, err
		}
		if l != nil && l.Status == StatusWritten {
			out = append(out, *l)
		}
	}
	return out, nil
}

// LockCheck runs phases 1 and 2, evaluates the lock facts alone, verifying nothing, then
// compares each selected package's canon.lock and reports W6006 where the sources hold values
// it lacks (API.md B4; log-2026-09-29 M4 U8-r).
func (p *Project) LockCheck(ctx context.Context, selectors []string) (*Result, error) {
	r, err := p.prepare(ctx, selectors)
	if err != nil {
		return nil, err
	}
	if err := r.check(ctx); err != nil {
		return nil, err
	}
	r.newHost(r.bags)
	for _, cp := range r.prog.Packages {
		if r.selects(cp.Path) {
			r.cps = append(r.cps, cp)
		}
	}
	if err := r.compareLocks(ctx); err != nil {
		return nil, err
	}
	r.reportPending()
	r.reportStableAmendments()
	r.host.loader.FinishDefines()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.host.failure(r.s.set, r.prog); err != nil {
		return nil, err
	}
	return r.result(), nil
}

// reportPending reports W6006 for each compared lock the sources hold values for that it lacks,
// at its file, or at no place when the package has none yet.
func (r *run) reportPending() {
	for _, st := range r.locks {
		if st.whole {
			st.file.Pending(st.sources, source.Span{File: st.id}, r.bags[st.pkg])
		}
	}
}
