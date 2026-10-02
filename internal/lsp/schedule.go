package lsp

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// schedule runs a pass once no change came for the debounce, until ctx ends; a pass that
// fails or panics is logged and the next one runs.
func (s *server) schedule(ctx context.Context) error {
	timer := time.NewTimer(s.cfg.debounce)
	timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.kick:
			timer.Reset(s.cfg.debounce)
		case <-timer.C:
			s.logged(s.pass(ctx))
		}
	}
}

// work is what one pass recomputes: dirty projects, dirty loose documents (nil text once
// closed), and the URI the client gave each open document.
type work struct {
	keys     []string
	projects []*workspaceProject
	loose    []looseWork
	uris     map[string]string
}

type looseWork struct {
	abs  string
	text []byte
	open bool
}

// pass recomputes what changed and publishes it. When a change came meanwhile, or the pass
// panicked, nothing is published and its work waits for the next pass.
func (s *server) pass(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	w := s.begin(cancel)
	defer s.end(cancel)
	if len(w.keys) == 0 {
		return nil
	}
	var lists map[string]map[string][]diagnostic
	err := safego.Run(func() error {
		var err error
		lists, err = s.compute(ctx, w)
		return err
	})
	var panicked *safego.PanicError
	if errors.As(err, &panicked) {
		s.requeue(w.keys)
		return err
	}
	if ctx.Err() != nil {
		s.requeue(w.keys)
		return nil
	}
	s.publish(lists, w.uris)
	if s.cfg.passed != nil {
		s.cfg.passed()
	}
	return err
}

// compute is the new diagnostics of every part of w, by owner; a project that fails keeps its
// last ones.
func (s *server) compute(ctx context.Context, w work) (map[string]map[string][]diagnostic, error) {
	if s.cfg.computing != nil {
		s.cfg.computing(ctx)
	}
	lists := map[string]map[string][]diagnostic{}
	var errs []error
	for _, p := range w.projects {
		files, err := p.check(ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		lists[p.root] = files
	}
	for _, lw := range w.loose {
		files, err := looseDiagnostics(lw)
		errs = append(errs, err)
		lists[lw.abs] = files
	}
	return lists, errors.Join(errs...)
}

// begin takes the dirty set, with what each part needs, and makes cancel the running pass's.
func (s *server) begin(cancel context.CancelFunc) work {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancel = cancel
	w := work{uris: map[string]string{}}
	//canon:unordered the keys are sorted below
	for key := range s.dirty {
		w.keys = append(w.keys, key)
	}
	slices.Sort(w.keys)
	clear(s.dirty)
	//canon:unordered a lookup table
	for abs, doc := range s.docs {
		w.uris[abs] = doc.uri
	}
	for _, key := range w.keys {
		if p := s.projects[key]; p != nil {
			w.projects = append(w.projects, p)
			continue
		}
		lw := looseWork{abs: key}
		if doc := s.docs[key]; doc != nil {
			lw.text, lw.open = doc.text, true
		}
		w.loose = append(w.loose, lw)
	}
	return w
}

// buffers is the text of every open document of the project at root; s.mu is held.
func (s *server) buffers(root string) map[string][]byte {
	out := map[string][]byte{}
	//canon:unordered a lookup table
	for abs, doc := range s.docs {
		if doc.root == root {
			out[abs] = doc.text
		}
	}
	return out
}

// end cancels the pass and forgets its cancel; passes run one at a time.
func (s *server) end(cancel context.CancelFunc) {
	cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancel = nil
}

// requeue marks keys dirty again after a cancelled pass.
func (s *server) requeue(keys []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		s.dirty[key] = true
	}
}
