package workspace

import (
	"context"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// Op names a read whose identical concurrent calls on one snapshot share a computation (S8).
type Op string

// call is one shared computation: its result once done, and the calls waiting for it.
type call struct {
	done    chan struct{}
	val     any
	err     error
	waiters int
	cancel  context.CancelFunc
}

// Key is the key of op on a selection in the order given, so that a result, such as the first
// unknown selector it names, never depends on which call ran; extra parts tell calls apart.
func Key(op Op, selection []string, extra ...string) string {
	var b strings.Builder
	b.WriteString(strconv.Quote(string(op)))
	for _, list := range [][]string{selection, extra} {
		b.WriteString(strconv.Itoa(len(list))) // the count keeps nil and [""] apart
		for _, part := range list {
			b.WriteString(strconv.Quote(part))
		}
	}
	return b.String()
}

// Share runs fn once for the calls with one key on s while it runs (S8), cancelled when the last
// of them leaves or the project closes; each call returns ctx.Err() once its ctx is done (S11).
// A panic in fn is a *safego.PanicError (X2).
func Share[T any](ctx context.Context, s *Snapshot, key string, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	c := s.join(ctx, key, func(ctx context.Context) (any, error) { return fn(ctx) })
	select {
	case <-c.done:
		s.leave(key, c)
		if c.err != nil {
			return zero, s.p.closedOr(c.err)
		}
		v, _ := c.val.(T)
		return v, nil
	case <-ctx.Done():
		s.leave(key, c)
		return zero, ctx.Err()
	}
}

// join is the running computation of key, started now on its own goroutine if there is none.
func (s *Snapshot) join(ctx context.Context, key string, fn func(context.Context) (any, error)) *call {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.calls[key]; ok {
		c.waiters++
		return c
	}
	run, cancel := context.WithCancel(context.WithoutCancel(ctx))
	c := &call{done: make(chan struct{}), waiters: 1, cancel: cancel}
	s.calls[key] = c
	s.p.running(c, true)
	safego.Go(func() error {
		v, err := fn(run)
		c.val = v
		return err
	}, func(err error) {
		s.p.running(c, false)
		cancel()
		c.err = err
		s.forget(key, c)
		close(c.done)
	})
	return c
}

// leave is one call no longer waiting for c; the last one cancels it.
func (s *Snapshot) leave(key string, c *call) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.waiters--
	if c.waiters == 0 {
		c.cancel()
		if s.calls[key] == c {
			delete(s.calls, key)
		}
	}
}

// forget drops c from the running computations, so a call after it starts a new one.
func (s *Snapshot) forget(key string, c *call) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls[key] == c {
		delete(s.calls, key)
	}
}
