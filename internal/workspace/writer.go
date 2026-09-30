package workspace

import (
	"context"
)

// Write runs fn as the one writer on a refreshed snapshot while readers keep the current one;
// then the disk is read again and that snapshot published and returned with fn's error, a
// panic in fn ending the write first (API.md S9-S11).
func (p *Project) Write(ctx context.Context, cause Cause, fn func(context.Context, *Snapshot) error) (next *Snapshot, err error) {
	return p.writeAs(ctx, func() wrote { return wrote{cause: cause} }, fn)
}

// wrote is what a writer did, known once it returns: why the project has a new snapshot, the
// files it wrote by absolute name, and the snapshot of the sources as it planned them, whose
// contents it wrote (API.md E18); nil for none.
type wrote struct {
	cause   Cause
	files   []string
	planned *Snapshot
}

// writeAs is Write whose cause, files written and planned snapshot are known once fn returns: an
// edit that wrote nothing publishes an external change, if the disk changed meanwhile; one that
// wrote is published with its files read again, however the refresh went (W15).
func (p *Project) writeAs(ctx context.Context, after func() wrote, fn func(context.Context, *Snapshot) error) (next *Snapshot, err error) {
	if err := p.lock(ctx); err != nil {
		return nil, err
	}
	defer p.unlock()
	s, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		next = p.end(context.WithoutCancel(ctx), s, after())
	}()
	return nil, fn(ctx, s)
}

// lock takes the one-writer lock, or returns ctx.Err() or ErrClosed first (S9, S11, O6).
func (p *Project) lock(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case p.write <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	case <-p.closing:
		return ErrClosed
	}
	if _, _, err := p.current(); err != nil {
		p.unlock()
		return err
	}
	return nil
}

func (p *Project) unlock() {
	<-p.write
}

// begin refreshes the current snapshot, then marks a writer running so that no refresh reads
// the disk while it writes (S9).
func (p *Project) begin(ctx context.Context) (*Snapshot, error) {
	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()
	s, _, err := p.current()
	if err != nil {
		return nil, err
	}
	if s, err = p.refresh(ctx, s, CauseExternal); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.writing = true
	p.mu.Unlock()
	return s, nil
}

// end reads the disk again after a writer and publishes what it wrote (S10), its planned snapshot
// if any; a refresh that fails or misses the files written reads them again, never publishing the
// snapshot before the write (log-2026-09-29 M4 U5b-r).
func (p *Project) end(ctx context.Context, s *Snapshot, w wrote) *Snapshot {
	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()
	p.mu.Lock()
	p.writing = false
	p.mu.Unlock()
	if w.planned != nil && len(w.files) > 0 {
		return p.endPlanned(ctx, s, w)
	}
	next, err := p.refresh(ctx, s, w.cause)
	switch {
	case len(w.files) == 0 && err != nil:
		return s
	case len(w.files) == 0 || err == nil && next != s:
		return next
	}
	return p.reread(s, w)
}

// endPlanned publishes the snapshot the re-check read (API.md E18), so what it computed serves
// the calls after it (NFR-02), once the disk holds what was written; another change the refresh
// reads is folded into the event in a later snapshot (W15).
func (p *Project) endPlanned(ctx context.Context, s *Snapshot, w wrote) *Snapshot {
	n := w.planned
	n.fs.settle(s.fs.over)
	next, changed, err := p.recheck(ctx, n.fs)
	if err != nil {
		return p.reread(s, w)
	}
	names := make([]name, 0, len(w.files)+len(changed))
	for _, abs := range w.files {
		names = append(names, name{kind: kindFile, abs: abs})
	}
	if next != nil {
		n = p.snapshot(next)
	}
	p.publish(n, w.cause, n.displays(append(names, changed...)))
	return n
}

// reread publishes s with the files w wrote read again from the disk.
func (p *Project) reread(s *Snapshot, w wrote) *Snapshot {
	dropped := map[name]*entry{}
	names := make([]name, 0, len(w.files))
	for _, abs := range w.files {
		dropAt(dropped, abs)
		names = append(names, name{kind: kindFile, abs: abs})
	}
	ns := p.snapshot(s.fs.fork(s.fs.over, dropped))
	p.publish(ns, w.cause, ns.displays(names))
	return ns
}
