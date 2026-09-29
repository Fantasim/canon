package workspace

import (
	"context"
)

// Write runs fn as the one writer on a refreshed snapshot while readers keep the current one;
// then the disk is read again and that snapshot published and returned with fn's error, a
// panic in fn ending the write first (API.md S9-S11).
func (p *Project) Write(ctx context.Context, cause Cause, fn func(context.Context, *Snapshot) error) (next *Snapshot, err error) {
	if err := p.lock(ctx); err != nil {
		return nil, err
	}
	defer p.unlock()
	s, err := p.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { next = p.end(context.WithoutCancel(ctx), s, cause) }()
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

// end reads the disk again after a writer and publishes what it wrote (S10).
func (p *Project) end(ctx context.Context, s *Snapshot, cause Cause) *Snapshot {
	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()
	p.mu.Lock()
	p.writing = false
	p.mu.Unlock()
	next, err := p.refresh(ctx, s, cause)
	if err != nil {
		return s
	}
	return next
}
