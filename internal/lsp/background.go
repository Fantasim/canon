package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// runningRequest is a background request's cancel, one per request so that a later one under
// the same id is never forgotten for it.
type runningRequest struct {
	cancel context.CancelFunc
}

// background runs a request that reads a project on its own goroutine, cancellable by id. It
// answers for its document as it was when read: changed since, ContentModified (DECISIONS 285);
// once its ctx ended, a cancellation; if it panicked, an internal error.
func (s *server) background(ctx context.Context, m message, h requestHandler) {
	ctx, cancel := context.WithCancel(ctx)
	key, run := idKey(m.ID), &runningRequest{cancel: cancel}
	abs := documentOf(m.Params)
	s.mu.Lock()
	s.running[key] = run
	version := s.versions[abs]
	s.mu.Unlock()
	s.inflight.Add(1)
	answered := false
	safego.Go(func() error {
		if s.cfg.answering != nil {
			s.cfg.answering()
		}
		result, err := h(s, ctx, m.Params)
		result, err = s.stale(ctx, abs, version, result, err)
		answered = true
		return s.respond(m.ID, result, err)
	}, func(err error) {
		if !answered {
			err = s.respond(m.ID, nil, err) // a panic: it is answered all the same
		}
		s.finished(key, run)
		s.logged(err)
		s.inflight.Done()
	})
}

// stale is a background answer, unless its ctx ended (a cancellation) or its document changed
// since it was read (ContentModified).
func (s *server) stale(ctx context.Context, abs string, version uint64, result any, err error) (any, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf(fmtWrap, errCancelled, ctx.Err())
	}
	s.mu.Lock()
	changed := s.versions[abs] != version
	s.mu.Unlock()
	if changed {
		return nil, errModified
	}
	return result, err
}

// documentOf is the absolute name of the document a request's params name, "" for none.
func documentOf(params json.RawMessage) string {
	var p struct {
		TextDocument documentID `json:"textDocument"`
	}
	if json.Unmarshal(params, &p) != nil {
		return ""
	}
	abs, _ := pathOf(p.TextDocument.URI)
	return abs
}

// finished forgets a background request, if its id is still its own.
func (s *server) finished(key string, run *runningRequest) {
	run.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[key] == run {
		delete(s.running, key)
	}
}

type cancelParams struct {
	ID json.RawMessage `json:"id"`
}

// cancelRequest cancels a request still running in the background ($/cancelRequest, LSP 3.17);
// any other id is ignored, as the protocol allows.
func (s *server) cancelRequest(params json.RawMessage) error {
	var p cancelParams
	if err := decode(params, &p); err != nil {
		return err
	}
	s.mu.Lock()
	run := s.running[idKey(p.ID)]
	s.mu.Unlock()
	if run != nil {
		run.cancel()
	}
	return nil
}

// idKey is a request id in one form, however the client spaced it.
func idKey(id json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, id); err != nil {
		return string(id)
	}
	return b.String()
}
